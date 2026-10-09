package console

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vax"
)

// evaxGrammarPathForConsole locates internal/bootdata/files/evax.dcl, the
// grammar govax actually parses at runtime (cmd/govax's own resolver, Phase
// 15's embedded-fallback mechanism) -- not testdata/dcl/evax.dcl, a pure,
// untouched `git archive` import from the upstream C repo that has diverged
// from the bootdata copy since Phase 22 added MOUNT/DISMOUNT (see
// docs/PHASE-22.md's "Grammar file" design decision). Dispatch tests exist
// to exercise the same grammar/handler wiring cmd/govax uses, so they track
// the bootdata copy exactly like internal/console/dcl's own grammar-parser
// tests do.
func evaxGrammarPathForConsole(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "bootdata", "files", "console.dcl")
}

func loadEvaxGrammar(t *testing.T) *dcl.Grammar {
	t.Helper()

	g, err := dcl.LoadGrammarFile(evaxGrammarPathForConsole(t))
	if err != nil {
		t.Fatalf("LoadGrammarFile: %v", err)
	}

	return g
}

func newTestDispatcher(t *testing.T) (*Dispatcher, *Console) {
	t.Helper()

	c, _ := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	return d, c
}

func TestDispatch_setAndExamineRegister(t *testing.T) {
	d, c := newTestDispatcher(t)
	if err := d.Dispatch("SET R4=1234"); err != nil {
		t.Fatalf("Dispatch(SET): %v", err)
	}

	if got := c.CPU.GPR(vax.R4); got != 0x1234 {
		t.Errorf("R4 = %#x, want 0x1234 (radix 16 default)", got)
	}
}

// TestDispatch_runActivatesImage checks that RUN (and its /NOEXECUTE
// qualifier) reach Console.Run -- real VMS image activation (Phase 13),
// not plain CPU execution (that's EXEC/GO/G, see TestDispatch_stepAndGo).
func TestDispatch_runActivatesImage(t *testing.T) {
	c := newRunnableConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := d.Dispatch(`RUN/NOEXECUTE "` + exeFixturePath(t, "simple.exe") + `"`); err != nil {
		t.Fatalf("Dispatch(RUN/NOEXECUTE): %v", err)
	}

	if len(c.images().ICBList) == 0 {
		t.Fatal("expected RUN to have loaded at least the main image")
	}

	// simple.exe (per docs/PHASE-13.md's own milestone notes) runs to a
	// clean completion, so a real (non-/NOEXECUTE) RUN can be dispatched
	// end-to-end here too, via the "R" abbreviation.
	if err := d.Dispatch(`R "` + exeFixturePath(t, "simple.exe") + `"`); err != nil {
		t.Fatalf("Dispatch(R): %v", err)
	}
}

func TestDispatch_showViaDCL(t *testing.T) {
	d, _ := newTestDispatcher(t)

	if err := d.Console.ShowRegisters(); err != nil {
		t.Fatalf("Dispatch(SHOW REGISTERS): %v", err)
	}

	if err := d.Console.ShowPSL(); err != nil {
		t.Fatalf("Dispatch(SHOW PSL): %v", err)
	}
}

func TestDispatch_showRegisterShortcut(t *testing.T) {
	d, c := newTestDispatcher(t)
	c.CPU.SetGPR(vax.R2, 0x55)

	if err := d.Console.ShowRegisterOrPrivReg("R2"); err != nil {
		t.Fatalf("Dispatch(SHOW R2): %v", err)
	}
}

func TestDispatch_vminitViaDCL(t *testing.T) {
	d, c := newTestDispatcher(t)
	if err := d.Dispatch("VMINIT/P0=20/P1=20/S0=0/KSP=2/ESP=2/SSP=2/ISP=2"); err != nil {
		t.Fatalf("Dispatch(VMINIT): %v", err)
	}

	if !c.VMInitValid {
		t.Error("expected VMInitValid true after VMINIT")
	}
}

