package debugger_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

func TestInstructionBreakpoints_addRemoveClear(t *testing.T) {
	c, buf := newTestConsole(t)

	if err := dbgOf(c).AddInstructionBreakpoint("nop"); err != nil { // lower-case: must fold
		t.Fatalf("AddInstructionBreakpoint: %v", err)
	}

	nop := cpu.Instructions().ByName("NOP")
	if !dbgOf(c).InstructionBreakpoints[nop] {
		t.Fatalf("InstructionBreakpoints does not contain NOP after Add")
	}

	if !strings.Contains(buf.String(), "Breakpoint set on instruction 01 NOP") {
		t.Errorf("output = %q, want a confirmation naming the opcode and mnemonic", buf.String())
	}

	buf.Reset()

	if err := dbgOf(c).RemoveInstructionBreakpoint("NOP"); err != nil {
		t.Fatalf("RemoveInstructionBreakpoint: %v", err)
	}

	if dbgOf(c).InstructionBreakpoints[nop] {
		t.Fatalf("InstructionBreakpoints still contains NOP after Remove")
	}

	if !strings.Contains(buf.String(), "Removed breakpoint on instruction 01 NOP") {
		t.Errorf("output = %q, want a removal confirmation", buf.String())
	}

	// Removing an opcode that was never flagged is a silent no-op, matching
	// RemoveBreakpoint's own address-breakpoint behavior.
	buf.Reset()

	if err := dbgOf(c).RemoveInstructionBreakpoint("NOP"); err != nil {
		t.Fatalf("RemoveInstructionBreakpoint (no-op): %v", err)
	}

	if buf.String() != "" {
		t.Errorf("output = %q, want no output for removing an unset breakpoint", buf.String())
	}

	if err := dbgOf(c).AddInstructionBreakpoint("HALT"); err != nil {
		t.Fatalf("AddInstructionBreakpoint(HALT): %v", err)
	}

	if err := dbgOf(c).AddInstructionBreakpoint("NOP"); err != nil {
		t.Fatalf("AddInstructionBreakpoint(NOP): %v", err)
	}

	buf.Reset()

	if err := dbgOf(c).ClearAllInstructionBreakpoints(); err != nil {
		t.Fatalf("ClearAllInstructionBreakpoints: %v", err)
	}

	if len(dbgOf(c).InstructionBreakpoints) != 0 {
		t.Errorf("InstructionBreakpoints left %d entries after ClearAll", len(dbgOf(c).InstructionBreakpoints))
	}

	if !strings.Contains(buf.String(), "Cleared 2 instruction breakpoints") {
		t.Errorf("output = %q, want a count summary", buf.String())
	}
}

func TestInstructionBreakpoints_unknownOpcode(t *testing.T) {
	c, _ := newTestConsole(t)

	if err := dbgOf(c).AddInstructionBreakpoint("BOGUSOP"); err == nil {
		t.Error("expected an error for an unrecognized mnemonic")
	}

	if err := dbgOf(c).RemoveInstructionBreakpoint("BOGUSOP"); err == nil {
		t.Error("expected an error for an unrecognized mnemonic")
	}
}

func TestShowInstructionBreakpoints_emptyAndSet(t *testing.T) {
	c, buf := newTestConsole(t)

	if err := dbgOf(c).ShowInstructionBreakpoints(); err != nil {
		t.Fatalf("ShowInstructionBreakpoints: %v", err)
	}

	if !strings.Contains(buf.String(), "No instruction breakpoints set") {
		t.Errorf("output = %q, want the empty-list message", buf.String())
	}

	buf.Reset()

	if err := dbgOf(c).AddInstructionBreakpoint("NOP"); err != nil {
		t.Fatalf("AddInstructionBreakpoint: %v", err)
	}

	buf.Reset()

	if err := dbgOf(c).ShowInstructionBreakpoints(); err != nil {
		t.Fatalf("ShowInstructionBreakpoints: %v", err)
	}

	if !strings.Contains(buf.String(), "01  NOP") || !strings.Contains(buf.String(), "1 instruction breakpoint set") {
		t.Errorf("output = %q, want the opcode listed and a singular count summary", buf.String())
	}
}

