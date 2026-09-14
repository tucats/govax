package console

import (
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

func TestDispatch_runIsRemappedToExecute(t *testing.T) {
	d, c := newTestDispatcher(t)
	loadProgram(t, c, 0x200, opHalt)
	if err := d.Dispatch("RUN 200"); err != nil {
		t.Fatalf("Dispatch(RUN): %v", err)
	}
	if !c.Engine.Halted() {
		t.Error("expected RUN to execute until HALT")
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
	err := d.Dispatch("SHOW DEVICES")
	if err == nil {
		t.Error("expected an error for the unimplemented SHOW DEVICES")
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
