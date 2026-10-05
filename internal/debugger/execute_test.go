package debugger_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestDispatch_clearBreakpoint(t *testing.T) {
	d, c := newTestDispatcher(t)
	dbgOf(c).AddBreakpoint(0x400)

	if err := d.DispatchConsole("CLEAR BREAKPOINT/ALL"); err != nil {
		t.Fatalf("Dispatch(CLEAR BREAKPOINT/ALL): %v", err)
	}

	if len(dbgOf(c).Breakpoints) != 0 {
		t.Errorf("expected breakpoints cleared, got %d", len(dbgOf(c).Breakpoints))
	}
}

func TestDispatch_setBreakTemporary(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.DispatchConsole("SET BREAK/TEMPORARY 400"); err != nil {
		t.Fatalf("Dispatch(SET BREAK/TEMPORARY): %v", err)
	}

	if len(dbgOf(c).Breakpoints) != 1 || !dbgOf(c).Breakpoints[0].Temporary {
		t.Fatalf("Breakpoints = %+v, want one temporary breakpoint", dbgOf(c).Breakpoints)
	}
}

func TestShowBreakpoints(t *testing.T) {
	c, buf := newTestConsole(t)
	buf.Reset()

	if err := dbgOf(c).ShowBreakpoints(); err != nil {
		t.Fatalf("ShowBreakpoints: %v", err)
	}

	if !strings.Contains(buf.String(), "no breakpoints are set") {
		t.Errorf("output = %q, want a no-breakpoints message", buf.String())
	}

	dbgOf(c).AddBreakpoint(0x300)
	buf.Reset()

	if err := dbgOf(c).ShowBreakpoints(); err != nil {
		t.Fatalf("ShowBreakpoints: %v", err)
	}

	if !strings.Contains(buf.String(), "00000300") {
		t.Errorf("output = %q, want it to contain the breakpoint address", buf.String())
	}
}

func TestClearBreakpoint(t *testing.T) {
	c, _ := newTestConsole(t)
	dbgOf(c).AddBreakpoint(0x100)
	dbgOf(c).AddBreakpoint(0x200)

	if err := dbgOf(c).ClearBreakpoint(0x100, false); err != nil {
		t.Fatalf("ClearBreakpoint: %v", err)
	}

	if len(dbgOf(c).Breakpoints) != 1 {
		t.Errorf("len(Breakpoints) = %d, want 1", len(dbgOf(c).Breakpoints))
	}

	if err := dbgOf(c).ClearBreakpoint(0, true); err != nil {
		t.Fatalf("ClearBreakpoint: %v", err)
	}

	if len(dbgOf(c).Breakpoints) != 0 {
		t.Errorf("len(Breakpoints) = %d, want 0", len(dbgOf(c).Breakpoints))
	}
}

func TestExecute_stopsAtBreakpoint(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop, opNop, opNop, opHalt)
	dbgOf(c).AddBreakpoint(0x202)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x202 {
		t.Errorf("PC at breakpoint = %#x, want 0x202", got)
	}

	if !strings.Contains(buf.String(), "break at") {
		t.Errorf("output = %q, want a break message", buf.String())
	}
}

func TestExecute_breakpointAtStartDoesNotStopImmediately(t *testing.T) {
	c, _ := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop, opHalt)
	dbgOf(c).AddBreakpoint(0x200)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Should run to completion (HALT), not stop immediately at the
	// breakpoint it started on. PC lands past both instructions (decode
	// advances PC before a Handler, including HALT's, runs).
	if got := c.CPU.GPR(vax.PC); got != 0x202 {
		t.Errorf("PC after halt = %#x, want 0x202", got)
	}
}

func TestStep_advancesOneInstruction(t *testing.T) {
	c, buf := newTestConsole(t)
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugUserStep)
	loadProgram(t, c, 0x200, opNop, opNop)

	addr := uint32(0x200)
	if err := c.Step(&addr, "INTO"); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x201 {
		t.Errorf("PC after one step = %#x, want 0x201", got)
	}

	if !strings.Contains(buf.String(), "stepped to") {
		t.Errorf("output = %q, want a step message", buf.String())
	}

	if err := c.Step(nil, "INTO"); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x202 {
		t.Errorf("PC after second step = %#x, want 0x202", got)
	}
}

func TestBreakpoints_addRemoveClear(t *testing.T) {
	c, _ := newTestConsole(t)
	dbgOf(c).AddBreakpoint(0x300)
	dbgOf(c).AddBreakpoint(0x300) // duplicate, should not double-add
	dbgOf(c).AddBreakpoint(0x400)

	if len(dbgOf(c).Breakpoints) != 2 {
		t.Fatalf("len(Breakpoints) = %d, want 2", len(dbgOf(c).Breakpoints))
	}

	dbgOf(c).RemoveBreakpoint(0x300)

	if len(dbgOf(c).Breakpoints) != 1 || dbgOf(c).Breakpoints[0].Addr != 0x400 {
		t.Errorf("RemoveBreakpoint left unexpected state: %+v", dbgOf(c).Breakpoints)
	}

	dbgOf(c).ClearAllBreakpoints()

	if len(dbgOf(c).Breakpoints) != 0 {
		t.Errorf("ClearAllBreakpoints left %d breakpoints", len(dbgOf(c).Breakpoints))
	}
}

// TestReportStopReason_attention is a direct, non-racy unit test of the
// message reportStopReason prints for ErrAttention, complementing
// TestExecute_stopsOnAttention's own end-to-end (if inherently
// timing-dependent) coverage above.

// TestStep_tracesOnlyWhenConsoleTraceIsOn: a STEP reports where it landed
// and nothing else, as the VMS debugger does; SET TRACE adds the console's
// trace line for each instruction it executes.
func TestStep_tracesOnlyWhenConsoleTraceIsOn(t *testing.T) {
	c, buf := newTestConsole(t)
	noUserStep(c)
	movR0Program(t, c, 0x200)
	c.Trace = false

	addr := uint32(0x200)
	if err := c.Step(&addr, "INTO"); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if out := buf.String(); strings.Contains(out, "[KSP ") || !strings.HasPrefix(out, "stepped to ") {
		t.Errorf("output = %q, want only the stepped-to line", out)
	}

	buf.Reset()

	c.Trace = true

	if err := c.Step(nil, "INTO"); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if out := buf.String(); !strings.Contains(out, "[KSP ") {
		t.Errorf("output = %q, want the trace line with SET TRACE", out)
	}
}
