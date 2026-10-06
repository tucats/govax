package console

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// Dispatcher parses one command line with the DCL grammar and runs the
// handler bound to the verb or syntax it ends in.
//
// console_dispatch.c has two tiers: console_dispatch_table's "fixed"
// commands, matched by their first four characters and parsed by hand
// (EXAMINE, SET, STEP, ...), and the DCL grammar for the rest (SHOW,
// CLEAR, VMINIT, ...). This port kept that split until docs/PHASE-37.md
// moved every fixed command onto the grammar (commands.go,
// setcommand.go); their old spellings ("EX", "D", "G", "@", ...) are
// grammar verbs or aliases now. Two of them differ from the C source:
// DEPOSIT (exam.go) is a Go-native addition, and RUN/R means what it does
// in the C source (run.go's Console.Run, Phase 13) — VMS image
// activation, not plain CPU execution (that's EXECUTE/GO/G).
type Dispatcher struct {
	Console *Console
	Grammar *dcl.Grammar
	Help    *Help

	// line is the command line being dispatched, for a handler that
	// records it (MACRO's SRC header).
	line string

	// symbolDepth is how many DCL symbol substitutions the command being
	// dispatched has been through (dclsym.go).
	symbolDepth int
}

// NewDispatcher returns a Dispatcher wired to c and g, with every DCL
// verb/syntax this port implements bound to its handler.
func NewDispatcher(c *Console, g *dcl.Grammar, h *Help) *Dispatcher {
	d := &Dispatcher{Console: c, Grammar: g, Help: h}
	d.bindGrammar()

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

	// console_dispatch.c checks vax.console.assembler_mode before ever
	// reading a verb: while interactive ASM mode (docs/PHASE-19.md) is on,
	// every line -- including one that happens to spell a command name --
	// is a statement for the assembler, not a console command.
	if d.Console.assemblerMode {
		return d.assembleInteractiveLine(line)
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

	return d.DispatchConsole(line)
}

// DispatchConsole parses and executes one console command line -- DCL
// symbols and the console grammar -- whatever mode the front end is in.
// Dispatch calls it when no debugger session is active; XFC$CONSOLE_CMD
// (Console.ConsoleCommand) calls it always.
func (d *Dispatcher) DispatchConsole(line string) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "!") {
		return nil
	}

	// Interactive assembler mode, as in Dispatch, for the callers that
	// come straight here.
	if d.Console.assemblerMode {
		return d.assembleInteractiveLine(line)
	}

	// A symbol assignment, DELETE/SYMBOL, or a command whose first word
	// is a DCL symbol (a foreign command or an alias): DCL looks for a
	// symbol before a verb (dclsym.go).
	if handled, err := d.dclSymbolLine(line); handled {
		return err
	}

	// DBG_DCL: console_dispatch.c:121 sets the third-party DCL parser
	// library's own verbosity knob (DCLsetdebug) before falling through to
	// it. internal/console/dcl is this port's own grammar interpreter, not
	// a wrapped external library with a separate verbosity knob to set, so
	// this traces the line being handed to it instead -- the closest
	// equivalent visibility this port can offer. See docs/PHASE-17.md
	// sub-phase 5.
	if d.Console.CPU != nil && d.Console.CPU.DebugEnabled(vax.DebugDCL) {
		fmt.Fprintf(d.Console.CPU.DebugWriter(), "DEBUG(DCL): parsing %q\n", line)
	}

	r, err := d.Grammar.Parse(line)
	if err != nil {
		return err
	}

	d.line = line

	// A DCL /entry= redirect (ABOUT, XTEST, SHOW VERSION -- the C
	// source's exe$about/exe$xtest, all real VAX routines
	// defined in kernel.asm itself, not native C functions -- see
	// docs/PHASE-16.md sub-phase 4) resolves the entry name as a VAX symbol
	// (populated by an .ENTRY once kernel.asm has been ASMed/booted) and
	// CALLs it with no arguments, the same primitive the CALL command
	// itself uses. Requires a booted microkernel exactly as the C source
	// does (exe$about etc. simply don't exist otherwise).
	if r.EntryPoint != "" {
		addr, _, err := d.Console.Evaluator().Eval(r.EntryPoint)
		if err != nil {
			return err
		}

		return d.Console.Call(addr, false)
	}

	return d.Grammar.Dispatch(r)
}

