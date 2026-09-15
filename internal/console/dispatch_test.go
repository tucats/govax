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

func TestDispatch_entryPointCommandErrors(t *testing.T) {
	d, _ := newTestDispatcher(t)
	err := d.Dispatch("ABOUT")
	if err == nil || !strings.Contains(err.Error(), "RTL microkernel") {
		t.Errorf("Dispatch(ABOUT) = %v, want an RTL-not-implemented error", err)
	}
}

func TestDispatch_unboundShowSubformErrors(t *testing.T) {
	d, _ := newTestDispatcher(t)
	err := d.Dispatch("SHOW NVRAM")
	if err == nil {
		t.Error("expected an error for the unimplemented SHOW NVRAM")
	}
}

func TestDispatch_notImplementedFixedCommand(t *testing.T) {
	d, _ := newTestDispatcher(t)
	if err := d.Dispatch("ASM"); err == nil {
		t.Error("expected an error for ASM")
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

func TestDispatch_emptyLineIsNoop(t *testing.T) {
	d, _ := newTestDispatcher(t)
	if err := d.Dispatch("   "); err != nil {
		t.Fatalf("Dispatch(blank): %v", err)
	}
	if err := d.Dispatch("! a comment"); err != nil {
		t.Fatalf("Dispatch(comment): %v", err)
	}
}
