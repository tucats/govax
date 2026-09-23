package console

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// Dispatcher routes one command line to either a fixed-spelling handler
// (matching console_dispatch_table's exact, <=4-character verb spellings —
// e.g. "EXAM"/"EX"/"DUMP" all reach EXAMINE, since read_verb only ever
// looks at a token's first 4 characters) or, if no fixed spelling matches,
// the DCL grammar — matching console_dispatch's own two-tier fallback.
//
// The fixed-spelling set and DCL/verb split here follows console_dispatch_
// table's own -1-vs-real-function split (SHOW, EXIT/QUIT, CLEAR, TEST,
// VMINIT are DCL-driven; EXAMINE, SET, STEP, ... are fixed) with a few
// deliberate deviations, documented where they're implemented: DEPOSIT
// (exam.go) is a Go-native addition with no C-source command of its own.
// RUN/R now means what it does in the C source (see run.go's Console.Run,
// Phase 13) — VMS executable-image activation, not plain CPU execution
// (that's EXEC/GO/G, unaffected). INIT — a real-function fixed entry in the
// C source — moved onto the DCL grammar in Phase 23 (see bindGrammar's
// INITIALIZE_VAX bind) as part of unifying it with the new, govax-native
// INITIALIZE/CONTAINER under one verb; it's no longer in fixedCommands
// below, reaching INITIALIZE_VAX purely via Grammar.matchVerb's
// unambiguous-prefix matching instead.
type Dispatcher struct {
	Console *Console
	Grammar *dcl.Grammar
	Help    *Help
}

// NewDispatcher returns a Dispatcher wired to c and g, with every DCL
// verb/syntax this port implements bound to its handler.
func NewDispatcher(c *Console, g *dcl.Grammar, h *Help) *Dispatcher {
	d := &Dispatcher{Console: c, Grammar: g, Help: h}
	d.bindGrammar()

	return d
}

// Dispatch parses and executes one command line, matching console_
// dispatch's own read-verb/fixed-table/DCL-fallback structure.
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

	verb, _ := readCommandVerb(line)

	verb4 := strings.ToUpper(verb)
	if len(verb4) > 4 {
		verb4 = verb4[:4]
	}

	if h, ok := fixedCommands[verb4]; ok {
		_, rest := readCommandVerb(line)

		return h(d, rest)
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

	// A DCL /entry= redirect (ABOUT, FORTH, XTEST, SHOW VERSION -- the C
	// source's exe$about/exe$forth_dcl/exe$xtest, all real VAX routines
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
// token by itself" special case (the INCLUDE-file shorthand).
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

// parseHexOrEmpty parses a SHOW STACK-family "count" parameter, matching
// console_show.c's own asm_hex parse of it (always hexadecimal,
// independent of the console's current default radix); "" (the parameter
// wasn't supplied) returns 0, meaning "use the default".
func parseHexOrEmpty(s string) (uint32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, vmserrors.New(vmserrors.CLI_BADHEXVAL, s)
	}

	return uint32(v), nil
}

