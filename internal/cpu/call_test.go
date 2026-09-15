package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// shortLiteral returns the addressing-mode byte for S^#n, n in 0-63.
func shortLiteral(n uint32) byte { return byte(n) }

func mustLongword(t *testing.T, cpu *vax.CPU, mem *vm.Memory, addr uint32) uint32 {
	t.Helper()
	v, err := mem.LoadLongword(cpu, addr)
	if err != nil {
		t.Fatalf("LoadLongword(%#x): %v", addr, err)
	}
	return v
}

// TestEmulCallsFrameLayout hand-verifies CALLS's stack-frame construction
// against a fully worked-out address trace (see docs/PHASE-07.md's progress
// log for the arithmetic), including a deliberately misaligned initial SP to
// exercise the frame's SPA (saved-SP-alignment) bits.
func TestEmulCallsFrameLayout(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	const sp0 = 0x9003
	const fp0 = 0x1234
	const ap0 = 0x5678
	cpu.SetGPR(vax.SP, sp0)
	cpu.SetGPR(vax.FP, fp0)
	cpu.SetGPR(vax.AP, ap0)
	cpu.SetGPR(vax.R2, 0x11111111)
	cpu.SetGPR(vax.R3, 0x22222222)

	// Entry mask at 0x2000: bits 2 and 3 set (save R2, R3).
	putBytes(t, cpu, mem, 0x2000, 0x0C, 0x00)

	// CALLS #3, @#0x2000
	stepInstruction(t, e, 0xFB, shortLiteral(3), 0x9F, 0x00, 0x20, 0x00, 0x00)

	const wantSP = 0x8FE0
	const wantAP = 0x8FFF // address of the pushed count longword
	const wantFP = 0x8FE0
	const wantPC = 0x2002         // entry point + 2 (past the entry mask)
	const wantReturnAddr = 0x1007 // instruction is 7 bytes, starting at base

	if got := cpu.GPR(vax.SP); got != wantSP {
		t.Errorf("SP = %#x, want %#x", got, wantSP)
	}
	if got := cpu.GPR(vax.AP); got != wantAP {
		t.Errorf("AP = %#x, want %#x", got, wantAP)
	}
	if got := cpu.GPR(vax.FP); got != wantFP {
		t.Errorf("FP = %#x, want %#x", got, wantFP)
	}
	if got := cpu.GPR(vax.PC); got != wantPC {
		t.Errorf("PC = %#x, want %#x", got, wantPC)
	}

	if got := mustLongword(t, cpu, mem, 0x8FE0); got != 0 {
		t.Errorf("condition-handler placeholder at FP = %#x, want 0", got)
	}
	const wantMask = 0xE00C0000 // spa=3, calltype=1 (CALLS), mask=0x00C, psw=0
	if got := mustLongword(t, cpu, mem, 0x8FE4); got != wantMask {
		t.Errorf("mask longword at FP+4 = %#x, want %#x", got, wantMask)
	}
	if got := mustLongword(t, cpu, mem, 0x8FE8); got != ap0 {
		t.Errorf("saved AP at FP+8 = %#x, want %#x", got, ap0)
	}
	if got := mustLongword(t, cpu, mem, 0x8FEC); got != fp0 {
		t.Errorf("saved FP at FP+12 = %#x, want %#x", got, fp0)
	}
	if got := mustLongword(t, cpu, mem, 0x8FF0); got != wantReturnAddr {
		t.Errorf("saved PC at FP+16 = %#x, want %#x", got, wantReturnAddr)
	}
	if got := mustLongword(t, cpu, mem, 0x8FF4); got != 0x11111111 {
		t.Errorf("saved R2 at FP+20 = %#x, want 0x11111111", got)
	}
	if got := mustLongword(t, cpu, mem, 0x8FF8); got != 0x22222222 {
		t.Errorf("saved R3 at FP+24 = %#x, want 0x22222222", got)
	}
	if got := mustLongword(t, cpu, mem, 0x8FFF); got != 3 {
		t.Errorf("pushed count at AP = %#x, want 3", got)
	}

	if psl := cpu.PSL(); psl.N() || psl.Z() || psl.V() || psl.C() {
		t.Error("emul_call.c never clears condition codes on CALLS/CALLG " +
			"(see docs/DEVIATIONS.md); expected them unchanged (all clear " +
			"from the zero-value CPU), not explicitly cleared")
	}
}