func TestDispatch_exitStopsRunning(t *testing.T) {
	d, c := newTestDispatcher(t)
	if err := d.Dispatch("EXIT"); err != nil {
		t.Fatalf("Dispatch(EXIT): %v", err)
	}

	if c.Running() {
		t.Error("expected Running() false after EXIT")
	}
}

// TestDispatch_entryPointCommandUndefinedWithoutMicrokernel checks that a
// DCL /entry= command (ABOUT, XTEST, SHOW VERSION — see
// docs/PHASE-16.md sub-phase 4) reports the entry symbol as undefined
// rather than as a categorically-unimplemented mechanism, once no
// microkernel has been booted to define it — the same "Undefined symbol"
// error CALL EXE$ABOUT would report directly.
func TestDispatch_entryPointCommandUndefinedWithoutMicrokernel(t *testing.T) {
	d, _ := newTestDispatcher(t)

	err := d.Dispatch("ABOUT")
	if err == nil || !strings.Contains(err.Error(), "Undefined symbol") || !strings.Contains(err.Error(), "EXE$ABOUT") {
		t.Errorf("Dispatch(ABOUT) = %v, want an undefined-symbol error naming EXE$ABOUT", err)
	}
}

// TestDispatch_entryPointCommandCallsRealRoutine boots kernel.asm (which
// defines EXE$ABOUT as a real .ENTRY, see kernel.asm's own "ABOUT console
// command" section) and checks that both ABOUT and SHOW VERSION — the two
// DCL surfaces that redirect to it via /entry=exe$about — actually resolve
// the symbol and CALL it, reaching kernel.asm's own LIB$PUT_OUTPUT/TXCS
// character-output loop (confirmed by the banner's first character
// appearing in Console.Out) rather than failing outright. It can't check
// for the full banner: LIB$PUT_OUTPUT writes one character at a time via
// TXCS/TXDB and busy-waits on TXCS's ready bit between them, which this
// port's TXCS emulation never asserts again after the first write (no
// interrupt-delivery modeling for it — the same documented limitation
// regression_test.go's own TestRegression_rtlDependentAsmFixtures tracks
// for other TXCS/RXCS-polling fixtures), so the run always ends by hitting
// the instruction-limit guard rather than completing — a benign stop
// (reportStopReason), not a Dispatch error.
func TestDispatch_entryPointCommandCallsRealRoutine(t *testing.T) {
	for _, cmd := range []string{"ABOUT", "SHOW VERSION"} {
		t.Run(cmd, func(t *testing.T) {
			// Inlines newRunnableConsole's own body (image_test.go) rather
			// than calling it directly: RTL's console-output writer is
			// captured by value at Init time (corevms.NewEnvironment's
			// consoleOut), so the buffer this test inspects has to be in
			// place before Init runs, not swapped in afterward.
			var buf strings.Builder

			c := New(&buf)
			if err := c.Init(4096 * 512); err != nil {
				t.Fatalf("Init: %v", err)
			}

			// p1Pages 200, not newRunnableConsole's own 100 -- see that
			// helper's doc comment on why kernel.asm's ".p1vector" needs it.
			if err := c.VMInit(2000, 200, 0, 4, 4, 4, 4, 8); err != nil {
				t.Fatalf("VMInit: %v", err)
			}

			g := loadEvaxGrammar(t)
			d := NewDispatcher(c, g, nil)

			if _, _, err := c.Assemble(kernelPath(t)); err != nil {
				t.Fatalf("Assemble(kernel.asm): %v", err)
			}

			c.Engine.SetLimits(100_000, 0)

			if err := d.Dispatch(cmd); err != nil {
				t.Fatalf("Dispatch(%s): %v", cmd, err)
			}

			if out := buf.String(); !strings.HasPrefix(out, "g") {
				t.Errorf("Dispatch(%s) output = %q, want it to start with EXE$ABOUT's banner ('govax Console Microkernel...')", cmd, out)
			}
		})
	}
}

