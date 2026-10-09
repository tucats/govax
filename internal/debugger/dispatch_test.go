package debugger_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vax"
)

func TestDispatch_stepAndGo(t *testing.T) {
	_, c := newTestDispatcher(t)
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugUserStep)
	loadProgram(t, c, 0x200, opNop, opNop, opHalt)

	c.CPU.SetGPR(vax.PC, 0x200)

	if err := c.Debugger.Dispatch("STEP"); err != nil {
		t.Fatalf("Dispatch(STEP): %v", err)
	}

	if c.CPU.GPR(vax.PC) != 0x201 {
		t.Errorf("PC after STEP = %#x, want 0x201", c.CPU.GPR(vax.PC))
	}

	if err := c.Debugger.Dispatch("GO"); err != nil {
		t.Fatalf("Dispatch(GO): %v", err)
	}

	if c.CPU.GPR(vax.PC) != 0x203 {
		t.Errorf("PC after GO = %#x, want 0x203", c.CPU.GPR(vax.PC))
	}
}

// TestDispatch_callStepQualifier checks CALL/STEP is accepted (parsed and
// dispatched without error) -- console_call's own /STEP|/BREAK|/DEBUG
// qualifier, matching RUN's identical convention.
func TestDispatch_callStepQualifier(t *testing.T) {
	c := newRunnableConsole(t)
	g := consoletest.ConsoleGrammar(t)
	d := console.NewDispatcher(c, g, nil)

	if err := d.DispatchConsole(`ASM "` + consoletest.RepoPath(t, "testdata", "asm", "xor.asm") + `"`); err != nil {
		t.Fatalf("Dispatch(ASM): %v", err)
	}

	if err := c.Debugger.Dispatch("CALL/STEP TEST"); err != nil {
		t.Fatalf("Dispatch(CALL/STEP): %v", err)
	}
}

// TestDispatch_callStepStopsAfterOneInstruction confirms CALL/STEP executes
// only the entered procedure's first instruction and hands control back to
// the console -- console_exec.c's console_call delegates straight to
// console_step (a single instruction) rather than running to completion, so
// the rest must be walked with explicit STEP commands. A prior version of
// Console.Call instead looped through every instruction internally, only
// printing "Stepped to" without ever actually stopping.
func TestDispatch_callStepStopsAfterOneInstruction(t *testing.T) {
	c := newRunnableConsole(t)
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugUserStep)

	g := consoletest.ConsoleGrammar(t)
	d := console.NewDispatcher(c, g, nil)

	src := "\t.entry\tdbltest, ^m<>\n\tmovl\t4(ap), r0\n\taddl2\tr0, r0\n\tret\n\t.end\n"
	path := filepath.Join(t.TempDir(), "dbl_test.asm")

	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := d.DispatchConsole(`ASM "` + path + `"`); err != nil {
		t.Fatalf("Dispatch(ASM): %v", err)
	}

	if err := c.Debugger.Dispatch("CALL/STEP DBLTEST(^D21)"); err != nil {
		t.Fatalf("Dispatch(CALL/STEP): %v", err)
	}

	// Only the MOVL has executed: R0 holds the argument, not yet doubled.
	if got := c.CPU.GPR(vax.R0); got != 21 {
		t.Errorf("R0 after CALL/STEP = %d, want 21 (only the first instruction should have run)", got)
	}

	// STEP continues past the ADDL2 ...
	if err := c.Debugger.Dispatch("STEP"); err != nil {
		t.Fatalf("Dispatch(STEP) [ADDL2]: %v", err)
	}

	if got := c.CPU.GPR(vax.R0); got != 42 {
		t.Errorf("R0 after STEP = %d, want 42", got)
	}

	// ... and a further STEP executes the RET, cleanly returning control to
	// the console (no error) rather than erroring on the internal
	// console-call-completion signal.
	if err := c.Debugger.Dispatch("STEP"); err != nil {
		t.Fatalf("Dispatch(STEP) [RET]: %v", err)
	}
}
