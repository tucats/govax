package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

const (
	opHalt = 0x00
	opNop  = 0x01
)

func loadProgram(t *testing.T, c *Console, addr uint32, bytes ...byte) {
	t.Helper()
	for i, b := range bytes {
		if err := c.Deposit("", addr+uint32(i), SizeByte, uint32(b)); err != nil {
			t.Fatalf("Deposit: %v", err)
		}
	}
}

func TestExecute_runsUntilHalt(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop, opNop, opNop, opHalt)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "HALT") {
		t.Errorf("output = %q, want a HALT message", buf.String())
	}
	if got := c.CPU.GPR(vax.PC); got != 0x204 {
		t.Errorf("PC after halt = %#x, want 0x204", got)
	}
}

func TestExecute_stopsAtBreakpoint(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop, opNop, opNop, opHalt)
	c.AddBreakpoint(0x202)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := c.CPU.GPR(vax.PC); got != 0x202 {
		t.Errorf("PC at breakpoint = %#x, want 0x202", got)
	}
	if !strings.Contains(buf.String(), "Break at") {
		t.Errorf("output = %q, want a break message", buf.String())
	}
}

func TestExecute_breakpointAtStartDoesNotStopImmediately(t *testing.T) {
	c, _ := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop, opHalt)
	c.AddBreakpoint(0x200)

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
	loadProgram(t, c, 0x200, opNop, opNop)

	addr := uint32(0x200)
	if err := c.Step(&addr); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got := c.CPU.GPR(vax.PC); got != 0x201 {
		t.Errorf("PC after one step = %#x, want 0x201", got)
	}
	if !strings.Contains(buf.String(), "Stepped to") {
		t.Errorf("output = %q, want a step message", buf.String())
	}

	if err := c.Step(nil); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got := c.CPU.GPR(vax.PC); got != 0x202 {
		t.Errorf("PC after second step = %#x, want 0x202", got)
	}
}

func TestBreakpoints_addRemoveClear(t *testing.T) {
	c, _ := newTestConsole(t)
	c.AddBreakpoint(0x300)
	c.AddBreakpoint(0x300) // duplicate, should not double-add
	c.AddBreakpoint(0x400)
	if len(c.Breakpoints) != 2 {
		t.Fatalf("len(Breakpoints) = %d, want 2", len(c.Breakpoints))
	}

	c.RemoveBreakpoint(0x300)
	if len(c.Breakpoints) != 1 || c.breakpointAt(0x400) == nil {
		t.Errorf("RemoveBreakpoint left unexpected state: %+v", c.Breakpoints)
	}

	c.ClearAllBreakpoints()
	if len(c.Breakpoints) != 0 {
		t.Errorf("ClearAllBreakpoints left %d breakpoints", len(c.Breakpoints))
	}
}

func TestExecute_requiresInit(t *testing.T) {
	c := New(&strings.Builder{})
	if err := c.Execute(nil); err == nil {
		t.Error("expected error before Init")
	}
}