// TestExecute_stopsAtInstructionBreakpoint sets a breakpoint on HALT and
// confirms execution stops right before HALT would run -- not after, the
// way reportStopReason's own "HALT instruction executed" message would
// read -- matching decode_opcode.c's own OP_DBG_BREAK check firing before
// the instruction's Handler is ever invoked.
func TestExecute_stopsAtInstructionBreakpoint(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop, opNop, opHalt)

	if err := dbgOf(c).AddInstructionBreakpoint("HALT"); err != nil {
		t.Fatalf("AddInstructionBreakpoint: %v", err)
	}

	buf.Reset()

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x202 {
		t.Errorf("PC at instruction breakpoint = %#x, want 0x202 (HALT never ran)", got)
	}

	if !strings.Contains(buf.String(), "Instruction break at 00000202") {
		t.Errorf("output = %q, want an instruction-break message", buf.String())
	}

	if strings.Contains(buf.String(), "HALT instruction executed") {
		t.Errorf("output = %q, HALT must not have actually executed", buf.String())
	}
}

// TestExecute_instructionBreakpointAtStartDoesNotStopImmediately confirms
// the same skip-first-check rule address breakpoints get (runLoop's
// skipFirstCheck) also applies to instruction breakpoints: starting exactly
// on a flagged opcode must not stop before that instruction ever runs, but
// the same opcode occurring again later must still be caught.
func TestExecute_instructionBreakpointAtStartDoesNotStopImmediately(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop, opNop, opHalt)

	if err := dbgOf(c).AddInstructionBreakpoint("NOP"); err != nil {
		t.Fatalf("AddInstructionBreakpoint: %v", err)
	}

	buf.Reset()

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// The NOP at 0x200 (the run's own starting point) must run; the second
	// NOP at 0x201 is a fresh address and must stop.
	if got := c.CPU.GPR(vax.PC); got != 0x201 {
		t.Errorf("PC = %#x, want 0x201 (stopped at the second NOP, not the first)", got)
	}

	if !strings.Contains(buf.String(), "Instruction break at 00000201") {
		t.Errorf("output = %q, want an instruction-break message at 0x201", buf.String())
	}
}

// TestDispatch_instructionBreakpoints: the VMS spelling of an instruction
// breakpoint, SET BREAK/INSTRUCTION=opcode, stops before each execution of
// that opcode (but not at the instruction GO starts from), SHOW BREAK lists
// it, and CANCEL BREAK/INSTRUCTION=opcode takes it away.
func TestDispatch_instructionBreakpoints(t *testing.T) {
	c, buf := newTestConsole(t)

	say(t, c, "SET BREAK/INSTRUCTION=NOP")

	if out := say(t, c, "SHOW BREAK"); !strings.Contains(out, "NOP") {
		t.Errorf("SHOW BREAK = %q, want NOP listed", out)
	}

	loadProgram(t, c, 0x200, opNop, opNop, opHalt)
	c.CPU.SetGPR(vax.PC, 0x200)
	buf.Reset()

	if err := c.Debugger.Dispatch("GO"); err != nil {
		t.Fatalf("GO: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x201 {
		t.Errorf("PC after GO = %#x, want 0x201 (stopped at the second NOP)", got)
	}

	if !strings.Contains(buf.String(), "break on instruction(s) at 00000201") {
		t.Errorf("output = %q, want an instruction-break message at 0x201", buf.String())
	}

	say(t, c, "CANCEL BREAK/INSTRUCTION=NOP")

	if out := say(t, c, "SHOW BREAK"); strings.Contains(out, "NOP") {
		t.Errorf("SHOW BREAK after the cancel = %q, want no NOP", out)
	}
}

// TestDispatch_setBreakInstructionBadOpcode: an opcode that is no
// instruction's mnemonic is refused.
func TestDispatch_setBreakInstructionBadOpcode(t *testing.T) {
	c, _ := newTestConsole(t)

	if _, err := sayErr(c, "SET BREAK/INSTRUCTION=BOGUSOP"); err == nil {
		t.Error("expected an error for SET BREAK/INSTRUCTION with an unknown mnemonic")
	}
}

func TestDispatch_setBreakBadQualifier(t *testing.T) {
	c, _ := newTestConsole(t)

	if _, err := sayErr(c, "SET BREAK/BOGUS 100"); err == nil {
		t.Error("expected an error for an unrecognized SET BREAK qualifier")
	}
}