func TestDispatch_unboundShowSubformErrors(t *testing.T) {
	d, _ := newTestDispatcher(t)
	// SHOW ASSEMBLER_FLAGS remains unbound -- it needs SET ASSEMBLER's flag
	// set, not yet implemented (docs/PHASE-16.md sub-phase 1c). SHOW DEBUG,
	// previously unbound here too, is now implemented -- see
	// docs/PHASE-17.md and TestDispatch_showDebug.
	err := d.Dispatch("SHOW ASSEMBLER_FLAGS")
	if err == nil {
		t.Error("expected an error for the unimplemented SHOW ASSEMBLER_FLAGS")
	}
}

func TestDispatch_debugDCLTrace(t *testing.T) {
	c, buf := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)
	c.CPU.SetDebug(vax.DebugDCL)

	if err := d.Dispatch("SHOW RADIX"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), `DEBUG(DCL): parsing "SHOW RADIX"`) {
		t.Errorf("output = %q, want a DEBUG(DCL) parse trace", buf.String())
	}
}

func TestDispatch_setTraceAndShowTrace(t *testing.T) {
	d, c := newTestDispatcher(t)

	c.SetTrace(true)

	if !c.Trace {
		t.Error("Trace = false, want true after SET TRACE")
	}

	if err := d.Console.ShowTrace(); err != nil {
		t.Fatalf("Dispatch(SHOW TRACE): %v", err)
	}

	c.SetTrace(false)

	if c.Trace {
		t.Error("Trace = true, want false after SET NOTRACE")
	}
}

func TestDispatch_setDebugAndShowDebug(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch("SET DEBUG VM,NOUSERHALT"); err != nil {
		t.Fatalf("Dispatch(SET DEBUG): %v", err)
	}

	if !c.CPU.DebugEnabled(vax.DebugVM) || c.CPU.DebugEnabled(vax.DebugUserHalt) {
		t.Errorf("Debug() = %#x, want VM set and USERHALT cleared", c.CPU.Debug())
	}

	if err := d.Dispatch("SHOW DEBUG"); err != nil {
		t.Fatalf("Dispatch(SHOW DEBUG): %v", err)
	}

	if err := d.Dispatch("SET DEBUG BOGUS"); err == nil {
		t.Error("expected an error for an invalid SET DEBUG flag")
	}
}

func TestDispatch_setVerbose(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch("SET NOVERBOSE"); err != nil {
		t.Fatalf("Dispatch(SET NOVERBOSE): %v", err)
	}

	if c.Verbose {
		t.Error("expected Verbose false after SET NOVERBOSE")
	}

	if err := d.Dispatch("SET VERIFY"); err != nil {
		t.Fatalf("Dispatch(SET VERIFY): %v", err)
	}

	if !c.Verify {
		t.Error("expected Verify true after SET VERIFY")
	}
}

func TestDispatch_setRadixKeywords(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch("SET RADIX DEC"); err != nil {
		t.Fatalf("Dispatch(SET RADIX DEC): %v", err)
	}

	if c.Radix != 10 {
		t.Errorf("Radix = %d, want 10", c.Radix)
	}

	if err := d.Dispatch("SET RADIX HEX"); err != nil {
		t.Fatalf("Dispatch(SET RADIX HEX): %v", err)
	}

	if c.Radix != 16 {
		t.Errorf("Radix = %d, want 16", c.Radix)
	}

	// The pre-existing bare-numeric form must keep working too.
	if err := d.Dispatch("SET RADIX 8"); err != nil {
		t.Fatalf("Dispatch(SET RADIX 8): %v", err)
	}

	if c.Radix != 8 {
		t.Errorf("Radix = %d, want 8", c.Radix)
	}
}