// TestEmulCallsRoundTrip calls with a zero-argument frame (so RET's SP
// restore lands exactly back where it started) and checks that CALLS/RET
// round-trip every register CALLS touched.
func TestEmulCallsRoundTrip(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	const sp0 = 0x9000
	const fp0 = 0x1234
	const ap0 = 0x5678
	cpu.SetGPR(vax.SP, sp0)
	cpu.SetGPR(vax.FP, fp0)
	cpu.SetGPR(vax.AP, ap0)
	cpu.SetGPR(vax.R2, 0x11111111)
	cpu.SetGPR(vax.R3, 0x22222222)

	// Entry mask at 0x2000: bits 2 and 3 set (save R2, R3), immediately
	// followed by a RET instruction at the procedure's entry point (0x2002).
	putBytes(t, cpu, mem, 0x2000, 0x0C, 0x00, 0x04)

	// CALLS #0, @#0x2000
	stepInstruction(t, e, 0xFB, shortLiteral(0), 0x9F, 0x00, 0x20, 0x00, 0x00)

	if got := cpu.GPR(vax.PC); got != 0x2002 {
		t.Fatalf("PC after CALLS = %#x, want 0x2002", got)
	}

	// R2/R3 were clobbered by the entry mask save; corrupt them further so a
	// bug in RET's register restore would be visible.
	cpu.SetGPR(vax.R2, 0)
	cpu.SetGPR(vax.R3, 0)

	// Step onto the RET at 0x2002.
	if err := e.Step(); err != nil {
		t.Fatalf("Step (RET): %v", err)
	}

	if got := cpu.GPR(vax.SP); got != sp0 {
		t.Errorf("SP after RET = %#x, want %#x", got, sp0)
	}
	if got := cpu.GPR(vax.FP); got != fp0 {
		t.Errorf("FP after RET = %#x, want %#x", got, fp0)
	}
	if got := cpu.GPR(vax.AP); got != ap0 {
		t.Errorf("AP after RET = %#x, want %#x", got, ap0)
	}
	if got := cpu.GPR(vax.PC); got != 0x1007 {
		t.Errorf("PC after RET = %#x, want 0x1007 (the CALLS return address)", got)
	}
	if got := cpu.GPR(vax.R2); got != 0x11111111 {
		t.Errorf("R2 after RET = %#x, want 0x11111111", got)
	}
	if got := cpu.GPR(vax.R3); got != 0x22222222 {
		t.Errorf("R3 after RET = %#x, want 0x22222222", got)
	}
}

// TestEngineCallEntryRunsUntilSentinelReturn exercises the Console-facing
// CallEntry primitive (docs/PHASE-13.md's Console.Call, built on this): a
// zero-argument call built directly (no CALLS instruction), stepped through
// a procedure body that itself CALLS/RETs a nested routine before returning,
// ending in ErrConsoleCallReturned rather than a fault at the sentinel PC.
func TestEngineCallEntryRunsUntilSentinelReturn(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	cpu.SetGPR(vax.SP, 0x9000)
	cpu.SetGPR(vax.FP, 0x1234)
	cpu.SetGPR(vax.AP, 0x5678)

	// Outer procedure at 0x2000: empty entry mask, CALLS a nested procedure
	// at 0x3000, then RET.
	putBytes(t, cpu, mem, 0x2000,
		0x00, 0x00, // entry mask: no registers saved
		0xFB, shortLiteral(0), 0x9F, 0x00, 0x30, 0x00, 0x00, // CALLS #0, @#0x3000
		0x04, // RET
	)
	// Nested procedure at 0x3000: empty entry mask, RET immediately.
	putBytes(t, cpu, mem, 0x3000, 0x00, 0x00, 0x04)

	if err := e.CallEntry(0x2000); err != nil {
		t.Fatalf("CallEntry: %v", err)
	}
	if got := cpu.GPR(vax.PC); got != 0x2002 {
		t.Fatalf("PC after CallEntry = %#x, want 0x2002", got)
	}

	steps := 0
	for {
		steps++
		if steps > 10 {
			t.Fatal("too many steps without reaching ErrConsoleCallReturned")
		}
		err := e.Step()
		if err == nil {
			continue
		}
		if errors.Is(err, ErrConsoleCallReturned) {
			break
		}
		t.Fatalf("Step: %v", err)
	}

	// SP/AP round-trip back to what they were before CallEntry, exactly as a
	// real caller's RET would restore them. PC/FP are left at SentinelReturn
	// -- there is no real caller context to restore them to, since the
	// "caller" here was never a VAX procedure -- which is exactly what tells
	// Console.Call the outermost call is done.
	if got := cpu.GPR(vax.SP); got != 0x9000 {
		t.Errorf("SP after return = %#x, want 0x9000", got)
	}
	if got := cpu.GPR(vax.AP); got != 0x5678 {
		t.Errorf("AP after return = %#x, want 0x5678", got)
	}
	if got := cpu.GPR(vax.FP); got != SentinelReturn {
		t.Errorf("FP after return = %#x, want SentinelReturn", got)
	}
	if got := cpu.GPR(vax.PC); got != SentinelReturn {
		t.Errorf("PC after return = %#x, want SentinelReturn", got)
	}
}

