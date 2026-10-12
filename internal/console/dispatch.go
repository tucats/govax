package console

import (
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/help"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// Dispatcher parses one command line with the DCL grammar and runs the
// handler bound to the verb or syntax it ends in.
type Dispatcher struct {
	Console *Console
	Grammar *dcl.Grammar
	Help    *help.File
}

// NewDispatcher returns a Dispatcher wired to c and g, with every DCL
// verb/syntax this port implements bound to its handler.
func NewDispatcher(c *Console, g *dcl.Grammar, h *help.File) *Dispatcher {
	d := &Dispatcher{Console: c, Grammar: g, Help: h}
	d.bindGrammar()

	// The first dispatcher is the console's, which XFC$CONSOLE_CMD and
	// the subprocesses' DCL (subcli.go) use.
	if c.Dispatcher == nil {
		c.Dispatcher = d
		c.Grammar = g
		c.route = d.Dispatch
	}

	return d
}

// Dispatch parses and executes one command line: assembler mode's
// statements, DCL symbol assignments and symbol-named commands, and
// otherwise the DCL grammar.
func (d *Dispatcher) Dispatch(line string) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "!") {
		return nil
	}

	// Check vax.console.assembler_mode before ever reading a verb:
	// while interactive ASM mode (docs/PHASE-19.md) is on,
	// every line -- including one that happens to spell a command name --
	// is a statement for the assembler, not a console command.
	if d.Console.InAssemblerMode() {
		return d.Console.AssembleLine(line)
	}

	// With a debugger session in progress, the line is a debugger
	// command. Routing here, rather than in the front end alone, lets a
	// command file (@file, INCLUDE) or a script on stdin mix the two: its
	// lines go to whichever grammar is current as each is read, so a GO
	// that stops at a breakpoint is followed by debugger commands, and
	// the debugger's EXIT by console commands again. (XFC$CONSOLE_CMD
	// calls DispatchConsole instead, since a VAX program asking for a
	// console command means the console's.)
	if d.Console.InDebugger() {
		return d.Console.Debugger.Dispatch(line)
	}

	err := d.Console.DispatchConsole(line)

	// A command of process 1's command level 0 is done: the other
	// processes run until they're idle (dcllend.go). A procedure's lines
	// give them their turns as the procedure runs them (runLevel).
	if d.Console.CommandLevel() == 0 {
		d.Console.lendWhileIdle()
	}

	return err
}