// TestDispatch_setSymbolQualifiers checks SET's own /PERMANENT and /ENTRY
// qualifier scan ahead of the general NAME=value symbol form.
func TestDispatch_setSymbolQualifiers(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch("SET /PERMANENT PERMSYM=100"); err != nil {
		t.Fatalf("Dispatch(SET /PERMANENT): %v", err)
	}

	sym, ok := c.Symbols.Find("PERMSYM")
	if !ok || !sym.IsPermanent() {
		t.Errorf("PERMSYM = %+v, ok=%v; want a permanent symbol", sym, ok)
	}

	if err := d.Dispatch("SET/ENTRY ENTRYSYM=200"); err != nil {
		t.Fatalf("Dispatch(SET/ENTRY): %v", err)
	}

	sym, ok = c.Symbols.Find("ENTRYSYM")
	if !ok || !sym.IsEntry() {
		t.Errorf("ENTRYSYM = %+v, ok=%v; want an entry symbol", sym, ok)
	}
}

func TestDispatch_clearStringsAndInterrupt(t *testing.T) {
	d, c := newTestDispatcher(t)

	c.Symbols.Set("CONSOLE$STRINGPOOL_BASE", 0x2000, SymbolSystem)
	c.Symbols.Set("CONSOLE$STRINGPOOL_SIZE", 8, SymbolSystem)
	c.Symbols.Set("CONSOLE$STRINGPOOL", 0x2008, SymbolSystem)

	if err := d.Dispatch("CLEAR STRINGS"); err != nil {
		t.Fatalf("Dispatch(CLEAR STRINGS): %v", err)
	}

	if got, _ := c.Symbols.Get("CONSOLE$STRINGPOOL"); got != 0x2000 {
		t.Errorf("CONSOLE$STRINGPOOL = %#x, want reset to 0x2000", got)
	}

	psl := c.CPU.PSL()
	psl.SetIPL(20)
	c.CPU.SetPSL(psl)
	c.Engine.SetQuantum(4)
	c.Engine.Interrupt(0x24, 20, 0)

	if err := c.ClearAllInterrupts(); err != nil {
		t.Fatalf("ClearAllInterrupts: %v", err)
	}

	if _, queued := c.Engine.PendingInterrupts(); len(queued) != 0 {
		t.Errorf("queued = %+v, want empty after CLEAR INTERRUPT/ALL", queued)
	}
}

// TestDispatch_notImplementedCommand checks BOOT, still gated behind
// notImplemented (commands.go; device/RTL support) -- ASM used to be this test's own
// example (its bare, no-filename form returned CLI_NOASMREPL) until
// docs/PHASE-19.md implemented interactive assembler mode; see
// TestDispatchASM_bareEntersInteractiveMode (asm_repl_test.go) for its
// current behavior.
func TestDispatch_notImplementedCommand(t *testing.T) {
	d, _ := newTestDispatcher(t)
	if err := d.Dispatch("BOOT"); err == nil {
		t.Error("expected an error for BOOT")
	}
}

func TestDispatch_saveLoadROMRequiresQualifier(t *testing.T) {
	d, _ := newTestDispatcher(t)
	if err := d.Dispatch("SAVE foo.vax"); err == nil {
		t.Error("expected an error for plain SAVE without /ROM or /NVRAM")
	}
}

func TestDispatch_help(t *testing.T) {
	c, _ := newTestConsole(t)
	g := loadEvaxGrammar(t)
	h := ParseHelp("$HELP\nTop-level help.\n")

	d := NewDispatcher(c, g, h)
	if err := d.Dispatch("HELP"); err != nil {
		t.Fatalf("Dispatch(HELP): %v", err)
	}
}

