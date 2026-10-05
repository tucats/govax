package debugger_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/debugger"
	"github.com/tucats/govax/internal/vax"
)

// loadCallProgram lays out the same CALLS/RET pair execute_test.go's
// TestCall_returnsCleanlyThroughSentinelFrame uses: a procedure at 0x200
// (empty entry mask, CALLS a nested procedure at 0x300, then RET) and the
// nested procedure at 0x300 (empty entry mask, RET immediately). The CALLS
// instruction occupies 0x202-0x208 (7 bytes); its own RET sits at 0x209.
func loadCallProgram(t *testing.T, c *console.Console) {
	t.Helper()

	loadProgram(t, c, 0x200,
		0x00, 0x00, // entry mask: no registers saved
		0xFB, 0x00, 0x9F, 0x00, 0x03, 0x00, 0x00, // CALLS #0, @#0x300
		0x04, // RET
	)
	loadProgram(t, c, 0x300, 0x00, 0x00, 0x04) // entry mask, RET
}

func TestStepOver_runsCallToCompletion(t *testing.T) {
	c, buf := newTestConsole(t)
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugUserStep)
	loadCallProgram(t, c)

	addr := uint32(0x202) // the CALLS instruction itself
	if err := c.Step(&addr, "OVER"); err != nil {
		t.Fatalf("Step/OVER: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x209 {
		t.Errorf("PC after STEP/OVER = %#x, want 0x209 (past the CALLS, not inside it)", got)
	}

	if out := buf.String(); out != "stepped to 00000209: RET\n" {
		t.Errorf("output = %q, want the step's report", out)
	}
}

// TestStepOver_silentInsideCalledRoutine confirms STEP/OVER's traversal of
// the called routine is silent even with Console.Trace on -- vax.c forces
// its own local disasm flag to zero for exactly this span (see step.go's
// stepUnit doc comment) -- while the CALLS instruction itself is still
// traced exactly once.
func TestStepOver_silentInsideCalledRoutine(t *testing.T) {
	c, buf := newTestConsole(t)
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugUserStep)
	loadCallProgram(t, c)
	c.Trace = true

	addr := uint32(0x202)
	if err := c.Step(&addr, "OVER"); err != nil {
		t.Fatalf("Step/OVER: %v", err)
	}

	out := buf.String()
	if strings.Count(out, "[KSP ") != 1 {
		t.Errorf("output = %q, want exactly one traced instruction line (the CALLS itself)", out)
	}
}

func TestStepOver_ordinaryInstructionActsLikeStepInto(t *testing.T) {
	c, buf := newTestConsole(t)
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugUserStep)
	loadProgram(t, c, 0x200, opNop, opNop)

	addr := uint32(0x200)
	if err := c.Step(&addr, "OVER"); err != nil {
		t.Fatalf("Step/OVER: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x201 {
		t.Errorf("PC after STEP/OVER of a NOP = %#x, want 0x201", got)
	}

	if !strings.Contains(buf.String(), "stepped to") {
		t.Errorf("output = %q, want a step message", buf.String())
	}
}