// bindGrammar binds every DCL verb/syntax this port implements a handler
// for. Everything else (device/RTL/assembler-dependent SHOW/CLEAR/DEFINE
// sub-forms, TEST) is deliberately left unbound:
// Grammar.Dispatch's own "no handler bound" error already reports that
// clearly, so no separate stub code is needed for each one.
func (d *Dispatcher) bindGrammar() {
	g := d.Grammar

	// docs/PHASE-37.md: the former fixed commands (commands.go).
	d.bindConsoleCommands()
	d.Console.bindDCL(g)

	g.Bind("DEBUG", func(id int64, r *dcl.Result) error { return d.Console.StartDebugger() })

	g.Bind("VMINIT", func(id int64, r *dcl.Result) error {
		return d.Console.VMInit(
			uint32(r.Int("P0")), uint32(r.Int("P1")), uint32(r.Int("S0")),
			uint32(r.Int("KSP")), uint32(r.Int("ESP")), uint32(r.Int("SSP")), uint32(r.Int("ISP")),
			uint32(r.Int("STRINGPOOL")),
		)
	})

	g.Bind("CLEAR_STRINGS", func(id int64, r *dcl.Result) error { return d.Console.ClearString() })
	g.Bind("CLEAR_MEMORY", func(id int64, r *dcl.Result) error { return d.Console.ClearMemory() })

	g.Bind("SHOW_RADIX", func(id int64, r *dcl.Result) error { return d.Console.ShowRadix() })
	// SHOW VERSION names the microkernel. (It and the ABOUT command once
	// called the microkernel's EXE$ABOUT; ABOUT is gone, HELP ABOUT
	// taking its place, and so is the routine.)
	g.Bind("SHOW_VERSION", func(id int64, r *dcl.Result) error { return d.Console.ShowVersion() })

	g.Bind("SHOW_NVRAM", func(id int64, r *dcl.Result) error { return d.Console.ShowNVRAM() })
	g.Bind("SHOW_ROM", func(id int64, r *dcl.Result) error { return d.Console.ShowROM() })
	g.Bind("SHOW_STRING", func(id int64, r *dcl.Result) error { return d.Console.ShowString() })

	g.Bind("SHOW_SHARE", func(id int64, r *dcl.Result) error { return d.Console.ShowSharePrefix() })

	// Phase 23 (docs/PHASE-23.md, subtask 3): SHOW DEFAULT displays the
	// operator's current default device/directory (internal/console/
	// default.go's Console.ShowDefault, backed by internal/rms.Session).
	// It was a DCL grammar entry before SET DEFAULT was (docs/PHASE-37.md),
	// since SHOW already has a large family of "syntax show_*" entries
	// (show_map/show_tb/... above) that DEFAULT slots into the same way.
	g.Bind("SHOW_DEFAULT", func(id int64, r *dcl.Result) error { return d.Console.cmd().ShowDefault() })

	// SHOW SYSTEM and SHOW PROCESS (docs/PHASE-44.md, subtask 10).
	g.Bind("SHOW_SYSTEM", func(id int64, r *dcl.Result) error { return d.Console.cmd().ShowSystem() })
	g.Bind("SHOW_PROCESS", func(id int64, r *dcl.Result) error {
		return d.Console.cmd().ShowProcess(r.String("PROCESS_NAME"), r.String("IDENTIFICATION"))
	})

	g.Bind("SHOW_DEBUG", func(id int64, r *dcl.Result) error { return d.Console.ShowDebug() })

	// SHOW MEMORY is allowed in both console and debugger
	g.Bind("SHOW_MEMORY", func(id int64, r *dcl.Result) error { return d.Console.ShowMemory() })

	// Phase 09 (internal/io): device abstraction and logical name tables.
	g.Bind("SHOW_DEVICE", func(id int64, r *dcl.Result) error {
		return d.Console.cmd().ShowDevices(r.String("NAME"), r.Present("FULL"))
	})

	g.Bind("DEFINE_DEVICE", func(id int64, r *dcl.Result) error {
		d.Console.cmd().DefineDevice(r.String("NAME"), iodev.DeviceOptions{
			Cluster:     uint32(r.Int("CLUSTER")),
			Cylinders:   uint32(r.Int("CYLINDERS")),
			DevBufSize:  uint32(r.Int("DEVBUFSIZE")),
			DevChar:     uint32(r.Int("DEVCHAR")),
			DevChar2:    uint32(r.Int("DEVCHAR2")),
			DevClass:    iodev.DeviceClass(r.Int("DEVCLASS")),
			DevDepend:   uint32(r.Int("DEVDEPEND")),
			DevDepend2:  uint32(r.Int("DEVDEPEND2")),
			DevType:     uint32(r.Int("DEVTYPE")),
			FreeBlocks:  uint32(r.Int("FREEBLOCKS")),
			LockID:      uint32(r.Int("LOCKID")),
			MaxBlock:    uint32(r.Int("MAXBLOCK")),
			MaxFiles:    uint32(r.Int("MAXFILES")),
			OwnUIC:      uint32(r.Int("OWNUIC")),
			RecSize:     uint32(r.Int("RECSIZE")),
			Sectors:     uint32(r.Int("SECTORS")),
			Serial:      uint32(r.Int("SERIAL")),
			VolName:     r.String("VOLNAME"),
			MediaName:   r.String("MEDIANAME"),
			MediaType:   r.String("MEDIATYPE"),
			RootDevName: r.String("ROOTDEVNAME"),
		})

		return nil
	})

	// Phase 25: DEFINE, ASSIGN, DEASSIGN, CREATE/NAME_TABLE, SHOW LOGICAL
	// and SHOW TRANSLATION (logical.go).
	bindLogicalCommands(g, d.Console)

	// Phase 22 (internal/rms): MOUNT/DISMOUNT attach/detach a disk-image
	// container file to a device name via internal/rms.MountTable
	// (Console.Mount/Dismount, internal/dclcmd/mount.go). Neither verb has
	// a reference/eVAX or testdata/dcl/console.dcl counterpart -- see this
	// package's own grammar file (internal/bootdata/files/console.dcl)'s
	// "govax-native extension" comment at MOUNT's definition.
	g.Bind("MOUNT", func(id int64, r *dcl.Result) error {
		// The WRITE qualifier defaults to present (a bare MOUNT is
		// writable, matching Console.Mount's own doc comment): only an
		// explicit /NOWRITE -- r.Negated, not r.Present, since an
		// unspecified qualifier is "absent", not "negated" (see
		// dcl.Result.Negated's own doc comment) -- turns it off. Real
		// VMS's /NOWRITE reaches WRITE this same way, as the grammar's
		// automatic "NO"-prefix negation of the one declared WRITE
		// qualifier (this file's own DCL grammar comment, and
		// TestParse_mountNowrite in internal/dcl/parse_test.go).
		write := !r.Negated("WRITE")

		return d.Console.cmd().MountCommand(r.String("DEVICE"), r.String("FILE"), write)
	})

	g.Bind("DISMOUNT", func(id int64, r *dcl.Result) error {
		return d.Console.cmd().Dismount(r.String("DEVICE"))
	})

	// Phase 23 (docs/PHASE-23.md, subtask 1): INITIALIZE/VAX carries
	// forward the old fixed-table INIT command's exact behavior (cmdInit,
	// removed) now that INIT/INITIALIZE are unified onto this grammar as
	// one verb, redirected by /VAX vs. /CONTAINER. PAGES has no /prompt= in
	// the grammar (see console.dcl's own comment on initialize_vax), so a
	// missing page count is checked explicitly here rather than triggering
	// a formal-requirement error with different wording -- preserving
	// CLI_NEEDPAGES's exact original message either way.
	g.Bind("INITIALIZE_VAX", func(id int64, r *dcl.Result) error {
		if !r.Present("PAGES") {
			return vmserrors.New(vmserrors.CLI_NEEDPAGES)
		}

		v, _, err := (&machine.Evaluator{Symbols: d.Console.Symbols, Radix: d.Console.Radix, Mem: d.Console.Mem, CPU: d.Console.CPU}).Eval(r.String("PAGES"))
		if err != nil {
			return vmserrors.Wrap(vmserrors.CLI_NEEDPAGES, err)
		}

		return d.Console.Init(v * 512)
	})

	// Phase 23 (docs/PHASE-23.md, subtask 4): INITIALIZE/CONTAINER formats
	// a brand-new, empty ODS-2 volume via internal/rms.InitializeContainer
	// (Console.InitializeContainer, internal/console/initialize.go). PATH
	// and SIZE both carry /prompt= in the grammar (console.dcl's own
	// initialize_container syntax), so -- unlike INITIALIZE_VAX's PAGES --
	// Grammar.Dispatch's own prompting/required-parameter machinery already
	// guarantees they're present by the time this closure runs; LABEL and
	// CLUSTER are genuinely optional (r.String/r.Int's own zero values --
	// "" and 0 -- are exactly what Console.InitializeContainer treats as
	// "use the default" already, so no explicit r.Present check is needed
	// for either).
	g.Bind("INITIALIZE_CONTAINER", func(id int64, r *dcl.Result) error {
		return d.Console.cmd().InitializeContainer(
			r.String("PATH"),
			uint32(r.Int("SIZE")),
			r.String("LABEL"),
			uint16(r.Int("CLUSTER")),
			r.String("DEVICE"),
		)
	})

	// Phase 23 (docs/PHASE-23.md, subtask 5): DIRECTORY lists the files on
	// a mounted volume via internal/rms.Session.Directory (Console.
	// Directory, internal/dclcmd/directory.go). SPEC carries no /prompt=
	// in the grammar (console.dcl's own directory verb), so a bare DIRECTORY
	// with nothing typed after it reaches here with r.String("SPEC") == ""
	// -- exactly the "list the whole current default directory" case
	// Console.Directory's own doc comment describes, not a missing
	// argument.
	g.Bind("DIRECTORY", func(id int64, r *dcl.Result) error {
		opts := rms.DirectoryOptions{
			Full: r.Present("FULL"),
			File: r.Present("FILE"),
			Size: r.Present("SIZE"),
			Date: r.Present("DATE"),

			Owner:      r.Present("OWNER"),
			Protection: r.Present("PROTECTION"),
		}

		// /VERSIONS=n lists only each file's n newest versions; its value
		// is required (the grammar's $integer type) and must be at least
		// 1, since an absent /VERSIONS is what lists every version.
		if r.Present("VERSIONS") {
			n := r.Int("VERSIONS")
			if n < 1 {
				return vmserrors.New(vmserrors.CLI_BADVERSIONS, n)
			}

			opts.Versions = int(n)
		}

		// VMS's DIRECTORY image ends with SS$_NORMAL (VMS 7.3,
		// testdata/dcl50's round 5).
		if err := d.Console.cmd().Directory(r.String("SPEC"), opts); err != nil {
			return err
		}

		d.Console.setCommandStatus(ssNormal, nil)

		return nil
	})

	// Phase 23 (docs/PHASE-23.md, subtask 6): DELETE reclaims a file's
	// storage via internal/rms.Session.Delete (Console.Delete, internal/
	// console/delete.go). SPEC carries /prompt= in the grammar (console.dcl's
	// own delete verb), so Grammar.Dispatch's own required-parameter
	// machinery already guarantees it's present by the time this closure
	// runs -- unlike DIRECTORY's SPEC, a bare DELETE has no sensible
	// "delete everything" default to fall back to.
	g.Bind("DELETE", func(id int64, r *dcl.Result) error {
		return d.Console.cmd().Delete(r.String("SPEC"))
	})

	// Phase 23 (docs/PHASE-23.md, subtask 7): PURGE trims old versions via
	// internal/rms.Session.Purge (Console.Purge, internal/dclcmd/purge.go).
	// LIMIT carries no /prompt= in the grammar (console.dcl's own purge verb),
	// so an omitted /LIMIT reaches here as r.Present("LIMIT") == false --
	// resolved to the default of 1 (ods2's own cmdPurge convention) here,
	// rather than in internal/rms.Session.Purge itself, since r.Int's own
	// zero value can't be told apart from an explicit, invalid /LIMIT=0
	// (which Session.Purge does reject, as *rms.InvalidLimitError).
	// docs/PHASE-22.md subtask 18: RENAME renames or moves files on a
	// mounted volume (Console.Rename, internal/dclcmd/rename.go). A
	// negated /NONEW_VERSION is the only way to turn NEW_VERSION off.
	g.Bind("RENAME", func(id int64, r *dcl.Result) error {
		newVersion := !(r.Present("NEW_VERSION") && r.Negated("NEW_VERSION"))

		return d.Console.cmd().Rename(r.List("INPUTS"), r.String("OUTPUT"), r.Present("LOG"), newVersion)
	})

	g.Bind("PURGE", func(id int64, r *dcl.Result) error {
		limit := uint16(1)
		if r.Present("LIMIT") {
			limit = uint16(r.Int("LIMIT"))
		}

		return d.Console.cmd().Purge(r.String("SPEC"), limit)
	})

	// Phase 23 (docs/PHASE-23.md, subtask 8): TYPE writes one file's
	// content to the console via internal/rms.Session.Type (Console.Type,
	// internal/dclcmd/type.go). SPEC carries /prompt= in the grammar
	// (console.dcl's own type verb), so Grammar.Dispatch's own required-
	// parameter machinery already guarantees it's present by the time this
	// closure runs. /PAGE shows it a screenful at a time; /HOST makes SPEC
	// a host file name.
	// VMS's TYPE image ends with SS$_NORMAL, its message marked as shown
	// (%X10000001: VMS 7.3, testdata/dcl50's round 5).
	g.Bind("TYPE", func(id int64, r *dcl.Result) error {
		if err := d.Console.cmd().Type(r.String("SPEC"), r.Present("HOST"), r.Present("PAGE")); err != nil {
			return err
		}

		d.Console.setCommandStatus(ssNormal|stsInhibitMsg, nil)

		return nil
	})

	// Phase 23 (docs/PHASE-23.md, subtasks 9-10): COPY moves one or more
	// files' content between a mounted volume and the host filesystem, or
	// between two mounted volumes, via internal/rms.Session.Copy
	// (Console.Copy, internal/dclcmd/copy.go). SOURCE and DESTINATION
	// each carry their own private HOST qualifier (console.dcl's own copy
	// verb, using the parameter-scoped-qualifier grammar feature from
	// subtask 2), read here via r.ParamPresent(paramName, "HOST") rather
	// than the ordinary entry-level r.Present -- see internal/console/
	// dcl's parse.go/grammar.go for how that resolution works. Every
	// other qualifier is entry-level and folds into one rms.CopyOptions,
	// threaded straight through Console.Copy into Session.Copy unchanged
	// -- see CopyOptions' own doc comment for which direction(s) each one
	// actually affects. VFC (r.Present("VFC")) is read here only to
	// document that it's intentionally discarded: this project's default
	// text-mode copy already always expands VFC carriage control the way
	// TYPE does, so there is no "un-interpreted" mode /VFC could opt out
	// of -- see CopyOptions' own doc comment for the fuller reasoning.
	// /CRLF and /LF's mutual exclusivity is enforced by console.dcl's own
	// "disallow crlf and lf" grammar statement, so this closure never
	// needs to check for both at once itself.
	g.Bind("COPY", func(id int64, r *dcl.Result) error {
		_ = r.Present("VFC") // accepted for compatibility, never consulted -- see comment above

		opts := rms.CopyOptions{
			Binary:  r.Present("BINARY"),
			Quiet:   r.Present("QUIET"),
			Verbose: r.Present("VERBOSE"),
			Test:    r.Present("TEST"),
			Time:    r.Present("TIME"),
			Ignore:  r.Present("IGNORE"),
			Dirs:    r.Present("DIRS"),
			Stream:  r.Present("STREAM"),
			CRLF:    r.Present("CRLF"),
		}

		return d.Console.cmd().Copy(
			r.String("SOURCE"), r.ParamPresent("SOURCE", "HOST"),
			r.String("DESTINATION"), r.ParamPresent("DESTINATION", "HOST"),
			opts,
		)
	})

	// docs/PHASE-27.md subtask 10: MACRO assembles a MACRO-32 source into
	// an object module (Console.Macro, internal/dclcmd/macro.go). The
	// command as typed goes into the object's SRC header, as real MACRO
	// records its command line.
	// docs/PHASE-30.md: LINK links object modules into an executable
	// image (Console.Link, internal/dclcmd/link.go).
	g.Bind("LINK", func(id int64, r *dcl.Result) error {
		files := r.List("OBJECTS")
		items := r.Items("OBJECTS")
		inputs := make([]link.InputFile, len(files))

		for i, name := range files {
			inputs[i] = link.InputFile{
				Name:      name,
				Library:   items[i].Present("LIBRARY"),
				Include:   items[i].List("INCLUDE"),
				Selective: items[i].Present("SELECTIVE_SEARCH"),
				Options:   items[i].Present("OPTIONS"),
			}
		}

		return d.Console.cmd().Link(dclcmd.LinkOptions{
			Files:        inputs,
			Host:         r.ParamPresent("OBJECTS", "HOST"),
			Executable:   r.String("EXECUTABLE"),
			NoExecutable: r.Present("EXECUTABLE") && r.Negated("EXECUTABLE"),
			NoTraceback:  r.Present("TRACEBACK") && r.Negated("TRACEBACK"),
			Debug:        r.Present("DEBUG") && !r.Negated("DEBUG") && !r.Defaulted("DEBUG"),
			DebugModule:  r.String("DEBUG"),
			NoSysLib:     r.Present("SYSLIB") && r.Negated("SYSLIB"),
			Map:          r.Present("MAP") && !r.Negated("MAP") && !r.Defaulted("MAP"),
			MapFile:      r.String("MAP"),
			Brief:        r.Present("BRIEF"),
		})
	})

	// docs/PHASE-38.md: ANALYZE/OBJECT describes object files
	// (Console.AnalyzeObject, internal/dclcmd/analyze.go). Each kind of
	// analysis is a qualifier with its own syntax; a bare ANALYZE names
	// none.
	g.Bind("ANALYZE", func(id int64, r *dcl.Result) error {
		return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "/OBJECT or /IMAGE")
	})

	// docs/PHASE-40.md: ANALYZE/IMAGE describes image files
	// (Console.AnalyzeImage).
	g.Bind("ANALYZE_IMAGE", func(id int64, r *dcl.Result) error {
		return d.Console.cmd().AnalyzeImage(dclcmd.AnalyzeOptions{
			Files:       r.List("FILES"),
			Host:        r.ParamPresent("FILES", "HOST"),
			Output:      r.Present("OUTPUT") && !r.Negated("OUTPUT") && !r.Defaulted("OUTPUT"),
			OutputFile:  r.String("OUTPUT"),
			Header:      r.Present("HEADER") && !r.Negated("HEADER"),
			Fixups:      r.Present("FIXUP_SECTION") && !r.Negated("FIXUP_SECTION"),
			CommandLine: d.Console.line,
		})
	})

	g.Bind("ANALYZE_OBJECT", func(id int64, r *dcl.Result) error {
		selected := dclcmd.AnalyzeRecordTypes(func(name string) bool { return r.Present(name) && !r.Negated(name) })

		return d.Console.cmd().AnalyzeObject(dclcmd.AnalyzeOptions{
			Files:       r.List("FILES"),
			Host:        r.ParamPresent("FILES", "HOST"),
			Output:      r.Present("OUTPUT") && !r.Negated("OUTPUT") && !r.Defaulted("OUTPUT"),
			OutputFile:  r.String("OUTPUT"),
			Include:     r.Present("INCLUDE") && !r.Negated("INCLUDE") && !r.Defaulted("INCLUDE"),
			Modules:     nonEmpty(r.List("INCLUDE")),
			Select:      selected,
			CommandLine: d.Console.line,
		})
	})

	// docs/PHASE-28.md subtask 7: LIBRARY creates, changes, extracts
	// from, and lists libraries (Console.Library, internal/console/
	// library.go).
	g.Bind("LIBRARY", func(id int64, r *dcl.Result) error {
		return d.Console.cmd().Library(dclcmd.LibraryOptions{
			Library:     r.String("LIBRARY"),
			LibraryHost: r.ParamPresent("LIBRARY", "HOST"),
			Inputs:      r.List("INPUTS"),
			InputHost:   r.ParamPresent("INPUTS", "HOST"),
			Create:      r.Present("CREATE"),
			Insert:      r.Present("INSERT"),
			Replace:     r.Present("REPLACE"),
			Delete:      r.List("DELETE"),
			Extract:     r.List("EXTRACT"),
			Output:      r.String("OUTPUT"),
			List:        r.Present("LIST") && !r.Negated("LIST") && !r.Defaulted("LIST"),
			ListFile:    r.String("LIST"),
			Full:        r.Present("FULL"),
			Names:       r.Present("NAMES"),
			Width:       int(r.Int("WIDTH")),
			Macro:       r.Present("MACRO"),
			Object:      r.Present("OBJECT"),
			NoSqueeze:   r.Present("SQUEEZE") && r.Negated("SQUEEZE"),
			Selective:   r.Present("SELECTIVE_SEARCH"),
			Log:         r.Present("LOG"),
		})
	})

	g.Bind("MACRO", func(id int64, r *dcl.Result) error {
		// /SHOW=(...) and /NOSHOW=(...) are one qualifier, negated or
		// not.
		var show, noshow []string
		if r.Negated("SHOW") {
			noshow = r.List("SHOW")
		} else {
			show = r.List("SHOW")
		}

		return d.Console.cmd().Macro(dclcmd.MacroOptions{
			Source:      r.String("SOURCE"),
			SourceHost:  r.ParamPresent("SOURCE", "HOST"),
			Object:      r.String("OBJECT"),
			NoObject:    r.Present("OBJECT") && r.Negated("OBJECT"),
			List:        r.Present("LIST") && !r.Negated("LIST") && !r.Defaulted("LIST"),
			ListFile:    r.String("LIST"),
			Show:        show,
			NoShow:      noshow,
			Xref:        r.Present("CROSS_REFERENCE") && !r.Negated("CROSS_REFERENCE") && !r.Defaulted("CROSS_REFERENCE"),
			XrefKinds:   r.List("CROSS_REFERENCE"),
			Enable:      r.List("ENABLE"),
			Disable:     r.List("DISABLE"),
			Debug:       r.Present("DEBUG") && !r.Negated("DEBUG") && !r.Defaulted("DEBUG"),
			DebugKinds:  r.List("DEBUG"),
			NoDebug:     r.Present("DEBUG") && r.Negated("DEBUG"),
			Libraries:   r.List("LIBRARY"),
			CommandLine: d.Console.line,
		})
	})
}

// nonEmpty drops empty strings from a list: a list qualifier given
// without a value (/INCLUDE) has its empty default.
func nonEmpty(list []string) []string {
	var out []string

	for _, s := range list {
		if s != "" {
			out = append(out, s)
		}
	}

	return out
}

// isDecimalDigit reports whether ch is a decimal digit.
func isDecimalDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

// DispatchConsole parses and executes one console command line -- DCL
// symbols and the console grammar -- whatever mode the front end is in
// (Interpreter.DispatchConsole).
func (d *Dispatcher) DispatchConsole(line string) error {
	return d.Console.Interpreter.DispatchConsole(line)
}