// TestDispatch_if exercises the IF <expr> [THEN] <command> console verb
// (commands.go's ifCommand) against the same pattern vax.init uses (IF DEFINED("...") THEN
// SET ...): the conditioned command runs only when the expression is
// nonzero, and THEN is optional either way.
func TestDispatch_if(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch(`IF DEFINED("CONSOLE$ARG_FILE") THEN SET R0=1`); err != nil {
		t.Fatalf("Dispatch(IF, false): %v", err)
	}

	if got := c.CPU.GPR(vax.R0); got != 0 {
		t.Errorf("R0 = %#x, want 0 (condition should be false)", got)
	}

	if err := d.Dispatch(`IF 1 THEN SET R0=1`); err != nil {
		t.Fatalf("Dispatch(IF, true, THEN): %v", err)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %#x, want 1", got)
	}

	if err := d.Dispatch(`IF 1=1 SET R0=2`); err != nil {
		t.Fatalf("Dispatch(IF, true, no THEN): %v", err)
	}

	if got := c.CPU.GPR(vax.R0); got != 2 {
		t.Errorf("R0 = %#x, want 2", got)
	}
}

// TestDispatch_initializeVax exercises Phase 23 subtask 1's unified
// INITIALIZE verb end to end through the real Dispatcher: /VAX carries
// forward INIT's exact pre-Phase-23 behavior (Console.Init, page count *
// 512 bytes), now reached via the DCL grammar instead of dispatch.go's old
// fixedCommands entry.
func TestDispatch_initializeVax(t *testing.T) {
	c, _ := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	if err := d.Dispatch("INITIALIZE/VAX ^d20"); err != nil {
		t.Fatalf("Dispatch(INITIALIZE/VAX): %v", err)
	}

	if c.Mem == nil {
		t.Fatal("expected memory allocated after INITIALIZE/VAX")
	}

	if got := c.Mem.Size(); got != 20*512 {
		t.Errorf("Mem.Size() = %d, want %d (20 pages)", got, 20*512)
	}
}

// TestDispatch_initAbbreviatesInitializeVax checks that the old "INIT"
// spelling still works end to end -- purely as DCL's own unambiguous-
// prefix verb matching now that INITIALIZE is a grammar verb, not a second
// fixedCommands entry (removed by this subtask; see dispatch.go's own
// bindGrammar comment on INITIALIZE_VAX).
func TestDispatch_initAbbreviatesInitializeVax(t *testing.T) {
	c, _ := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	if err := d.Dispatch("INIT/VAX ^d20"); err != nil {
		t.Fatalf("Dispatch(INIT/VAX): %v", err)
	}

	if got := c.Mem.Size(); got != 20*512 {
		t.Errorf("Mem.Size() = %d, want %d (20 pages)", got, 20*512)
	}
}

// TestDispatch_initializeVaxNeedsPages checks that INITIALIZE/VAX with no
// page count reports the same CLI_NEEDPAGES wording cmdInit always has,
// even though PAGES carries no formal /prompt= in the grammar (see
// evax.dcl's own comment on initialize_vax).
func TestDispatch_initializeVaxNeedsPages(t *testing.T) {
	d, _ := newTestDispatcher(t)

	err := d.Dispatch("INITIALIZE/VAX")
	if err == nil || !strings.Contains(err.Error(), "NEEDPAGES") {
		t.Errorf("Dispatch(INITIALIZE/VAX) = %v, want a NEEDPAGES error", err)
	}
}

// TestDispatch_initializeBareErrors checks that a bare INITIALIZE (no /VAX
// or /CONTAINER) reaches Grammar.Dispatch's own "no handler bound" error --
// per docs/PHASE-23.md, neither qualifier is a default, so there is no
// meaningful do-nothing form of this verb.
func TestDispatch_initializeBareErrors(t *testing.T) {
	d, _ := newTestDispatcher(t)

	if err := d.Dispatch("INITIALIZE"); err == nil {
		t.Error("expected an error for a bare INITIALIZE with no /VAX or /CONTAINER")
	}
}

func TestDispatch_emptyLineIsNoop(t *testing.T) {
	d, _ := newTestDispatcher(t)
	if err := d.Dispatch("   "); err != nil {
		t.Fatalf("Dispatch(blank): %v", err)
	}

	if err := d.Dispatch("! a comment"); err != nil {
		t.Fatalf("Dispatch(comment): %v", err)
	}
}