// TestStepReturn_stopsAtTheFramesRet: STEP/RETURN stops *at* the RET that
// ends the frame (before it runs), reporting where it was given and where
// it stopped, and the next STEP executes the RET (docs/PHASE-42.md,
// subtask 7).
func TestStepReturn_stopsAtTheFramesRet(t *testing.T) {
	c, buf := newTestConsole(t)
	noUserStep(c)
	loadCallProgram(t, c)

	// Manually execute the CALLS instruction (bypassing Console.Step) to
	// land inside the nested procedure with a real frame established.
	c.CPU.SetGPR(vax.PC, 0x202)
	c.Engine.BeginRun()

	if err := c.Engine.Step(); err != nil {
		t.Fatalf("Engine.Step (CALLS): %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x302 {
		t.Fatalf("PC after CALLS = %#x, want 0x302 (inside the nested procedure)", got)
	}

	if err := c.Step(nil, "RETURN"); err != nil {
		t.Fatalf("Step/RETURN: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x302 {
		t.Errorf("PC after STEP/RETURN = %#x, want 0x302 (at the RET, not past it)", got)
	}

	if got, want := buf.String(), "stepped on return from 00000302 to 00000302: RET\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}

	buf.Reset()

	if err := c.Step(nil, "OVER"); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x209 {
		t.Errorf("PC after the RET = %#x, want 0x209 (back in the caller)", got)
	}
}

// TestStepReturn_boundToItsFrame: a RET of a deeper frame doesn't satisfy
// the wait; the RET of the frame the command was given in does. Here the
// command is given in the outer procedure, which calls the nested one.
func TestStepReturn_boundToItsFrame(t *testing.T) {
	c, buf := newTestConsole(t)
	noUserStep(c)
	loadCallProgram(t, c)

	// Enter the outer procedure as a CALLS would, so it has a frame.
	if err := c.Engine.CallEntry(0x200); err != nil {
		t.Fatalf("CallEntry: %v", err)
	}

	if err := c.Step(nil, "RETURN"); err != nil {
		t.Fatalf("Step/RETURN: %v", err)
	}

	// The nested RET at 0x302 went by; the outer RET at 0x209 is the stop.
	if got := c.CPU.GPR(vax.PC); got != 0x209 {
		t.Errorf("PC after STEP/RETURN = %#x, want 0x209 (the outer frame's RET)", got)
	}

	if got, want := buf.String(), "stepped on return from 00000202 to 00000209: RET\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestStepReturn_noFramesReportsError(t *testing.T) {
	c, _ := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop)

	addr := uint32(0x200)
	if err := c.Step(&addr, "RETURN"); err == nil {
		t.Fatal("expected an error with no call frame established (FP/AP both zero)")
	}
}

func TestStep_respectsBreakpointHitDuringStepOver(t *testing.T) {
	c, buf := newTestConsole(t)
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugUserStep)
	loadCallProgram(t, c)
	dbgOf(c).AddBreakpoint(0x302) // a real, permanent breakpoint at the callee's entry

	addr := uint32(0x202)
	if err := c.Step(&addr, "OVER"); err != nil {
		t.Fatalf("Step/OVER: %v", err)
	}

	// The permanent breakpoint at the callee's entry must still fire (STEP/
	// OVER's continuation phase does not treat that address as its own
	// "starting point" the way the very first STEP command's PC is -- see
	// runLoop's skipFirstCheck doc comment), reporting "break at", not
	// "Stepped to".
	if got := c.CPU.GPR(vax.PC); got != 0x302 {
		t.Errorf("PC = %#x, want 0x302 (stopped at the permanent breakpoint)", got)
	}

	if !strings.Contains(buf.String(), "break at") {
		t.Errorf("output = %q, want a break message", buf.String())
	}
}

func TestSetStepMode_andShowStepMode(t *testing.T) {
	c, buf := newTestConsole(t)

	db := dbgOf(c)

	if db.StepMode != debugger.StepOver {
		t.Errorf("StepMode default = %v, want OVER (VMS's default)", db.StepMode)
	}

	for _, tc := range []struct {
		word string
		want debugger.StepMode
	}{
		{"OVER", debugger.StepOver},
		{"/OVER", debugger.StepOver},
		{"into", debugger.StepInto},
		{"/RETURN", debugger.StepReturn},
		{"RET", debugger.StepReturn},
	} {
		if err := db.SetStepMode(tc.word); err != nil {
			t.Fatalf("SetStepMode(%q): %v", tc.word, err)
		}

		if db.StepMode != tc.want {
			t.Errorf("SetStepMode(%q): StepMode = %v, want %v", tc.word, db.StepMode, tc.want)
		}
	}

	if err := db.SetStepMode("BOGUS"); err == nil {
		t.Error("expected an error for an unrecognized SET STEP mode")
	}

	buf.Reset()

	db.StepMode = debugger.StepOver

	if err := db.ShowStepMode(); err != nil {
		t.Fatalf("ShowStepMode: %v", err)
	}

	if want := "step type: source, nosilent, by line,\n           over routine calls\n"; buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestDispatch_stepQualifiers(t *testing.T) {
	d, c := newTestDispatcher(t)
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugUserStep)
	loadCallProgram(t, c)

	if err := d.DispatchConsole("SET STEP OVER"); err != nil {
		t.Fatalf("Dispatch(SET STEP OVER): %v", err)
	}

	if dbgOf(c).StepMode != debugger.StepOver {
		t.Fatalf("StepMode = %v, want OVER", dbgOf(c).StepMode)
	}

	if err := d.DispatchConsole("SHOW STEP_MODE"); err != nil {
		t.Fatalf("Dispatch(SHOW STEP_MODE): %v", err)
	}

	// A bare STEP now defaults to OVER (per the SET STEP above) and should
	// run the CALLS at 0x202 to completion.
	if err := d.DispatchConsole("DEP PC = 202"); err != nil {
		t.Fatalf("Dispatch(DEPOSIT PC): %v", err)
	}

	if err := d.DispatchConsole("STEP"); err != nil {
		t.Fatalf("Dispatch(STEP): %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x209 {
		t.Errorf("PC after bare STEP (default OVER) = %#x, want 0x209", got)
	}

	// An explicit /INTO overrides the OVER default for one invocation.
	if err := d.DispatchConsole("DEP PC = 202"); err != nil {
		t.Fatalf("Dispatch(DEPOSIT PC): %v", err)
	}

	if err := d.DispatchConsole("STEP/INTO"); err != nil {
		t.Fatalf("Dispatch(STEP/INTO): %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x302 {
		t.Errorf("PC after STEP/INTO = %#x, want 0x302 (stepped into the call)", got)
	}
}