// readCommandVerb reads the leading command word (up to whitespace, '/',
// ',', or '='), matching read_verb — including its "a leading '@' is a
// token by itself" special case (the INCLUDE-file shorthand). DCL symbol
// lookup (dclsym.go) uses it, and IF and SET PTE use it for their THEN
// and TO words.
func readCommandVerb(s string) (verb, rest string) {
	if strings.HasPrefix(s, "@") {
		return "@", s[1:]
	}

	i := 0
	for i < len(s) {
		switch s[i] {
		case ' ', '\t', '/', ',', '=':
			return s[:i], s[i:]
		}

		i++
	}

	return s, ""
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

	g.Bind("EXIT", func(id int64, r *dcl.Result) error { return d.Console.Quit() })
	g.Bind("DEBUG", func(id int64, r *dcl.Result) error { return d.Console.StartDebugger() })

	g.Bind("VMINIT", func(id int64, r *dcl.Result) error {
		return d.Console.VMInit(
			uint32(r.Int("P0")), uint32(r.Int("P1")), uint32(r.Int("S0")),
			uint32(r.Int("KSP")), uint32(r.Int("ESP")), uint32(r.Int("SSP")), uint32(r.Int("ISP")),
			uint32(r.Int("STRINGPOOL")),
		)
	})

	g.Bind("CLEAR_SYM_ALL", func(id int64, r *dcl.Result) error { return d.Console.ClearSymbol("", true) })
	g.Bind("CLEAR_SYMBOLS", func(id int64, r *dcl.Result) error { return d.Console.ClearSymbol(r.String("P1"), false) })

	g.Bind("CLEAR_SYM_TEMP", func(id int64, r *dcl.Result) error { return d.Console.ClearSymbolTemporary() })
	g.Bind("CLEAR_STRINGS", func(id int64, r *dcl.Result) error { return d.Console.ClearString() })
	g.Bind("CLEAR_MEMORY", func(id int64, r *dcl.Result) error { return d.Console.ClearMemory() })


	g.Bind("SHOW_SYM", func(id int64, r *dcl.Result) error { return d.Console.ShowSymbol(r.String("SYMBOL")) })
	g.Bind("SHOW_SYM_ALL", func(id int64, r *dcl.Result) error { return d.Console.ShowSymbols(r.String("SYMBOL")) })
	g.Bind("SHOW_SYM_SYS", func(id int64, r *dcl.Result) error { return d.Console.ShowSymbolsSystem(r.String("SYMBOL")) })
	g.Bind("SHOW_SYM_DCL", func(id int64, r *dcl.Result) error { return d.Console.ShowDCLSymbols(r.String("SYMBOL")) })
	g.Bind("SHOW_RADIX", func(id int64, r *dcl.Result) error { return d.Console.ShowRadix() })
	// SHOW VERSION has no bind of its own: its grammar syntax carries
	// /entry=exe$about (evax.dcl's own "syntax show_version/entry=exe$about"
	// — it shares ABOUT's real VAX routine), so Dispatch's EntryPoint check
	// above reaches it before Grammar.Dispatch ever would. The Go-native
	// ShowVersion stand-in this bind used to call (docs/PHASE-08.md's own
	// "rather than leaving ABOUT/SHOW VERSION with no output at all") is
	// retired now that the real /entry= redirect works.


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
	g.Bind("SHOW_DEFAULT", func(id int64, r *dcl.Result) error { return d.Console.ShowDefault() })

	g.Bind("SHOW_QUANTUM", func(id int64, r *dcl.Result) error { return d.Console.ShowQuantum() })

	g.Bind("SHOW_DEBUG", func(id int64, r *dcl.Result) error { return d.Console.ShowDebug() })

	g.Bind("SHOW_INSTRUCTIONS", func(id int64, r *dcl.Result) error {
		return d.Console.ShowInstructions(
			r.Present("MODES"), r.Present("PROFILE"), r.Present("UNIMPLEMENTED"), r.Present("ALL"),
			r.String("OPCODE"),
		)
	})


	// Phase 09 (internal/io): device abstraction and logical name tables.
	g.Bind("SHOW_DEVICE", func(id int64, r *dcl.Result) error {
		return d.Console.ShowDevices(r.String("NAME"), r.Present("FULL"))
	})

	g.Bind("DEFINE_DEVICE", func(id int64, r *dcl.Result) error {
		d.Console.DefineDevice(r.String("NAME"), iodev.DeviceOptions{
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
	// (Console.Mount/Dismount, internal/console/mount.go). Neither verb has
	// a reference/eVAX or testdata/dcl/evax.dcl counterpart -- see this
	// package's own grammar file (internal/bootdata/files/evax.dcl)'s
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
		// TestParse_mountNowrite in internal/console/dcl/parse_test.go).
		write := !r.Negated("WRITE")

		return d.Console.MountCommand(r.String("DEVICE"), r.String("FILE"), write)
	})

	g.Bind("DISMOUNT", func(id int64, r *dcl.Result) error {
		return d.Console.Dismount(r.String("DEVICE"))
	})

	// Phase 23 (docs/PHASE-23.md, subtask 1): INITIALIZE/VAX carries
	// forward the old fixed-table INIT command's exact behavior (cmdInit,
	// removed) now that INIT/INITIALIZE are unified onto this grammar as
	// one verb, redirected by /VAX vs. /CONTAINER. PAGES has no /prompt= in
	// the grammar (see evax.dcl's own comment on initialize_vax), so a
	// missing page count is checked explicitly here rather than triggering
	// a formal-requirement error with different wording -- preserving
	// CLI_NEEDPAGES's exact original message either way.
	g.Bind("INITIALIZE_VAX", func(id int64, r *dcl.Result) error {
		if !r.Present("PAGES") {
			return vmserrors.New(vmserrors.CLI_NEEDPAGES)
		}

		v, _, err := (&Evaluator{Symbols: d.Console.Symbols, Radix: d.Console.Radix, Mem: d.Console.Mem, CPU: d.Console.CPU}).Eval(r.String("PAGES"))
		if err != nil {
			return vmserrors.Wrap(vmserrors.CLI_NEEDPAGES, err)
		}

		return d.Console.Init(v * 512)
	})

	// Phase 23 (docs/PHASE-23.md, subtask 4): INITIALIZE/CONTAINER formats
	// a brand-new, empty ODS-2 volume via internal/rms.InitializeContainer
	// (Console.InitializeContainer, internal/console/initialize.go). PATH
	// and SIZE both carry /prompt= in the grammar (evax.dcl's own
	// initialize_container syntax), so -- unlike INITIALIZE_VAX's PAGES --
	// Grammar.Dispatch's own prompting/required-parameter machinery already
	// guarantees they're present by the time this closure runs; LABEL and
	// CLUSTER are genuinely optional (r.String/r.Int's own zero values --
	// "" and 0 -- are exactly what Console.InitializeContainer treats as
	// "use the default" already, so no explicit r.Present check is needed
	// for either).
	g.Bind("INITIALIZE_CONTAINER", func(id int64, r *dcl.Result) error {
		return d.Console.InitializeContainer(
			r.String("PATH"),
			uint32(r.Int("SIZE")),
			r.String("LABEL"),
			uint16(r.Int("CLUSTER")),
			r.String("DEVICE"),
		)
	})

	// Phase 23 (docs/PHASE-23.md, subtask 5): DIRECTORY lists the files on
	// a mounted volume via internal/rms.Session.Directory (Console.
	// Directory, internal/console/directory.go). SPEC carries no /prompt=
	// in the grammar (evax.dcl's own directory verb), so a bare DIRECTORY
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

		return d.Console.Directory(r.String("SPEC"), opts)
	})

	// Phase 23 (docs/PHASE-23.md, subtask 6): DELETE reclaims a file's
	// storage via internal/rms.Session.Delete (Console.Delete, internal/
	// console/delete.go). SPEC carries /prompt= in the grammar (evax.dcl's
	// own delete verb), so Grammar.Dispatch's own required-parameter
	// machinery already guarantees it's present by the time this closure
	// runs -- unlike DIRECTORY's SPEC, a bare DELETE has no sensible
	// "delete everything" default to fall back to.
	g.Bind("DELETE", func(id int64, r *dcl.Result) error {
		return d.Console.Delete(r.String("SPEC"))
	})

	// Phase 23 (docs/PHASE-23.md, subtask 7): PURGE trims old versions via
	// internal/rms.Session.Purge (Console.Purge, internal/console/purge.go).
	// LIMIT carries no /prompt= in the grammar (evax.dcl's own purge verb),
	// so an omitted /LIMIT reaches here as r.Present("LIMIT") == false --
	// resolved to the default of 1 (ods2's own cmdPurge convention) here,
	// rather than in internal/rms.Session.Purge itself, since r.Int's own
	// zero value can't be told apart from an explicit, invalid /LIMIT=0
	// (which Session.Purge does reject, as *rms.InvalidLimitError).
	// docs/PHASE-22.md subtask 18: RENAME renames or moves files on a
	// mounted volume (Console.Rename, internal/console/rename.go). A
	// negated /NONEW_VERSION is the only way to turn NEW_VERSION off.
	g.Bind("RENAME", func(id int64, r *dcl.Result) error {
		newVersion := !(r.Present("NEW_VERSION") && r.Negated("NEW_VERSION"))

		return d.Console.Rename(r.List("INPUTS"), r.String("OUTPUT"), r.Present("LOG"), newVersion)
	})

	g.Bind("PURGE", func(id int64, r *dcl.Result) error {
		limit := uint16(1)
		if r.Present("LIMIT") {
			limit = uint16(r.Int("LIMIT"))
		}

		return d.Console.Purge(r.String("SPEC"), limit)
	})

	// Phase 23 (docs/PHASE-23.md, subtask 8): TYPE writes one file's
	// content to the console via internal/rms.Session.Type (Console.Type,
	// internal/console/type.go). SPEC carries /prompt= in the grammar
	// (evax.dcl's own type verb), so Grammar.Dispatch's own required-
	// parameter machinery already guarantees it's present by the time this
	// closure runs.
	g.Bind("TYPE", func(id int64, r *dcl.Result) error {
		return d.Console.Type(r.String("SPEC"))
	})

	// Phase 23 (docs/PHASE-23.md, subtasks 9-10): COPY moves one or more
	// files' content between a mounted volume and the host filesystem, or
	// between two mounted volumes, via internal/rms.Session.Copy
	// (Console.Copy, internal/console/copy.go). SOURCE and DESTINATION
	// each carry their own private HOST qualifier (evax.dcl's own copy
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
	// /CRLF and /LF's mutual exclusivity is enforced by evax.dcl's own
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

		return d.Console.Copy(
			r.String("SOURCE"), r.ParamPresent("SOURCE", "HOST"),
			r.String("DESTINATION"), r.ParamPresent("DESTINATION", "HOST"),
			opts,
		)
	})

	// docs/PHASE-27.md subtask 10: MACRO assembles a MACRO-32 source into
	// an object module (Console.Macro, internal/console/macro.go). The
	// command as typed goes into the object's SRC header, as real MACRO
	// records its command line.
	// docs/PHASE-30.md: LINK links object modules into an executable
	// image (Console.Link, internal/console/link.go).
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

		return d.Console.Link(LinkOptions{
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
	// (Console.AnalyzeObject, internal/console/analyze.go). Each kind of
	// analysis is a qualifier with its own syntax; a bare ANALYZE names
	// none.
	g.Bind("ANALYZE", func(id int64, r *dcl.Result) error {
		return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "/OBJECT or /IMAGE")
	})

	// docs/PHASE-40.md: ANALYZE/IMAGE describes image files
	// (Console.AnalyzeImage).
	g.Bind("ANALYZE_IMAGE", func(id int64, r *dcl.Result) error {
		return d.Console.AnalyzeImage(AnalyzeOptions{
			Files:       r.List("FILES"),
			Host:        r.ParamPresent("FILES", "HOST"),
			Output:      r.Present("OUTPUT") && !r.Negated("OUTPUT") && !r.Defaulted("OUTPUT"),
			OutputFile:  r.String("OUTPUT"),
			Header:      r.Present("HEADER") && !r.Negated("HEADER"),
			Fixups:      r.Present("FIXUP_SECTION") && !r.Negated("FIXUP_SECTION"),
			CommandLine: d.line,
		})
	})

	g.Bind("ANALYZE_OBJECT", func(id int64, r *dcl.Result) error {
		var selected []obj.RecordType

		for _, q := range analyzeRecordQualifiers {
			if r.Present(q.name) && !r.Negated(q.name) {
				selected = append(selected, q.types...)
			}
		}

		return d.Console.AnalyzeObject(AnalyzeOptions{
			Files:       r.List("FILES"),
			Host:        r.ParamPresent("FILES", "HOST"),
			Output:      r.Present("OUTPUT") && !r.Negated("OUTPUT") && !r.Defaulted("OUTPUT"),
			OutputFile:  r.String("OUTPUT"),
			Include:     r.Present("INCLUDE") && !r.Negated("INCLUDE") && !r.Defaulted("INCLUDE"),
			Modules:     nonEmpty(r.List("INCLUDE")),
			Select:      selected,
			CommandLine: d.line,
		})
	})

	// docs/PHASE-28.md subtask 7: LIBRARY creates, changes, extracts
	// from, and lists libraries (Console.Library, internal/console/
	// library.go).
	g.Bind("LIBRARY", func(id int64, r *dcl.Result) error {
		return d.Console.Library(LibraryOptions{
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

		return d.Console.Macro(MacroOptions{
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
			CommandLine: d.line,
		})
	})
}

// assembleInteractiveLine hands one line to Console.AssembleInteractiveLine
// while InAssemblerMode is true, matching asmCommand's own "hasEntry ->
// CALL __ENTRY" post-command hook for whichever statement (bare or dotted
// END) finally stops assembly.
func (d *Dispatcher) assembleInteractiveLine(line string) error {
	done, entryAddr, hasEntry, err := d.Console.AssembleInteractiveLine(line)
	if err != nil {
		return err
	}

	if done && hasEntry {
		return d.Console.Call(entryAddr, false)
	}

	return nil
}