// TestEmulCallgArglistIsOperandAddress checks CALLG's AP is set to its
// arglist operand's address directly (never dereferenced), per the manual's
// "The AP is replaced by the arglist operand."
func TestEmulCallgArglistIsOperandAddress(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	cpu.SetGPR(vax.SP, 0x9000)
	putBytes(t, cpu, mem, 0x2000, 0x00, 0x00)             // empty entry mask
	putBytes(t, cpu, mem, 0x3000, 0xAA, 0xBB, 0xCC, 0xDD) // arglist contents (never read)

	// CALLG @#0x3000, @#0x2000
	stepInstruction(t, e, 0xFA,
		0x9F, 0x00, 0x30, 0x00, 0x00, // @#0x3000
		0x9F, 0x00, 0x20, 0x00, 0x00, // @#0x2000
	)

	if got := cpu.GPR(vax.AP); got != 0x3000 {
		t.Errorf("AP = %#x, want 0x3000 (the arglist operand's address)", got)
	}
	if got := cpu.GPR(vax.PC); got != 0x2002 {
		t.Errorf("PC = %#x, want 0x2002", got)
	}
}

// TestEmulCallgRegisterArglistFaults confirms that a register-mode arglist
// operand faults at decode time (internal/cpu/operand.go's OP_AD-register
// fix), rather than emul_call.c's own is_register[0] special case (which
// would use the register's value as the arglist address -- see call.go's
// doc comment on why that's not replicated).
func TestEmulCallgRegisterArglistFaults(t *testing.T) {
	e := newEngine()
	cpu := e.cpu

	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	cpu.SetGPR(vax.R1, 0x3000)
	putVector(t, e, ExcReservedAddr, 0x300, 0)

	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, e.mem, base, 0xFA, regMode(vax.R1), 0x9F, 0x00, 0x20, 0x00, 0x00)
	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v (fault should be handled, not propagated)", err)
	}
	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-addressing-mode fault vector)", got)
	}
}

// TestEmulCallReservedEntryMaskBitsFault checks the fix for Note 1 of the
// manual's CALLS/CALLG description: bits 13:12 of the entry mask must be
// zero, or a reserved-operand fault occurs. emul_call.c never checks this;
// fixed here per the user's direction (2026-09-14) rather than only logged
// in docs/DEVIATIONS.md.
func TestEmulCallReservedEntryMaskBitsFault(t *testing.T) {
	e := newEngine()
	cpu := e.cpu

	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	// Entry mask with bit 12 set (reserved) at 0x2000.
	putBytes(t, cpu, e.mem, 0x2000, 0x00, 0x10)

	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, e.mem, base, 0xFB, shortLiteral(0), 0x9F, 0x00, 0x20, 0x00, 0x00)
	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v (fault should be handled, not propagated)", err)
	}
	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

// TestEmulCallClearsConditionCodesAndSetsTrapEnables checks the fix for the
// PSW effects the manual documents for CALLS/CALLG but emul_call.c never
// implements: condition codes cleared, IV/DV set from entry-mask bits 14/15,
// FU cleared. Fixed here per the user's direction (2026-09-14).
func TestEmulCallClearsConditionCodesAndSetsTrapEnables(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.SP, 0x9000)

	psl := cpu.PSL()
	psl.SetNZVC(true, true, true, true)
	psl.SetFU(true)
	psl.SetDV(true) // should be cleared: entry mask's bit 15 (DV) is 0 below
	cpu.SetPSL(psl)

	// Entry mask with bit 14 set (IV enable), bit 15 clear (DV disable), no
	// registers to save.
	putBytes(t, cpu, mem, 0x2000, 0x00, 0x40)

	stepInstruction(t, e, 0xFA, 0x9F, 0x00, 0x30, 0x00, 0x00, 0x9F, 0x00, 0x20, 0x00, 0x00)

	got := cpu.PSL()
	if got.N() || got.Z() || got.V() || got.C() {
		t.Errorf("condition codes = N=%v Z=%v V=%v C=%v, want all clear", got.N(), got.Z(), got.V(), got.C())
	}
	if !got.IV() {
		t.Error("IV = false, want true (entry mask bit 14 was set)")
	}
	if got.DV() {
		t.Error("DV = true, want false (entry mask bit 15 was clear)")
	}
	if got.FU() {
		t.Error("FU = true, want false (CALLS/CALLG always clears it)")
	}
}

