package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

func TestInstructionBreakpoints_addRemoveClear(t *testing.T) {
	c, buf := newTestConsole(t)

	if err := c.AddInstructionBreakpoint("nop"); err != nil { // lower-case: must fold
		t.Fatalf("AddInstructionBreakpoint: %v", err)
	}

	nop := cpu.Instructions().ByName("NOP")
	if !c.InstructionBreakpoints[nop] {
		t.Fatalf("InstructionBreakpoints does not contain NOP after Add")
	}
	if !strings.Contains(buf.String(), "Breakpoint set on instruction 01 NOP") {
		t.Errorf("output = %q, want a confirmation naming the opcode and mnemonic", buf.String())
	}

	buf.Reset()
	if err := c.RemoveInstructionBreakpoint("NOP"); err != nil {
		t.Fatalf("RemoveInstructionBreakpoint: %v", err)
	}
	if c.InstructionBreakpoints[nop] {
		t.Fatalf("InstructionBreakpoints still contains NOP after Remove")
	}
	if !strings.Contains(buf.String(), "Removed breakpoint on instruction 01 NOP") {
		t.Errorf("output = %q, want a removal confirmation", buf.String())
	}

	// Removing an opcode that was never flagged is a silent no-op, matching
	// RemoveBreakpoint's own address-breakpoint behavior.
	buf.Reset()
	if err := c.RemoveInstructionBreakpoint("NOP"); err != nil {
		t.Fatalf("RemoveInstructionBreakpoint (no-op): %v", err)
	}
	if buf.String() != "" {
		t.Errorf("output = %q, want no output for removing an unset breakpoint", buf.String())
	}

	if err := c.AddInstructionBreakpoint("HALT"); err != nil {
		t.Fatalf("AddInstructionBreakpoint(HALT): %v", err)
	}
	if err := c.AddInstructionBreakpoint("NOP"); err != nil {
		t.Fatalf("AddInstructionBreakpoint(NOP): %v", err)
	}

	buf.Reset()
	if err := c.ClearAllInstructionBreakpoints(); err != nil {
		t.Fatalf("ClearAllInstructionBreakpoints: %v", err)
	}
	if len(c.InstructionBreakpoints) != 0 {
		t.Errorf("InstructionBreakpoints left %d entries after ClearAll", len(c.InstructionBreakpoints))
	}
	if !strings.Contains(buf.String(), "Cleared 2 instruction breakpoints") {
		t.Errorf("output = %q, want a count summary", buf.String())
	}
}

func TestInstructionBreakpoints_unknownOpcode(t *testing.T) {
	c, _ := newTestConsole(t)

	if err := c.AddInstructionBreakpoint("BOGUSOP"); err == nil {
		t.Error("expected an error for an unrecognized mnemonic")
	}
	if err := c.RemoveInstructionBreakpoint("BOGUSOP"); err == nil {
		t.Error("expected an error for an unrecognized mnemonic")
	}
}

func TestShowInstructionBreakpoints_emptyAndSet(t *testing.T) {
	c, buf := newTestConsole(t)

	if err := c.ShowInstructionBreakpoints(); err != nil {
		t.Fatalf("ShowInstructionBreakpoints: %v", err)
	}
	if !strings.Contains(buf.String(), "No instruction breakpoints set") {
		t.Errorf("output = %q, want the empty-list message", buf.String())
	}

	buf.Reset()
	if err := c.AddInstructionBreakpoint("NOP"); err != nil {
		t.Fatalf("AddInstructionBreakpoint: %v", err)
	}
	buf.Reset()
	if err := c.ShowInstructionBreakpoints(); err != nil {
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

	if err := c.AddInstructionBreakpoint("HALT"); err != nil {
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

	if err := c.AddInstructionBreakpoint("NOP"); err != nil {
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

func TestDispatch_instructionBreakpoints(t *testing.T) {
	c, buf := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	if err := d.Dispatch("SET BREAK/INSTRUCTION NOP"); err != nil {
		t.Fatalf("Dispatch(SET BREAK/INSTRUCTION NOP): %v", err)
	}
	if !c.InstructionBreakpoints[cpu.Instructions().ByName("NOP")] {
		t.Fatal("SET BREAK/INSTRUCTION NOP did not flag NOP")
	}

	buf.Reset()
	if err := d.Dispatch("SHOW BREAKPOINTS/INSTRUCTIONS"); err != nil {
		t.Fatalf("Dispatch(SHOW BREAKPOINTS/INSTRUCTIONS): %v", err)
	}
	if !strings.Contains(buf.String(), "NOP") {
		t.Errorf("output = %q, want NOP listed", buf.String())
	}

	loadProgram(t, c, 0x200, opNop, opNop, opHalt)
	buf.Reset()
	if err := d.Dispatch("EXEC 200"); err != nil {
		t.Fatalf("Dispatch(EXEC 200): %v", err)
	}
	if got := c.CPU.GPR(vax.PC); got != 0x201 {
		t.Errorf("PC after EXEC = %#x, want 0x201 (stopped at the second NOP)", got)
	}

	if err := d.Dispatch("CLEAR BREAKPOINT/INSTRUCTION NOP"); err != nil {
		t.Fatalf("Dispatch(CLEAR BREAKPOINT/INSTRUCTION NOP): %v", err)
	}
	if c.InstructionBreakpoints[cpu.Instructions().ByName("NOP")] {
		t.Fatal("CLEAR BREAKPOINT/INSTRUCTION NOP left NOP flagged")
	}

	if err := d.Dispatch("SET BREAK/INSTRUCTION HALT"); err != nil {
		t.Fatalf("Dispatch(SET BREAK/INSTRUCTION HALT): %v", err)
	}
	if err := d.Dispatch("CLEAR BREAKPOINT/INSTRUCTION/ALL"); err != nil {
		t.Fatalf("Dispatch(CLEAR BREAKPOINT/INSTRUCTION/ALL): %v", err)
	}
	if len(c.InstructionBreakpoints) != 0 {
		t.Errorf("InstructionBreakpoints left %d entries after CLEAR .../ALL", len(c.InstructionBreakpoints))
	}
}

func TestDispatch_setBreakInstructionRequiresOpcode(t *testing.T) {
	d, _ := newTestDispatcher(t)

	if err := d.Dispatch("SET BREAK/INSTRUCTION"); err == nil {
		t.Error("expected an error for SET BREAK/INSTRUCTION with no mnemonic")
	}
}

func TestDispatch_setBreakBadQualifier(t *testing.T) {
	d, _ := newTestDispatcher(t)

	if err := d.Dispatch("SET BREAK/BOGUS 100"); err == nil {
		t.Error("expected an error for an unrecognized SET BREAK qualifier")
	}
}
