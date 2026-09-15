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

// movR0Program is "MOVL #0x12345678, R0" followed by HALT: 0xD0 (MOVL),
// 0x8F (immediate-longword source), the 4 immediate bytes, 0x50 (register-
// direct destination, R0), then opHalt -- enough to exercise the
// instruction trace, the DebugRegisters changed-register dump (R0
// changes), and the DebugFullDisasm operand dump (one read, one write
// operand) all at once.
func movR0Program(t *testing.T, c *Console, addr uint32) {
	t.Helper()
	loadProgram(t, c, addr, 0xD0, 0x8F, 0x78, 0x56, 0x34, 0x12, 0x50, opHalt)
}

func TestExecute_tracesWhenTraceEnabled(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)
	c.Trace = true

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[KSP ") || !strings.Contains(out, "MOVL") {
		t.Errorf("output = %q, want a [KSP ...] MOVL trace line", out)
	}
}

func TestExecute_noTraceByDefault(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if strings.Contains(buf.String(), "MOVL") {
		t.Errorf("output = %q, want no MOVL trace line with Trace off", buf.String())
	}
}

func TestStep_alwaysTracesRegardlessOfConsoleTrace(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)
	c.Trace = false

	addr := uint32(0x200)
	if err := c.Step(&addr); err != nil {
		t.Fatalf("Step: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[KSP ") || !strings.Contains(out, "MOVL") {
		t.Errorf("output = %q, want STEP to trace even though Console.Trace is off", out)
	}
}

func TestExecute_debugRegistersChangeDump(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)
	c.Trace = true
	c.CPU.SetDebug(c.CPU.Debug() | vax.DebugRegisters)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "R0:  12345678") {
		t.Errorf("output = %q, want a changed-R0 register dump line", buf.String())
	}
}

func TestExecute_noRegisterDumpWhenDebugRegistersClear(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)
	c.Trace = true
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugRegisters)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if strings.Contains(buf.String(), "R0:  12345678") {
		t.Errorf("output = %q, want no register dump with DebugRegisters clear", buf.String())
	}
}

func TestExecute_debugFullDisasmOperandDump(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)
	c.Trace = true
	c.CPU.SetDebug(c.CPU.Debug() | vax.DebugFullDisasm)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "#0 read") {
		t.Errorf("output = %q, want an operand #0 read line", out)
	}
	if !strings.Contains(out, "#1 write") || !strings.Contains(out, "R0 = 12345678") {
		t.Errorf("output = %q, want an operand #1 write line naming R0 = 12345678", out)
	}
}

func TestCall_returnsCleanlyThroughSentinelFrame(t *testing.T) {
	c, _ := newTestConsole(t)

	// Procedure at 0x200: empty entry mask, CALLS a nested procedure at
	// 0x300, then RET.
	loadProgram(t, c, 0x200,
		0x00, 0x00, // entry mask: no registers saved
		0xFB, 0x00, 0x9F, 0x00, 0x03, 0x00, 0x00, // CALLS #0, @#0x300
		0x04, // RET
	)
	// Nested procedure at 0x300: empty entry mask, RET immediately.
	loadProgram(t, c, 0x300, 0x00, 0x00, 0x04)

	if err := c.Call(0x200, false); err != nil {
		t.Fatalf("Call: %v", err)
	}
}

func TestCall_tracesWhenTraceEnabled(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200,
		0x00, 0x00, // entry mask: no registers saved
		0xFB, 0x00, 0x9F, 0x00, 0x03, 0x00, 0x00, // CALLS #0, @#0x300
		0x04, // RET
	)
	loadProgram(t, c, 0x300, 0x00, 0x00, 0x04)
	c.Trace = true

	if err := c.Call(0x200, false); err != nil {
		t.Fatalf("Call: %v", err)
	}

	if !strings.Contains(buf.String(), "[KSP ") {
		t.Errorf("output = %q, want [KSP ...] trace lines during CALL", buf.String())
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

// TestExecute_stopsAtInstructionLimit exercises govax's own -instruction-limit
// flag (docs/PHASE-15.md's sub-phase 2): an infinite loop (NOP; BRB back to
// itself) must not hang Execute when a limit is configured -- it should stop
// cleanly, reporting the limit rather than propagating an error.
func TestExecute_stopsAtInstructionLimit(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200,
		opNop,
		0x11, 0xFD, // BRB base (displacement -3: back to the NOP)
	)
	c.Engine.SetLimits(5, 0)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v (want a clean stop, not an error)", err)
	}
	if !strings.Contains(buf.String(), "INSTRLIMIT") {
		t.Errorf("output = %q, want an instruction-limit message", buf.String())
	}
}

// TestExecute_beginRunGivesEachCommandAFreshBudget confirms a second Execute
// call gets its own full instruction budget rather than inheriting the
// first call's exhaustion (Engine.BeginRun's own contract).
func TestExecute_beginRunGivesEachCommandAFreshBudget(t *testing.T) {
	c, _ := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop, 0x11, 0xFD)
	c.Engine.SetLimits(3, 0)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("second Execute: %v (want a fresh budget, not immediate exhaustion)", err)
	}
}

func TestExecute_requiresInit(t *testing.T) {
	c := New(&strings.Builder{})
	if err := c.Execute(nil); err == nil {
		t.Error("expected error before Init")
	}
}