// TestEmulRetReservedOperandFault checks the fix for RET's Note 1: a
// reserved operand fault occurs if the popped mask longword's bits 15:8 are
// nonzero. emul_call.c never checks this; fixed here per the user's
// direction (2026-09-14).
func TestEmulRetReservedOperandFault(t *testing.T) {
	e := newEngine()
	cpu := e.cpu

	const fp = 0x8000
	cpu.SetGPR(vax.FP, fp)
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	// A hand-built frame whose mask longword has bit 8 set (part of the
	// otherwise-unused tmp1<15:8> range).
	putLongword(t, cpu, e.mem, fp+4, 0x00000100)
	putLongword(t, cpu, e.mem, fp+8, 0)  // AP
	putLongword(t, cpu, e.mem, fp+12, 0) // FP
	putLongword(t, cpu, e.mem, fp+16, 0) // PC

	stepInstruction(t, e, 0x04) // RET

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

// TestEmulRetRestoresFullPSWFromFrame checks the fix for RET's "PSW <-
// tmp1<15:0>" full-replacement behavior -- not the partial bits-6:15-only
// restore emul_call.c's emul_ret implements -- by round-tripping a CALLG
// whose entry mask sets IV, then confirming RET restores IV from the frame
// even though the live PSL was changed in between.
func TestEmulRetRestoresFullPSWFromFrame(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.SP, 0x9000)

	// Entry mask: no saved registers, IV set (bit 14), followed immediately
	// by a RET at the entry point.
	putBytes(t, cpu, mem, 0x2000, 0x00, 0x40, 0x04)

	stepInstruction(t, e, 0xFA, 0x9F, 0x00, 0x30, 0x00, 0x00, 0x9F, 0x00, 0x20, 0x00, 0x00)
	if !cpu.PSL().IV() {
		t.Fatal("IV = false after CALLG, want true")
	}

	// Tamper with the live PSL so a RET that merely preserved live state
	// (rather than reading the frame) would pass IV through unchanged.
	psl := cpu.PSL()
	psl.SetIV(false)
	psl.SetN(true)
	cpu.SetPSL(psl)

	if err := e.Step(); err != nil { // RET
		t.Fatalf("Step (RET): %v", err)
	}

	got := cpu.PSL()
	if !got.IV() {
		t.Error("IV = false after RET, want true (restored from the frame's saved psw)")
	}
	if got.N() {
		t.Error("N = true after RET, want false (frame's saved condition codes are always clear)")
	}
}

// TestEmulReiReversesModeSwitch hand-builds a return frame as it would sit
// on the kernel stack after an exception delivered from User mode (rather
// than going through Engine.HandleFault, which would also need a page table
// for the kernel stack once setModeStack's mode switch turns MAPEN on -- see
// docs/DEVIATIONS.md on that MAPEN side effect; this test targets REI's own
// restore mechanics, already covered on the delivery side by
// TestHandleFaultBasicFrame/TestHandleFaultModeSwitch) and checks that REI
// restores PC, the full PSL (including CurMod), and both mode stack
// pointers.
func TestEmulReiReversesModeSwitch(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	const userSP = 0x8000
	const kernelFrame = 0x6050
	const faultingPC = 0x5000

	psl := cpu.PSL()
	psl.SetCurMod(vax.Kernel) // as if already running in a fault handler
	cpu.SetPSL(psl)
	cpu.SetGPR(vax.SP, kernelFrame)
	cpu.SetPR(vax.USP, userSP)

	// The frame a real fault delivery would have left: saved PC, then saved
	// PSL (CurMod=User, N set) -- see emul_rei.c's own read order.
	putLongword(t, cpu, mem, kernelFrame, faultingPC)
	restoredPSL := psl
	restoredPSL.SetCurMod(vax.User)
	restoredPSL.SetN(true)
	putLongword(t, cpu, mem, kernelFrame+4, uint32(restoredPSL))

	stepInstruction(t, e, 0x02) // REI

	if got := cpu.GPR(vax.PC); got != faultingPC {
		t.Errorf("PC after REI = %#x, want %#x", got, faultingPC)
	}
	if got := cpu.PSL().CurMod(); got != vax.User {
		t.Errorf("CurMod after REI = %v, want User", got)
	}
	if !cpu.PSL().N() {
		t.Error("N after REI = false, want true (restored from the popped PSL)")
	}
	if got := cpu.GPR(vax.SP); got != userSP {
		t.Errorf("SP after REI = %#x, want %#x (USP)", got, userSP)
	}
	if got := cpu.PR(vax.KSP); got != kernelFrame+8 {
		t.Errorf("KSP after REI = %#x, want %#x (old SP, past the popped frame)", got, kernelFrame+8)
	}
}