// bindGrammar binds every DCL verb/syntax this port implements a handler
// for. Everything else (device/RTL/assembler-dependent SHOW/CLEAR/DEFINE
// sub-forms, TEST, CALL's DCL entry — unreachable anyway since CALL is
// intercepted by the fixed table first) is deliberately left unbound:
// Grammar.Dispatch's own "no handler bound" error already reports that
// clearly, so no separate stub code is needed for each one.
func (d *Dispatcher) bindGrammar() {
	g := d.Grammar

	g.Bind("EXIT", func(id int64, r *dcl.Result) error { return d.Console.Quit() })

	g.Bind("VMINIT", func(id int64, r *dcl.Result) error {
		return d.Console.VMInit(
			uint32(r.Int("P0")), uint32(r.Int("P1")), uint32(r.Int("S0")),
			uint32(r.Int("KSP")), uint32(r.Int("ESP")), uint32(r.Int("SSP")), uint32(r.Int("ISP")),
			uint32(r.Int("STRINGPOOL")),
		)
	})

	g.Bind("CLEAR_SYM_ALL", func(id int64, r *dcl.Result) error { return d.Console.ClearSymbol("", true) })
	g.Bind("CLEAR_SYMBOLS", func(id int64, r *dcl.Result) error { return d.Console.ClearSymbol(r.String("P1"), false) })
	g.Bind("CLEAR_BREAK_ALL", func(id int64, r *dcl.Result) error { return d.Console.ClearBreakpoint(0, true) })
	g.Bind("CLEAR_BREAKPOINT", func(id int64, r *dcl.Result) error {
		addr, _, err := d.Console.Evaluator().Eval(r.String("BREAK_ADDR"))
		if err != nil {
			return err
		}

		return d.Console.ClearBreakpoint(addr, false)
	})
	g.Bind("CLEAR_BREAK_INSTR_ALL", func(id int64, r *dcl.Result) error {
		return d.Console.ClearAllInstructionBreakpoints()
	})
	g.Bind("CLEAR_BREAK_INSTR", func(id int64, r *dcl.Result) error {
		return d.Console.RemoveInstructionBreakpoint(r.String("P1"))
	})
	g.Bind("CLEAR_BREAK_FAULT_ALL", func(id int64, r *dcl.Result) error {
		return d.Console.ClearAllFaultBreakpoints()
	})
	g.Bind("CLEAR_BREAK_FAULT", func(id int64, r *dcl.Result) error {
		return d.Console.RemoveFaultBreakpoint(r.String("P1"))
	})

	g.Bind("CLEAR_SYM_TEMP", func(id int64, r *dcl.Result) error { return d.Console.ClearSymbolTemporary() })
	g.Bind("CLEAR_STRINGS", func(id int64, r *dcl.Result) error { return d.Console.ClearString() })
	g.Bind("CLEAR_TB", func(id int64, r *dcl.Result) error { return d.Console.ClearTB() })
	g.Bind("CLEAR_MEMORY", func(id int64, r *dcl.Result) error { return d.Console.ClearMemory() })
	g.Bind("CLEAR_MEM_STAT", func(id int64, r *dcl.Result) error { return d.Console.ClearMemoryStatistics() })

	g.Bind("CLEAR_INTERRUPT", func(id int64, r *dcl.Result) error {
		code, err := parseHexOrEmpty(r.String("INTERRUPT_ID"))
		if err != nil {
			return err
		}

		return d.Console.ClearInterrupt(code)
	})
	g.Bind("CLEAR_INTERRUPT_ALL", func(id int64, r *dcl.Result) error { return d.Console.ClearAllInterrupts() })

	g.Bind("SHOW_REG", func(id int64, r *dcl.Result) error { return d.Console.ShowRegisters() })
	g.Bind("SHOW_PSL", func(id int64, r *dcl.Result) error { return d.Console.ShowPSL() })
	g.Bind("SHOW_MEMORY", func(id int64, r *dcl.Result) error { return d.Console.ShowMemory() })
	g.Bind("SHOW_SYM", func(id int64, r *dcl.Result) error { return d.Console.ShowSymbol(r.String("SYMBOL")) })
	g.Bind("SHOW_SYM_ALL", func(id int64, r *dcl.Result) error { return d.Console.ShowSymbols() })
	g.Bind("SHOW_SYM_SYS", func(id int64, r *dcl.Result) error { return d.Console.ShowSymbolsSystem() })
	g.Bind("SHOW_BREAK", func(id int64, r *dcl.Result) error { return d.Console.ShowBreakpoints() })
	g.Bind("SHOW_BREAK_INSTR", func(id int64, r *dcl.Result) error { return d.Console.ShowInstructionBreakpoints() })
	g.Bind("SHOW_RADIX", func(id int64, r *dcl.Result) error { return d.Console.ShowRadix() })
	g.Bind("SHOW_BASE", func(id int64, r *dcl.Result) error { return d.Console.ShowBase() })
	g.Bind("SHOW_CPU", func(id int64, r *dcl.Result) error { return d.Console.ShowCPU() })
	// SHOW VERSION has no bind of its own: its grammar syntax carries
	// /entry=exe$about (evax.dcl's own "syntax show_version/entry=exe$about"
	// — it shares ABOUT's real VAX routine), so Dispatch's EntryPoint check
	// above reaches it before Grammar.Dispatch ever would. The Go-native
	// ShowVersion stand-in this bind used to call (docs/PHASE-08.md's own
	// "rather than leaving ABOUT/SHOW VERSION with no output at all") is
	// retired now that the real /entry= redirect works.

	g.Bind("SHOW_STACK", func(id int64, r *dcl.Result) error {
		count, err := parseHexOrEmpty(r.String("COUNT"))
		if err != nil {
			return err
		}

		return d.Console.ShowStack(StackKSP, true, count, r.Present("ALL"))
	})
	g.Bind("SHOW_KSP", func(id int64, r *dcl.Result) error {
		count, err := parseHexOrEmpty(r.String("COUNT"))
		if err != nil {
			return err
		}

		return d.Console.ShowStack(StackKSP, false, count, r.Present("ALL"))
	})
	g.Bind("SHOW_ESP", func(id int64, r *dcl.Result) error {
		count, err := parseHexOrEmpty(r.String("COUNT"))
		if err != nil {
			return err
		}

		return d.Console.ShowStack(StackESP, false, count, r.Present("ALL"))
	})
	g.Bind("SHOW_SSP", func(id int64, r *dcl.Result) error {
		count, err := parseHexOrEmpty(r.String("COUNT"))
		if err != nil {
			return err
		}

		return d.Console.ShowStack(StackSSP, false, count, r.Present("ALL"))
	})
	g.Bind("SHOW_ISP", func(id int64, r *dcl.Result) error {
		count, err := parseHexOrEmpty(r.String("COUNT"))
		if err != nil {
			return err
		}

		return d.Console.ShowStack(StackISP, false, count, r.Present("ALL"))
	})
	g.Bind("SHOW_USP", func(id int64, r *dcl.Result) error {
		count, err := parseHexOrEmpty(r.String("COUNT"))
		if err != nil {
			return err
		}

		return d.Console.ShowStack(StackUSP, false, count, r.Present("ALL"))
	})

	g.Bind("SHOW_NVRAM", func(id int64, r *dcl.Result) error { return d.Console.ShowNVRAM() })
	g.Bind("SHOW_ROM", func(id int64, r *dcl.Result) error { return d.Console.ShowROM() })
	g.Bind("SHOW_MODE", func(id int64, r *dcl.Result) error { return d.Console.ShowMode() })
	g.Bind("SHOW_SHIM", func(id int64, r *dcl.Result) error { return d.Console.ShowShim() })
	g.Bind("SHOW_STRING", func(id int64, r *dcl.Result) error { return d.Console.ShowString() })

	g.Bind("SHOW_PAGE", func(id int64, r *dcl.Result) error {
		return d.Console.ShowPage(r.String("ADDRESS"), r.Present("WRITE"))
	})

	g.Bind("SHOW_SCB", func(id int64, r *dcl.Result) error {
		if r.Present("ALL") {
			return d.Console.ShowSCBAll()
		}

		return d.Console.ShowSCB()
	})

	g.Bind("SHOW_CALL_FRAMES", func(id int64, r *dcl.Result) error {
		return d.Console.ShowCallFrames(r.String("COUNT"))
	})

	g.Bind("SHOW_REGIONS", func(id int64, r *dcl.Result) error { return d.Console.ShowRegions() })
	g.Bind("SHOW_SHARE", func(id int64, r *dcl.Result) error { return d.Console.ShowSharePrefix() })
	g.Bind("SHOW_IMAGES", func(id int64, r *dcl.Result) error { return d.Console.ShowImages(r.Present("FULL")) })

	// Phase 23 (docs/PHASE-23.md, subtask 3): SHOW DEFAULT displays the
	// operator's current default device/directory (internal/console/
	// default.go's Console.ShowDefault, backed by internal/rms.Session).
	// Unlike SET DEFAULT (cmdSet's own "DEFAULT" case above), this half is
	// a real DCL grammar entry rather than a fixed-table sub-verb, since
	// SHOW already has a large family of "syntax show_*" entries
	// (show_map/show_tb/... above) that DEFAULT slots into the same way.
	g.Bind("SHOW_DEFAULT", func(id int64, r *dcl.Result) error { return d.Console.ShowDefault() })

	g.Bind("SHOW_QUANTUM", func(id int64, r *dcl.Result) error { return d.Console.ShowQuantum() })
	g.Bind("SHOW_CLOCK", func(id int64, r *dcl.Result) error { return d.Console.ShowClock() })
	g.Bind("SHOW_FAULT", func(id int64, r *dcl.Result) error { return d.Console.ShowFault() })

	g.Bind("SHOW_MAP", func(id int64, r *dcl.Result) error { return d.Console.ShowMap() })
	g.Bind("SHOW_TB", func(id int64, r *dcl.Result) error { return d.Console.ShowTB() })
	g.Bind("SHOW_DEBUG", func(id int64, r *dcl.Result) error { return d.Console.ShowDebug() })
	g.Bind("SHOW_TRACE", func(id int64, r *dcl.Result) error { return d.Console.ShowTrace() })
	g.Bind("SHOW_STEP", func(id int64, r *dcl.Result) error { return d.Console.ShowStepMode() })

	g.Bind("SHOW_INSTRUCTIONS", func(id int64, r *dcl.Result) error {
		return d.Console.ShowInstructions(
			r.Present("MODES"), r.Present("PROFILE"), r.Present("UNIMPLEMENTED"), r.Present("ALL"),
			r.String("OPCODE"),
		)
	})

	// The bare SHOW verb is reached for every show_types keyword with no
	// /syntax= redirect of its own — the plain register/privileged-
	// register name shortcuts (SHOW R0, SHOW PC, SHOW P0BR, ...).
	g.Bind("SHOW", func(id int64, r *dcl.Result) error {
		return d.Console.ShowRegisterOrPrivReg(r.Keyword("SHOW_TYPE"))
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

	g.Bind("SHOW_LOGICAL", func(id int64, r *dcl.Result) error {
		return d.Console.ShowLogicals(r.String("TABLE"), r.String("NAME"))
	})
	g.Bind("DEFINE_LOGICAL", func(id int64, r *dcl.Result) error {
		table := r.String("TABLE")
		if table == "" {
			table = "LNM_PROCESS" // define_logical.c's own default
		}

		return d.Console.DefineLogical(table, r.String("NAME"), r.String("VALUE"))
	})

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

		return d.Console.Mount(r.String("DEVICE"), r.String("FILE"), write)
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
		return d.Console.InitializeContainer(r.String("PATH"), uint32(r.Int("SIZE")), r.String("LABEL"), uint16(r.Int("CLUSTER")))
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
}

type fixedHandler func(d *Dispatcher, rest string) error

// fixedCommands is the Go equivalent of console_dispatch_table's real
// (non -1) entries — see this file's own top comment for the two
// deliberate deviations (DEPOSIT, RUN/R).
//
// Populated in init() rather than as a plain var initializer: cmdTime
// passes the Dispatcher.Dispatch method (to run a sub-command and time
// it), and Dispatch itself reads fixedCommands — a plain var initializer
// referencing cmdTime would make the compiler see that as an
// initialization cycle (fixedCommands -> cmdTime -> Dispatch ->
// fixedCommands), even though nothing is actually invoked until well after
// package initialization.
var fixedCommands map[string]fixedHandler

func init() {
	fixedCommands = map[string]fixedHandler{
		"ZERO": cmdZero,

		"EXAM": cmdExamine, "EX": cmdExamine, "DUMP": cmdExamine,
		"DEP": cmdDeposit, "D": cmdDeposit,

		"STEP": cmdStep, "ST": cmdStep, "S": cmdStep,

		"EXEC": cmdExecute, "GO": cmdExecute, "G": cmdExecute,
		"RUN": cmdRun, "R": cmdRun,

		"SAVE": cmdSave,
		"LOAD": cmdLoad,

		"TIME": cmdTime,
		"PRIN": cmdPrint, "ECHO": cmdPrint,
		"HELP": cmdHelp, "?": cmdHelp,
		"INCL": cmdInclude, "INC": cmdInclude, "@": cmdInclude,

		"SET": cmdSet,

		"ASM": cmdAssemble, "ASSE": cmdAssemble,
		"DISA": cmdDisassemble, "DIS": cmdDisassemble,
		"CALL": cmdCall,
		"IF":   cmdIf,
		"BOOT": cmdNotImplemented("BOOT", "device/RTL support"),
		"ROM":  cmdNotImplemented("ROM", "device support"),
	}
}

func cmdNotImplemented(name, dependency string) fixedHandler {
	return func(d *Dispatcher, rest string) error {
		return vmserrors.New(vmserrors.CLI_NEEDDEP, name, dependency)
	}
}

// cmdAssemble implements ASM: the batch "ASM <filename>" form
// (Console.Assemble) when a name is given, or AssembleBegin's interactive
// REPL mode (docs/PHASE-19.md) for a bare "ASM".
func cmdAssemble(d *Dispatcher, rest string) error {
	path := strings.Trim(strings.TrimSpace(rest), `"`)
	if path == "" {
		return d.Console.AssembleBegin()
	}

	entryAddr, hasEntry, err := d.Console.Assemble(path)
	if err != nil {
		return err
	}

	if hasEntry {
		// console.c's own post-command hook: a .END-named entry address
		// auto-invokes "CALL __ENTRY" (no arguments) once the file
		// finishes assembling.
		return d.Console.Call(entryAddr, false)
	}

	return nil
}

// assembleInteractiveLine hands one line to Console.AssembleInteractiveLine
// while InAssemblerMode is true, matching cmdAssemble's own "hasEntry ->
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

func cmdZero(d *Dispatcher, rest string) error { return d.Console.Zero() }

// cmdStep implements STEP [/OVER|/INTO|/IN|/INSTRUCTION|/RETURN] [address]
// (console_step.c): an optional leading qualifier (defaulting to
// Console.StepMode — see docs/PHASE-18.md), then an optional starting
// address expression exactly like EXEC/GO's own.
func cmdStep(d *Dispatcher, rest string) error {
	mode, rest := parseStepQualifier(rest, d.Console.StepMode)

	rest = strings.TrimSpace(rest)
	if rest == "" {
		return d.Console.Step(nil, mode)
	}

	v, _, err := d.Console.Evaluator().Eval(rest)
	if err != nil {
		return err
	}

	return d.Console.Step(&v, mode)
}

func cmdExecute(d *Dispatcher, rest string) error {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return d.Console.Execute(nil)
	}

	v, _, err := d.Console.Evaluator().Eval(rest)
	if err != nil {
		return err
	}

	return d.Console.Execute(&v)
}

// parseRunQualifier reads RUN's optional single leading qualifier, matching
// console_run's own read_verb-and-CHAR4-compare parsing: at most one of
// /NOINIT, /INIT, /BREAK|/DEBUG|/STEP, or /NOEXECUTE, matched by its first
// four characters (an unrecognized "/word" is left in place for the
// filename parse to reject, exactly as console_run puts back its saved
// pointer on a mismatch). defaultRunInits (Console.DefaultRunInits) seeds
// RunInits before either /INIT or /NOINIT can override it, matching
// console_run.c's own `run_inits = vax.debug & DBG_LIBINIT` default.
func parseRunQualifier(rest string, defaultRunInits bool) (RunOptions, string) {
	opts := RunOptions{RunInits: defaultRunInits}

	rest = strings.TrimLeft(rest, " \t")
	if !strings.HasPrefix(rest, "/") {
		return opts, rest
	}

	i := 1
	for i < len(rest) && rest[i] != ' ' && rest[i] != '\t' {
		i++
	}

	word := strings.ToUpper(rest[1:i])

	word4 := word
	if len(word4) > 4 {
		word4 = word4[:4]
	}

	tail := rest[i:]

	switch word4 {
	case "NOIN":
		opts.RunInits = false

		return opts, tail

	case "INIT":
		opts.RunInits = true

		return opts, tail

	case "BREA", "DEBU", "STEP": //nolint:goconst
		opts.Step = true

		return opts, tail

	case "NOEX":
		opts.NoExecute = true

		return opts, tail

	default:
		return opts, rest // unrecognized qualifier; leave it for the filename parse
	}
}

func cmdRun(d *Dispatcher, rest string) error {
	opts, rest := parseRunQualifier(rest, d.Console.DefaultRunInits())

	fn := strings.Trim(strings.TrimSpace(rest), `"`)
	if fn == "" {
		return vmserrors.New(vmserrors.CLI_NOFILE)
	}

	return d.Console.Run(fn, opts)
}

// parseCallQualifier reads CALL's optional leading /STEP|/BREAK|/DEBUG
// qualifier, matching console_call's own read_verb-and-CHAR4-compare check
// (see docs/PHASE-13.md's design notes on RUN's identical convention).
func parseCallQualifier(rest string) (step bool, tail string) {
	rest = strings.TrimLeft(rest, " \t")
	if !strings.HasPrefix(rest, "/") {
		return false, rest
	}

	i := 1
	for i < len(rest) && rest[i] != ' ' && rest[i] != '\t' {
		i++
	}

	word := strings.ToUpper(rest[1:i])
	if len(word) > 4 {
		word = word[:4]
	}

	switch word {
	case "STEP", "BREA", "DEBU", "DBG":
		return true, rest[i:]

	default:
		return false, rest
	}
}

// cmdCall implements the CALL command (Console.Call, Phase 13's own
// primitive extended in Phase 12 with console_call's argument-list syntax):
// CALL [/STEP] <entry-expr>[(arg1[,arg2...])].
func cmdCall(d *Dispatcher, rest string) error {
	step, rest := parseCallQualifier(rest)
	rest = strings.TrimSpace(rest)

	if rest == "" {
		return vmserrors.New(vmserrors.CLI_NEEDENTRY)
	}

	ev := d.Console.Evaluator()

	addr, remainder, err := ev.Eval(rest)
	if err != nil {
		return err
	}

	var args []uint32

	remainder = strings.TrimSpace(remainder)
	if strings.HasPrefix(remainder, "(") {
		remainder = remainder[1:]

		for {
			remainder = strings.TrimSpace(remainder)
			if strings.HasPrefix(remainder, ")") {
				remainder = remainder[1:]

				break
			}

			if remainder == "" {
				return vmserrors.New(vmserrors.CLI_INCOMPLETEARGS)
			}

			if len(args) > 0 {
				if !strings.HasPrefix(remainder, ",") {
					return vmserrors.New(vmserrors.CLI_NEEDCOMMA)
				}

				remainder = remainder[1:]
			}

			var v uint32

			v, remainder, err = ev.Eval(remainder)
			if err != nil {
				return err
			}

			args = append(args, v)
		}
	}

	return d.Console.Call(addr, step, args...)
}

// cmdIf implements the IF <expression> [THEN] <command> console verb
// (console_if, reference/eVAX/eVAX/Source/Console/console_include.c): if
// expression evaluates nonzero, the rest of the line is dispatched
// recursively as one command (so it can itself be another fixed or DCL
// command) -- vax.init uses "IF DEFINED(\"CONSOLE$ARG_FILE\") THEN SET
// NOVERBOSE" (see expr.go's DEFINED() support, added alongside this).
// Otherwise the rest of the line is simply not executed. Matches
// console_if's own optional "THEN" keyword (present or absent, either is
// accepted) ahead of the conditioned command.
func cmdIf(d *Dispatcher, rest string) error {
	v, rest, err := d.Console.Evaluator().Eval(rest)
	if err != nil {
		return err
	}

	rest = strings.TrimSpace(rest)

	if then, tail := readCommandVerb(rest); strings.EqualFold(then, "THEN") {
		rest = strings.TrimSpace(tail)
	}

	if v == 0 {
		return nil
	}

	return d.Dispatch(rest)
}

func cmdTime(d *Dispatcher, rest string) error {
	return d.Console.Time(strings.TrimSpace(rest), d.Dispatch)
}

func cmdPrint(d *Dispatcher, rest string) error {
	return d.Console.Print(rest)
}

func cmdHelp(d *Dispatcher, rest string) error {
	return d.Console.Help(d.Help, strings.Fields(rest))
}

func cmdInclude(d *Dispatcher, rest string) error {
	path := strings.Trim(strings.TrimSpace(rest), `"`)

	return d.Console.Include(path, d.Dispatch)
}

// parseExamSize reads an optional leading "/BYTE"/"/WORD"/"/LONGWORD"/
// "/ASCII"/"/PTE" format switch (unambiguous-prefix-matched, matching
// EXAMINE/DEPOSIT's shared size vocabulary — see exam.go), defaulting to
// SizeLongword.
func parseExamSize(rest string) (ExamSize, string) {
	rest = strings.TrimLeft(rest, " \t")
	if !strings.HasPrefix(rest, "/") {
		return SizeLongword, rest
	}

	i := 1
	for i < len(rest) && rest[i] != ' ' && rest[i] != '\t' {
		i++
	}

	sw, tail := strings.ToUpper(rest[1:i]), rest[i:]

	switch {
	case sw == "B" || strings.HasPrefix("BYTE", sw):
		return SizeByte, tail

	case sw == "W" || strings.HasPrefix("WORD", sw):
		return SizeWord, tail

	case sw == "L" || strings.HasPrefix("LONGWORD", sw):
		return SizeLongword, tail

	case sw == "A" || strings.HasPrefix("ASCII", sw):
		return SizeASCII, tail

	case sw == "PTE":
		return SizePTE, tail

	default:
		return SizeLongword, rest // not a recognized size switch; leave it for the caller
	}
}

func cmdExamine(d *Dispatcher, rest string) error {
	sz, rest := parseExamSize(rest)
	rest = strings.TrimSpace(rest)

	if rest == "" {
		return d.Console.Examine("", d.Console.DepositAddr, 1, sz)
	}

	if _, ok := registerNames[strings.ToUpper(rest)]; ok {
		return d.Console.Examine(rest, 0, 1, sz)
	}

	ev := d.Console.Evaluator()

	addr, remainder, err := ev.Eval(rest)
	if err != nil {
		return err
	}

	count := uint32(1)

	if remainder = strings.TrimSpace(remainder); remainder != "" {
		end, _, err := ev.Eval(remainder)
		if err != nil {
			return err
		}

		if end < addr {
			return vmserrors.New(vmserrors.CLI_BADRANGE)
		}

		count = (end-addr)/sizeBytes(sz) + 1
	}

	return d.Console.Examine("", addr, count, sz)
}

func cmdDeposit(d *Dispatcher, rest string) error {
	var targetStr, valueStr string

	sz, rest := parseExamSize(rest)
	rest = strings.TrimSpace(rest)

	if eq := strings.IndexByte(rest, '='); eq >= 0 {
		targetStr, valueStr = strings.TrimSpace(rest[:eq]), strings.TrimSpace(rest[eq+1:])
	} else if fields := strings.Fields(rest); len(fields) >= 2 {
		targetStr, valueStr = fields[0], fields[1]
	} else {
		return vmserrors.New(vmserrors.CLI_NEEDDEPOSIT)
	}

	ev := d.Console.Evaluator()

	val, _, err := ev.Eval(valueStr)
	if err != nil {
		return err
	}

	if _, ok := registerNames[strings.ToUpper(targetStr)]; ok {
		return d.Console.Deposit(targetStr, 0, sz, val)
	}

	addr, _, err := ev.Eval(targetStr)
	if err != nil {
		return err
	}

	return d.Console.Deposit("", addr, sz, val)
}

// cmdDisassemble implements DISASSEMBLE/DISA: an optional [start[ end]]
// address range (each an expression, matching EXAMINE's own convention),
// defaulting start to the current deposit address and end to start (a
// single instruction) — matching console_disasm.c's own argument parsing.
func cmdDisassemble(d *Dispatcher, rest string) error {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return d.Console.Disassemble(d.Console.DepositAddr, d.Console.DepositAddr)
	}

	ev := d.Console.Evaluator()

	start, remainder, err := ev.Eval(rest)
	if err != nil {
		return err
	}

	end := start
	if remainder = strings.TrimSpace(remainder); remainder != "" {
		end, _, err = ev.Eval(remainder)
		if err != nil {
			return err
		}
	}

	return d.Console.Disassemble(start, end)
}

