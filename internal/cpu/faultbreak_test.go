package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestFaultBreakpointsAddRemoveClear(t *testing.T) {
	e := newEngine()

	e.SetFaultBreakpoint(ExcPrivileged)
	e.SetFaultBreakpoint(ExcCustomer)

	got := e.FaultBreakpoints()
	if len(got) != 2 || got[0] != ExcPrivileged || got[1] != ExcCustomer {
		t.Fatalf("FaultBreakpoints() = %v, want sorted [%#02x, %#02x]", got, ExcPrivileged, ExcCustomer)
	}

	e.RemoveFaultBreakpoint(ExcPrivileged)
	if got := e.FaultBreakpoints(); len(got) != 1 || got[0] != ExcCustomer {
		t.Fatalf("after RemoveFaultBreakpoint: %v, want [%#02x]", got, ExcCustomer)
	}

	// Removing an unarmed code is a no-op, not an error.
	e.RemoveFaultBreakpoint(ExcPrivileged)

	e.ClearFaultBreakpoints()
	if got := e.FaultBreakpoints(); len(got) != 0 {
		t.Fatalf("after ClearFaultBreakpoints: %v, want empty", got)
	}
}

// TestFaultBreakInterceptsRaise checks that raise (the choke point for
// every decode-time or Handler-returned fault) skips HandleFault entirely
// when the fault's code is armed, matching vax.c's own "set
// vax.fault_pending, return VAX_BREAK" behavior -- no vector taken, PC left
// exactly at the faulting instruction's own start address.
func TestFaultBreakInterceptsRaise(t *testing.T) {
	e := newEngine()
	e.instructionPC = 0x4000
	e.cpu.SetGPR(vax.PC, 0x4002) // decode already advanced PC past the faulting instruction
	e.SetFaultBreakpoint(ExcPrivileged)

	err := e.raise(&Fault{Code: ExcPrivileged, Args: []uint32{0xAAAA}})

	var fb *FaultBreak
	if !errors.As(err, &fb) {
		t.Fatalf("raise() = %v, want a *FaultBreak", err)
	}
	if fb.Code != ExcPrivileged {
		t.Errorf("Code = %#02x, want %#02x", fb.Code, ExcPrivileged)
	}
	if got := e.cpu.GPR(vax.PC); got != 0x4000 {
		t.Errorf("PC = %#x, want 0x4000 (delivery skipped, PC left at the fault)", got)
	}

	// The fault must still be recorded in history even though delivery was
	// intercepted -- matching store_fault running inside set_fault, ahead
	// of the BREAK_FAULT check.
	hist := e.FaultHistory()
	if len(hist) != 1 || hist[0].Code != ExcPrivileged || hist[0].PC != 0x4000 {
		t.Errorf("FaultHistory() = %+v, want one ExcPrivileged entry at PC 0x4000", hist)
	}
}

// TestFaultBreakDoesNotInterceptUnarmedCode checks that raise delivers a
// fault normally (reaching HandleFault) when no fault breakpoint matches --
// same fixture shape as TestHandleFaultBasicFrame, but driven through raise
// rather than HandleFault directly.
func TestFaultBreakDoesNotInterceptUnarmedCode(t *testing.T) {
	e := newEngine()
	e.instructionPC = 0x4000
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedAddr, 0x100, 0)
	e.SetFaultBreakpoint(ExcPrivileged) // armed, but for a different code

	err := e.raise(&Fault{Code: ExcReservedAddr})
	if err != nil {
		t.Fatalf("raise() = %v, want nil (real delivery, unarmed code)", err)
	}
	if got := e.cpu.GPR(vax.PC); got != 0x100 {
		t.Errorf("PC = %#x, want 0x100 (the vector, meaning delivery actually ran)", got)
	}
}

// TestFaultBreakInterceptsDeliverPendingInterrupt mirrors
// TestFaultBreakInterceptsRaise for the device/software-interrupt delivery
// path (vax.c's own separate BREAK_FAULT check ahead of its interrupt-
// delivery handle_fault call).
func TestFaultBreakInterceptsDeliverPendingInterrupt(t *testing.T) {
	e := interruptEngine(t)
	e.cpu.SetGPR(vax.PC, 0x5000)
	e.interruptPending = true
	e.interruptCode = ExcConWrite
	e.interruptIPL = 20
	e.SetFaultBreakpoint(ExcConWrite)

	err := e.deliverPendingInterrupt()

	var fb *FaultBreak
	if !errors.As(err, &fb) || fb.Code != ExcConWrite {
		t.Fatalf("deliverPendingInterrupt() = %v, want *FaultBreak{Code: ExcConWrite}", err)
	}
	if got := e.cpu.GPR(vax.PC); got != 0x5000 {
		t.Errorf("PC = %#x, want 0x5000 (delivery skipped)", got)
	}
}
