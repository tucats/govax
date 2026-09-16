package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

func TestFaultBreakpoints_addRemoveClear(t *testing.T) {
	c, _ := newTestConsole(t)

	if err := c.AddFaultBreakpoint("10"); err != nil { // hex 0x10 == ExcPrivileged
		t.Fatalf("AddFaultBreakpoint: %v", err)
	}

	got := c.Engine.FaultBreakpoints()
	if len(got) != 1 || got[0] != cpu.ExcPrivileged {
		t.Fatalf("FaultBreakpoints() = %v, want [%#02x]", got, cpu.ExcPrivileged)
	}

	if err := c.RemoveFaultBreakpoint("10"); err != nil {
		t.Fatalf("RemoveFaultBreakpoint: %v", err)
	}

	if got := c.Engine.FaultBreakpoints(); len(got) != 0 {
		t.Fatalf("FaultBreakpoints() after remove = %v, want empty", got)
	}

	if err := c.AddFaultBreakpoint("10"); err != nil {
		t.Fatalf("AddFaultBreakpoint: %v", err)
	}

	if err := c.AddFaultBreakpoint("14"); err != nil {
		t.Fatalf("AddFaultBreakpoint: %v", err)
	}

	if err := c.ClearAllFaultBreakpoints(); err != nil {
		t.Fatalf("ClearAllFaultBreakpoints: %v", err)
	}

	if got := c.Engine.FaultBreakpoints(); len(got) != 0 {
		t.Fatalf("FaultBreakpoints() after ClearAll = %v, want empty", got)
	}
}

func TestShowBreakpoints_mergesFaultBreakpoints(t *testing.T) {
	c, buf := newTestConsole(t)

	c.AddBreakpoint(0x400)

	if err := c.AddFaultBreakpoint("10"); err != nil {
		t.Fatalf("AddFaultBreakpoint: %v", err)
	}

	buf.Reset()

	if err := c.ShowBreakpoints(); err != nil {
		t.Fatalf("ShowBreakpoints: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "00000400") {
		t.Errorf("output = %q, want the address breakpoint listed", out)
	}

	if !strings.Contains(out, "F Breakpoint on fault 10") {
		t.Errorf("output = %q, want the fault breakpoint listed with an F marker", out)
	}
}

func TestSetFaultHistory(t *testing.T) {
	c, _ := newTestConsole(t)

	if err := c.SetFaultHistory(2); err != nil {
		t.Fatalf("SetFaultHistory: %v", err)
	}
	
	if got := c.Engine.FaultHistorySize(); got != 2 {
		t.Errorf("FaultHistorySize() = %d, want 2", got)
	}
}

func TestShowFault_reportsHistoryAndNoneYet(t *testing.T) {
	c, buf := newTestConsole(t)

	buf.Reset()

	if err := c.ShowFault(); err != nil {
		t.Fatalf("ShowFault: %v", err)
	}

	if !strings.Contains(buf.String(), "No exceptions or interrupts have occurred yet") {
		t.Errorf("output = %q, want the no-history message", buf.String())
	}
}

// TestDispatch_setBreakpointFaultInterceptsExecution drives a fault-kind
// breakpoint end to end through the real Dispatcher: an undefined extended
// opcode (0xFD 0x00, the same fixture internal/cpu/decode_test.go's own
// TestDecodeInstructionUndefinedExtendedOpcodeFaults uses) raises
// ExcPrivileged (0x10) at decode time. With that code armed via SET
// BREAKPOINT/FAULT, GO must stop with PC left exactly at the faulting
// instruction (no vector taken -- so no SCB/handler setup is needed for
// this test at all, unlike a real delivery would require), print a
// break message, and still have recorded the fault in SHOW FAULT's
// history.
func TestDispatch_setBreakpointFaultInterceptsExecution(t *testing.T) {
	c, buf := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	loadProgram(t, c, 0x200, 0xFD, 0x00)
	c.CPU.SetGPR(vax.PC, 0x200)

	if err := d.Dispatch("SET BREAKPOINT/FAULT 10"); err != nil {
		t.Fatalf("Dispatch(SET BREAKPOINT/FAULT): %v", err)
	}

	buf.Reset()
	
	if err := d.Dispatch("GO"); err != nil {
		t.Fatalf("Dispatch(GO): %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x200 {
		t.Errorf("PC = %#x, want 0x200 (fault delivery skipped, not the SCB vector)", got)
	}

	if !strings.Contains(buf.String(), "Break on fault") {
		t.Errorf("output = %q, want a fault-break message", buf.String())
	}

	hist := c.Engine.FaultHistory()
	if len(hist) != 1 || hist[0].Code != cpu.ExcPrivileged {
		t.Fatalf("FaultHistory() = %+v, want one ExcPrivileged entry", hist)
	}
}

func TestDispatch_setFaultHistory(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch("SET FAULT 3"); err != nil {
		t.Fatalf("Dispatch(SET FAULT): %v", err)
	}

	if got := c.Engine.FaultHistorySize(); got != 3 {
		t.Errorf("FaultHistorySize() = %d, want 3", got)
	}

	if err := d.Dispatch("SET HISTORY 5"); err != nil {
		t.Fatalf("Dispatch(SET HISTORY): %v", err)
	}

	if got := c.Engine.FaultHistorySize(); got != 5 {
		t.Errorf("FaultHistorySize() = %d, want 5", got)
	}
}

func TestDispatch_clearBreakpointFault(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch("SET BREAKPOINT/FAULT 10"); err != nil {
		t.Fatalf("Dispatch(SET BREAKPOINT/FAULT): %v", err)
	}

	if err := d.Dispatch("SET BREAKPOINT/FAULT 14"); err != nil {
		t.Fatalf("Dispatch(SET BREAKPOINT/FAULT): %v", err)
	}

	if err := d.Dispatch("CLEAR BREAKPOINT/FAULT 10"); err != nil {
		t.Fatalf("Dispatch(CLEAR BREAKPOINT/FAULT): %v", err)
	}

	got := c.Engine.FaultBreakpoints()
	if len(got) != 1 || got[0] != cpu.ExcCustomer {
		t.Fatalf("FaultBreakpoints() = %v, want [%#02x]", got, cpu.ExcCustomer)
	}

	if err := d.Dispatch("CLEAR BREAKPOINT/FAULT/ALL"); err != nil {
		t.Fatalf("Dispatch(CLEAR BREAKPOINT/FAULT/ALL): %v", err)
	}

	if got := c.Engine.FaultBreakpoints(); len(got) != 0 {
		t.Fatalf("FaultBreakpoints() after /ALL = %v, want empty", got)
	}
}