// leadingQualifier reads one leading "/word" token off s (up to the next
// space or '/'), matching console_set.c's own read_verb-based qualifier
// scan for SET's /PERMANENT, /ENTRY, /LABEL qualifiers (console_set.c:
// 137-178) — unlike SET BREAK's qualifier (attached with no space to the
// verb, see the "verb, qualifier, _ := strings.Cut" split below), these are
// their own space-separated tokens ahead of the NAME=value symbol form.
func leadingQualifier(s string) (qual, tail string, ok bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "/") {
		return "", s, false
	}

	i := 1
	for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '/' {
		i++
	}

	return s[1:i], s[i:], true
}

// cmdSet implements SET's own small syntax: SET RADIX n, SET BREAKPOINT
// addr, SET BREAKPOINT/INSTRUCTION mnemonic, SET PSL <field>=<value>[,...],
// SET MODE, SET PTE/PAGE, SET VM/MAPEN, SET BASE, SET VERBOSE/VERIFY/
// NOVERBOSE, SET QUANTUM/UIQUANTUM, or the general SET [/PERMANENT]
// [/ENTRY] [/LABEL] <name>=<value> form (see set.go).
func cmdSet(d *Dispatcher, rest string) error {
	// SET's own /PERMANENT, /ENTRY, /LABEL qualifiers (console_set.c:
	// 131-178) are consumed before anything else -- none of the verb-switch
	// keywords below start with '/', so this loop never collides with them.
	var permQual, entryQual, labelQual bool

	for {
		q, tail, ok := leadingQualifier(rest)
		if !ok {
			rest = tail

			break
		}

		uq := strings.ToUpper(q)

		switch {
		case uq != "" && strings.HasPrefix("PERMANENT", uq):
			permQual = true

		case uq != "" && strings.HasPrefix("ENTRY", uq):
			entryQual = true

		case uq != "" && strings.HasPrefix("LABEL", uq):
			labelQual = true

		default:
			return vmserrors.New(vmserrors.CLI_BADQUALPREFIX, q)
		}

		rest = tail
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return vmserrors.New(vmserrors.CLI_NEEDSETARG)
	}

	// A "/qualifier" is attached directly to the verb (no space), matching
	// console_set.c's own read_verb-based qualifier check for SET BREAK —
	// see console_set.c:716-798.
	verb, qualifier, _ := strings.Cut(fields[0], "/")
	uverb := strings.ToUpper(verb)
	uqual := strings.ToUpper(qualifier)

	// after is everything following the verb token, trimmed -- used by the
	// cases below that need the raw remainder rather than space-split
	// fields (PSL's comma list, PTE's address+field-list, BASE's
	// expression).
	after := strings.TrimSpace(rest[len(fields[0]):])

	switch uverb {
	case "RADIX":
		if len(fields) < 2 {
			return vmserrors.New(vmserrors.CLI_NEEDRADIX)
		}

		n, ok := parseRadixArg(fields[1])
		if !ok {
			return vmserrors.New(vmserrors.CLI_BADRADIXVAL, fields[1])
		}

		return d.Console.SetRadix(n)

	case "BREAKPOINT", "BREAK":
		switch {
		case strings.HasPrefix(uqual, "INS"): // /INSTRUCTION, /INS, ...
			if len(fields) < 2 {
				return vmserrors.New(vmserrors.CLI_NEEDBREAKOPCODE)
			}

			return d.Console.AddInstructionBreakpoint(fields[1])

		case uqual != "" && (strings.HasPrefix("TEMPORARY", uqual) || strings.HasPrefix("TMP", uqual)):
			if len(fields) < 2 {
				return vmserrors.New(vmserrors.CLI_NEEDBREAKADDR)
			}

			addr, _, err := d.Console.Evaluator().Eval(fields[1])
			if err != nil {
				return err
			}

			d.Console.AddTemporaryBreakpoint(addr)

			return nil

		case uqual != "" && strings.HasPrefix("FAULT", uqual):
			if len(fields) < 2 {
				return vmserrors.New(vmserrors.CLI_NEEDBREAKADDR)
			}

			return d.Console.AddFaultBreakpoint(fields[1])

		case qualifier != "":
			return vmserrors.New(vmserrors.CLI_BADQUALIFIER, qualifier)
		}

		if len(fields) < 2 {
			return vmserrors.New(vmserrors.CLI_NEEDBREAKADDR)
		}

		addr, _, err := d.Console.Evaluator().Eval(fields[1])
		if err != nil {
			return err
		}

		d.Console.AddBreakpoint(addr)

		return nil

	case "STEP":
		if len(fields) < 2 {
			return vmserrors.New(vmserrors.CLI_NEEDSTEPMODE)
		}

		return d.Console.SetStepMode(fields[1])

	case "TRACE", "DISASSEMBLY":
		d.Console.SetTrace(true)

		return nil

	case "NOTRACE", "NODISASSEMBLE":
		d.Console.SetTrace(false)

		return nil

	case "DEBUG", "DBG":
		var names []string

		for _, f := range fields[1:] {
			for _, n := range strings.Split(f, ",") {
				if n = strings.TrimSpace(n); n != "" {
					names = append(names, n)
				}
			}
		}

		return d.Console.SetDebug(names)

	case "PSL":
		return cmdSetPSL(d, after)

	case "MODE":
		if len(fields) < 2 {
			return vmserrors.New(vmserrors.CLI_NEEDMODE)
		}

		return d.Console.SetMode(fields[1])

	case "PTE", "PAGE":
		return cmdSetPTE(d, after)

	case "FAULT", "HIST", "HISTORY":
		n, err := parseSetDecimal(fields)
		if err != nil {
			return err
		}

		return d.Console.SetFaultHistory(n)

	case "VM", "MAPEN":
		return d.Console.SetVM(true)

	case "NOVM", "NOMAPEN":
		return d.Console.SetVM(false)

	case "BASE":
		if after == "" {
			return vmserrors.New(vmserrors.CLI_NEEDSETARG)
		}

		addr, _, err := d.Console.Evaluator().Eval(after)
		if err != nil {
			return err
		}

		return d.Console.SetBase(addr)

	case "VERBOSE":
		return d.Console.SetVerbose()

	case "VERIFY":
		return d.Console.SetVerify()

	case "NOVERBOSE":
		return d.Console.SetNoVerbose()

	case "QUANTUM":
		n, err := parseSetDecimal(fields)
		if err != nil {
			return err
		}

		return d.Console.SetQuantum(n)

	case "UIQUANTUM":
		n, err := parseSetDecimal(fields)
		if err != nil {
			return err
		}

		return d.Console.SetUIQuantum(n)

	// DEFAULT (docs/PHASE-23.md, subtask 3) establishes the operator's
	// current default device/directory for DIRECTORY/DELETE/PURGE/COPY/TYPE
	// -- the natural, minimal extension of this same switch, mirroring
	// ods2's own cmdSet adding a "default" case alongside its register/
	// symbol assignment (see internal/rms/session.go's own doc comment).
	// Unlike SET DEFAULT's real VMS counterpart, there's nothing else on
	// this line to validate ahead of time: whatever text follows the verb
	// is handed straight to Session.SetDefault, which does its own file-
	// specification-syntax checking.
	case "DEFAULT":
		if after == "" {
			return vmserrors.New(vmserrors.CLI_NEEDSETARG)
		}

		return d.Console.SetDefault(after)
	}

	eq := strings.IndexByte(rest, '=')
	if eq < 0 {
		return vmserrors.New(vmserrors.CLI_BADSETSYNTAX, rest)
	}

	name := strings.TrimSpace(rest[:eq])

	val, _, err := d.Console.Evaluator().Eval(strings.TrimSpace(rest[eq+1:]))
	if err != nil {
		return err
	}

	return d.Console.SetSymbolQualified(name, val, permQual, entryQual, labelQual)
}

