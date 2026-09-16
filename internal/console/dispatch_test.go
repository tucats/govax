package console

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vax"
)

func evaxGrammarPathForConsole(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "dcl", "evax.dcl")
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

func TestDispatch_fixedTableExamine(t *testing.T) {
	d, _ := newTestDispatcher(t)
	if err := d.Console.Deposit("", 0x1000, SizeLongword, 0x99887766); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	if err := d.Dispatch("EXAMINE 1000"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
}

func TestDispatch_fourCharTruncation(t *testing.T) {
	d, _ := newTestDispatcher(t)
	// "EXAMINE" truncates to "EXAM" for fixed-table matching, same as
	// typing "EXAM" itself.
	if err := d.Dispatch("EXAMINE R0"); err != nil {
		t.Fatalf("Dispatch(EXAMINE R0): %v", err)
	}
	if err := d.Dispatch("EXAM R0"); err != nil {
		t.Fatalf("Dispatch(EXAM R0): %v", err)
	}
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

func TestDispatch_depositAndExamine(t *testing.T) {
	d, c := newTestDispatcher(t)
	// A hex literal starting with a letter (A-F) needs a leading digit or
	// a "^X"/"0X" radix prefix — matching real VAX DCL/MACRO number syntax
	// (asm_hex/asm_expr3 treat a leading letter as the start of a symbol
	// name, not a digit) — see internal/console/expr.go.
	if err := d.Dispatch("D 2000=0ABCD123"); err != nil {
		t.Fatalf("Dispatch(D): %v", err)
	}
	v, err := c.Mem.LoadLongword(c.CPU, 0x2000)
	if err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}
	if v != 0x0ABCD123 {
		t.Errorf("got %#x, want 0x0abcd123", v)
	}
}

func TestDispatch_stepAndGo(t *testing.T) {
	d, c := newTestDispatcher(t)
	loadProgram(t, c, 0x200, opNop, opNop, opHalt)
	if err := d.Dispatch("STEP 200"); err != nil {
		t.Fatalf("Dispatch(STEP): %v", err)
	}
	if c.CPU.GPR(vax.PC) != 0x201 {
		t.Errorf("PC after STEP = %#x, want 0x201", c.CPU.GPR(vax.PC))
	}
	if err := d.Dispatch("GO"); err != nil {
		t.Fatalf("Dispatch(GO): %v", err)
	}
	if c.CPU.GPR(vax.PC) != 0x203 {
		t.Errorf("PC after GO = %#x, want 0x203", c.CPU.GPR(vax.PC))
	}
}

// TestDispatch_runActivatesImage checks that RUN (and its /NOEXECUTE
// qualifier) reach Console.Run -- real VMS image activation (Phase 13),
// not plain CPU execution (that's EXEC/GO/G, see TestDispatch_stepAndGo).
func TestDispatch_runActivatesImage(t *testing.T) {
	c := newRunnableConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	if err := d.Dispatch("RUN/NOEXECUTE " + exeFixturePath(t, "simple.exe")); err != nil {
		t.Fatalf("Dispatch(RUN/NOEXECUTE): %v", err)
	}
	if len(c.ICBList) == 0 {
		t.Fatal("expected RUN to have loaded at least the main image")
	}

	// simple.exe (per docs/PHASE-13.md's own milestone notes) runs to a
	// clean completion, so a real (non-/NOEXECUTE) RUN can be dispatched
	// end-to-end here too, via the "R" abbreviation.
	if err := d.Dispatch("R " + exeFixturePath(t, "simple.exe")); err != nil {
		t.Fatalf("Dispatch(R): %v", err)
	}
}

// TestDispatch_asmThenCall exercises ASM and CALL together through the
// full Dispatcher (Phase 12): assembling xor.asm merges its "test" label
// into Console.Symbols, so a plain "CALL TEST" (no explicit address, no
// arguments) can find it by name and run it to completion.
func TestDispatch_asmThenCall(t *testing.T) {
	c := newRunnableConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	if err := d.Dispatch("ASM " + asmFixturePath(t, "xor.asm")); err != nil {
		t.Fatalf("Dispatch(ASM): %v", err)
	}
	if err := d.Dispatch("CALL TEST"); err != nil {
		t.Fatalf("Dispatch(CALL): %v", err)
	}

	const want = 0xC8600 ^ 0x10

	if got := c.CPU.GPR(vax.R4); got != want {
		t.Errorf("R4 = %#x, want %#x", got, want)
	}
}

// TestDispatch_callWithArgumentList checks CALL's "(arg1[,arg2...])"
// syntax (console_call's own optional argument list, Phase 12) against a
// small hand-assembled routine that doubles its one argument.
func TestDispatch_callWithArgumentList(t *testing.T) {
	c := newRunnableConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	src := "\t.entry\tdbltest, ^m<>\n\tmovl\t4(ap), r0\n\taddl2\tr0, r0\n\tret\n\t.end\n"
	path := filepath.Join(t.TempDir(), "dbl_test.asm")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := d.Dispatch("ASM " + path); err != nil {
		t.Fatalf("Dispatch(ASM): %v", err)
	}
	if err := d.Dispatch("CALL DBLTEST(^D21)"); err != nil {
		t.Fatalf("Dispatch(CALL): %v", err)
	}
	if got := c.CPU.GPR(vax.R0); got != 42 {
		t.Errorf("R0 = %d, want 42", got)
	}
}

// TestDispatch_callStepQualifier checks CALL/STEP is accepted (parsed and
// dispatched without error) -- console_call's own /STEP|/BREAK|/DEBUG
// qualifier, matching RUN's identical convention (parseRunQualifier).
func TestDispatch_callStepQualifier(t *testing.T) {
	c := newRunnableConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	if err := d.Dispatch("ASM " + asmFixturePath(t, "xor.asm")); err != nil {
		t.Fatalf("Dispatch(ASM): %v", err)
	}
	if err := d.Dispatch("CALL/STEP TEST"); err != nil {
		t.Fatalf("Dispatch(CALL/STEP): %v", err)
	}
}

func TestDispatch_showViaDCL(t *testing.T) {
	d, _ := newTestDispatcher(t)
	if err := d.Dispatch("SHOW REGISTERS"); err != nil {
		t.Fatalf("Dispatch(SHOW REGISTERS): %v", err)
	}
	if err := d.Dispatch("SHOW PSL"); err != nil {
		t.Fatalf("Dispatch(SHOW PSL): %v", err)
	}
}

func TestDispatch_showRegisterShortcut(t *testing.T) {
	d, c := newTestDispatcher(t)
	c.CPU.SetGPR(vax.R2, 0x55)
	if err := d.Dispatch("SHOW R2"); err != nil {
		t.Fatalf("Dispatch(SHOW R2): %v", err)
	}
}

func TestDispatch_clearBreakpoint(t *testing.T) {
	d, c := newTestDispatcher(t)
	c.AddBreakpoint(0x400)
	if err := d.Dispatch("CLEAR BREAKPOINT/ALL"); err != nil {
		t.Fatalf("Dispatch(CLEAR BREAKPOINT/ALL): %v", err)
	}
	if len(c.Breakpoints) != 0 {
		t.Errorf("expected breakpoints cleared, got %d", len(c.Breakpoints))
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
// DCL /entry= command (ABOUT, FORTH, XTEST, SHOW VERSION — see
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
			// captured by value at Init time (rtl.NewEnvironment's
			// consoleOut), so the buffer this test inspects has to be in
			// place before Init runs, not swapped in afterward.
			var buf strings.Builder
			c := New(&buf)
			if err := c.Init(4096 * 512); err != nil {
				t.Fatalf("Init: %v", err)
			}
			if err := c.VMInit(2000, 100, 0, 4, 4, 4, 4, 8); err != nil {
				t.Fatalf("VMInit: %v", err)
			}

			g := loadEvaxGrammar(t)
			d := NewDispatcher(c, g, nil)

			if _, _, err := c.Assemble(asmFixturePath(t, "kernel.asm")); err != nil {
				t.Fatalf("Assemble(kernel.asm): %v", err)
			}

			c.Engine.SetLimits(100_000, 0)

			if err := d.Dispatch(cmd); err != nil {
				t.Fatalf("Dispatch(%s): %v", cmd, err)
			}

			if out := buf.String(); !strings.HasPrefix(out, "e") {
				t.Errorf("Dispatch(%s) output = %q, want it to start with EXE$ABOUT's banner ('eVAX 1.1...')", cmd, out)
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

	if err := d.Dispatch("SET TRACE"); err != nil {
		t.Fatalf("Dispatch(SET TRACE): %v", err)
	}
	if !c.Trace {
		t.Error("Trace = false, want true after SET TRACE")
	}

	if err := d.Dispatch("SHOW TRACE"); err != nil {
		t.Fatalf("Dispatch(SHOW TRACE): %v", err)
	}

	if err := d.Dispatch("SET NOTRACE"); err != nil {
		t.Fatalf("Dispatch(SET NOTRACE): %v", err)
	}
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

// TestDispatch_setPSL checks SET PSL's own comma-separated field=value
// clause list (cmdSetPSL), including its CUR_MOD alias for SET MODE.
func TestDispatch_setPSL(t *testing.T) {
	d, c := newTestDispatcher(t)

	// IPL=10 is evaluated in the console's default radix 16, so this sets
	// IPL to 16 decimal, not ten -- matching every other numeric literal
	// this evaluator parses.
	if err := d.Dispatch("SET PSL N=1,V=1,IPL=10"); err != nil {
		t.Fatalf("Dispatch(SET PSL): %v", err)
	}
	psl := c.CPU.PSL()
	if !psl.N() || !psl.V() || psl.IPL() != 16 {
		t.Errorf("PSL = %#x, want N/V set and IPL=16 (0x10)", uint32(psl))
	}

	if err := d.Dispatch("SET PSL BOGUS=1"); err == nil {
		t.Error("expected an error for an unknown PSL field")
	}
}

func TestDispatch_setMode(t *testing.T) {
	d, c := newTestDispatcher(t)
	c.CPU.SetPR(vax.ESP, 0x5000)

	if err := d.Dispatch("SET MODE EXEC"); err != nil {
		t.Fatalf("Dispatch(SET MODE): %v", err)
	}
	if got := c.CPU.PSL().CurMod(); got != vax.Executive {
		t.Errorf("CurMod() = %d, want Executive", got)
	}
}

func TestDispatch_setVMAndNoVM(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch("SET VM"); err != nil {
		t.Fatalf("Dispatch(SET VM): %v", err)
	}
	if c.CPU.PR(vax.MAPEN) != 1 {
		t.Errorf("MAPEN = %d, want 1", c.CPU.PR(vax.MAPEN))
	}

	if err := d.Dispatch("SET NOMAPEN"); err != nil {
		t.Fatalf("Dispatch(SET NOMAPEN): %v", err)
	}
	if c.CPU.PR(vax.MAPEN) != 0 {
		t.Errorf("MAPEN = %d, want 0", c.CPU.PR(vax.MAPEN))
	}
}

func TestDispatch_setBaseAndVerbose(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch("SET BASE 3000"); err != nil {
		t.Fatalf("Dispatch(SET BASE): %v", err)
	}
	if c.DepositAddr != 0x3000 {
		t.Errorf("DepositAddr = %#x, want 0x3000", c.DepositAddr)
	}

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

func TestDispatch_setBreakTemporary(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch("SET BREAK/TEMPORARY 400"); err != nil {
		t.Fatalf("Dispatch(SET BREAK/TEMPORARY): %v", err)
	}
	if len(c.Breakpoints) != 1 || !c.Breakpoints[0].Temporary {
		t.Fatalf("Breakpoints = %+v, want one temporary breakpoint", c.Breakpoints)
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
	if !ok || !sym.Permanent {
		t.Errorf("PERMSYM = %+v, ok=%v; want a permanent symbol", sym, ok)
	}

	if err := d.Dispatch("SET/ENTRY ENTRYSYM=200"); err != nil {
		t.Fatalf("Dispatch(SET/ENTRY): %v", err)
	}
	sym, ok = c.Symbols.Find("ENTRYSYM")
	if !ok || !sym.IsEntry {
		t.Errorf("ENTRYSYM = %+v, ok=%v; want an entry symbol", sym, ok)
	}
}

func TestDispatch_setPTEWithRange(t *testing.T) {
	d, c, _ := newShowRunnableDispatcher(t)

	if err := d.Dispatch("SET PTE 200 TO 400 VALID=1,PFN=10"); err != nil {
		t.Fatalf("Dispatch(SET PTE ... TO ...): %v", err)
	}

	for _, addr := range []uint32{0x200, 0x400} {
		_, _, pte, err := c.Mem.LookupPTE(c.CPU, addr)
		if err != nil {
			t.Fatalf("LookupPTE(%#x): %v", addr, err)
		}
		if !pte.Valid() {
			t.Errorf("addr %#x: expected valid bit set", addr)
		}
	}

	// 0x000 (page 0) is below the range and must be untouched.
	if _, _, pte, err := c.Mem.LookupPTE(c.CPU, 0x000); err != nil {
		t.Fatalf("LookupPTE(0x0): %v", err)
	} else if pte.Valid() {
		t.Error("addr 0x0: expected the valid bit untouched (outside the TO range)")
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

	if err := d.Dispatch("CLEAR INTERRUPT/ALL"); err != nil {
		t.Fatalf("Dispatch(CLEAR INTERRUPT/ALL): %v", err)
	}
	if _, queued := c.Engine.PendingInterrupts(); len(queued) != 0 {
		t.Errorf("queued = %+v, want empty after CLEAR INTERRUPT/ALL", queued)
	}
}

func TestDispatch_clearSymbolTemporary(t *testing.T) {
	d, c := newTestDispatcher(t)
	c.Symbols.SetQualified("PERM", 1, true, false, false)
	c.Symbols.SetQualified("TEMP", 2, false, false, false)

	if err := d.Dispatch("CLEAR SYMBOL/TEMPORARY"); err != nil {
		t.Fatalf("Dispatch(CLEAR SYMBOL/TEMPORARY): %v", err)
	}
	if _, ok := c.Symbols.Get("PERM"); !ok {
		t.Error("expected PERM to survive")
	}
	if _, ok := c.Symbols.Get("TEMP"); ok {
		t.Error("expected TEMP to be cleared")
	}
}

// TestDispatch_notImplementedFixedCommand checks BOOT, still gated behind
// cmdNotImplemented (device/RTL support) -- ASM used to be this test's own
// example (its bare, no-filename form returned CLI_NOASMREPL) until
// docs/PHASE-19.md implemented interactive assembler mode; see
// TestDispatchASM_bareEntersInteractiveMode (asm_repl_test.go) for its
// current behavior.
func TestDispatch_notImplementedFixedCommand(t *testing.T) {
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

func TestDispatch_helpFixedCommand(t *testing.T) {
	c, _ := newTestConsole(t)
	g := loadEvaxGrammar(t)
	h := ParseHelp("$HELP\nTop-level help.\n")
	d := NewDispatcher(c, g, h)
	if err := d.Dispatch("HELP"); err != nil {
		t.Fatalf("Dispatch(HELP): %v", err)
	}
}

// TestDispatch_if exercises the IF <expr> [THEN] <command> console verb
// (cmdIf) against the same pattern vax.init uses (IF DEFINED("...") THEN
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

func TestDispatch_emptyLineIsNoop(t *testing.T) {
	d, _ := newTestDispatcher(t)
	if err := d.Dispatch("   "); err != nil {
		t.Fatalf("Dispatch(blank): %v", err)
	}
	if err := d.Dispatch("! a comment"); err != nil {
		t.Fatalf("Dispatch(comment): %v", err)
	}
}