// parseRadixArg accepts either console_set.c's own HEX/HEXA/16/DEC/DECI/10
// keyword forms or a bare number (this port's own pre-existing, more
// lenient numeric form, kept for backward compatibility — SetRadix itself
// rejects anything but 8/10/16 either way).
func parseRadixArg(s string) (int, bool) {
	switch strings.ToUpper(s) {
	case "HEX", "HEXA":
		return 16, true
	case "DEC", "DECI":
		return 10, true
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}

	return n, true
}

// parseSetDecimal parses SET QUANTUM/SET UIQUANTUM's single decimal
// argument, matching console_set.c's own asm_dec parse.
func parseSetDecimal(fields []string) (int, error) {
	if len(fields) < 2 {
		return 0, vmserrors.New(vmserrors.CLI_NEEDSETARG)
	}

	n, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, vmserrors.New(vmserrors.CLI_BADNUMBER, fields[1])
	}

	return n, nil
}

// cmdSetPSL implements SET PSL's own comma-separated "<field>=<value>[,...]"
// clause list, matching console_set.c:256-357's parsing loop.
func cmdSetPSL(d *Dispatcher, clauses string) error {
	if clauses == "" {
		return vmserrors.New(vmserrors.CLI_INVSETPSL, "")
	}

	for _, clause := range strings.Split(clauses, ",") {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}

		eq := strings.IndexByte(clause, '=')
		if eq < 0 {
			return vmserrors.New(vmserrors.CLI_INVSETPSL, clause)
		}

		field := strings.TrimSpace(clause[:eq])

		v, _, err := d.Console.Evaluator().Eval(strings.TrimSpace(clause[eq+1:]))
		if err != nil {
			return err
		}

		if err := d.Console.SetPSLField(field, v); err != nil {
			return err
		}
	}

	return nil
}

// cmdSetPTE implements SET PTE/SET PAGE: "<addr> [TO <addr2>]
// <field>=<value>[,...]", matching console_set.c's setpte_multiple/setpte/
// parse_pte_changes. A TO range applies the same field list to every
// 512-byte-aligned page from addr to addr2 inclusive, matching
// setpte_multiple's own loop.
func cmdSetPTE(d *Dispatcher, rest string) error {
	ev := d.Console.Evaluator()

	addr1, rest, err := ev.Eval(rest)
	if err != nil {
		return err
	}

	rest = strings.TrimSpace(rest)

	addrs := []uint32{addr1}

	if len(rest) >= 3 && strings.EqualFold(rest[:2], "TO") && (rest[2] == ' ' || rest[2] == '\t') {
		var addr2 uint32

		addr2, rest, err = ev.Eval(strings.TrimSpace(rest[2:]))
		if err != nil {
			return err
		}

		rest = strings.TrimSpace(rest)

		a1, a2 := addr1&^0x1FF, addr2&^0x1FF
		if a2 < a1 {
			return vmserrors.New(vmserrors.CLI_BADRANGE)
		}

		addrs = addrs[:0]
		for a := a1; a <= a2; a += 512 {
			addrs = append(addrs, a)
		}
	}

	if rest == "" {
		return vmserrors.New(vmserrors.CLI_BADSETSYNTAX, rest)
	}

	for _, addr := range addrs {
		for _, clause := range strings.Split(rest, ",") {
			clause = strings.TrimSpace(clause)
			if clause == "" {
				continue
			}

			eq := strings.IndexByte(clause, '=')
			if eq < 0 {
				return vmserrors.New(vmserrors.CLI_BADPTEFIELD, clause)
			}

			field := strings.TrimSpace(clause[:eq])

			v, _, err := ev.Eval(strings.TrimSpace(clause[eq+1:]))
			if err != nil {
				return err
			}

			if err := d.Console.SetPTE(addr, field, v); err != nil {
				return err
			}
		}
	}

	return nil
}

// cmdSave/cmdLoad implement SAVE/LOAD's "/ROM <file>" and "/NVRAM <file>"
// forms (rom.go); the plain (no qualifier) SAVE/LOAD .VAX-file form isn't
// implemented — see rom.go's doc comment.
func cmdSave(d *Dispatcher, rest string) error {
	// console_save.c checks `if (!vax_init) return VAX_NOVAX;` before doing
	// anything else -- ROM/NVRAM now live on Engine.Memory(), which doesn't
	// exist until INIT has allocated a machine.
	if err := d.Console.requireInit(); err != nil {
		return err
	}

	kind, file, err := parseRomOrNvramArg(rest)
	if err != nil {
		return err
	}

	if kind == "ROM" { //nolint:goconst
		return d.Console.SaveROM(file)
	}

	return d.Console.SaveNVRAM(file)
}

func cmdLoad(d *Dispatcher, rest string) error {
	// console_load.c checks `if (!vax_init) return VAX_NOVAX;` before doing
	// anything else -- see cmdSave's identical guard above.
	if err := d.Console.requireInit(); err != nil {
		return err
	}

	kind, file, err := parseRomOrNvramArg(rest)
	if err != nil {
		return err
	}

	if kind == "ROM" {
		return d.Console.LoadROM(file)
	}

	return d.Console.LoadNVRAM(file)
}

func parseRomOrNvramArg(rest string) (kind, file string, err error) {
	rest = strings.TrimSpace(rest)

	switch {
	case strings.HasPrefix(strings.ToUpper(rest), "/ROM"):
		kind, rest = "ROM", rest[4:]

	case strings.HasPrefix(strings.ToUpper(rest), "/NVRAM"):
		kind, rest = "NVRAM", rest[6:]

	default:
		return "", "", vmserrors.New(vmserrors.CLI_NEEDROMNVRAM)
	}

	file = strings.Trim(strings.TrimSpace(rest), `"`)
	if file == "" {
		return "", "", vmserrors.New(vmserrors.CLI_NEEDFILENAME, kind)
	}

	return kind, file, nil
}
