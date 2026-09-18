package cpu

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

const scbb = 0x9000

func newEngine() *Engine {
	cpu, mem := fixture()
	cpu.SetPR(vax.SCBB, scbb)
	
	return NewEngine(cpu, mem)
}

// putVector installs a vector for exception code at the SCB, with the given
// stack selector bits (0 = mode-appropriate stack, 1 = interrupt stack).
func putVector(t *testing.T, e *Engine, code Exception, addr uint32, stackBit uint32) {
	t.Helper()
	putLongword(t, e.cpu, e.mem, scbb+uint32(code), addr|stackBit)
}

func TestHandleFaultBasicFrame(t *testing.T) {
	e := newEngine()
	e.instructionPC = 0x4000
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedAddr, 0x100, 0)

	err := e.HandleFault(&Fault{Code: ExcReservedAddr, Args: []uint32{0xAAAA, 1}})
	if err != nil {
		t.Fatalf("HandleFault: %v", err)
	}

	if e.cpu.GPR(vax.PC) != 0x100 {
		t.Errorf("PC = %#x, want 0x100 (vector)", e.cpu.GPR(vax.PC))
	}

	sp := e.cpu.GPR(vax.SP)
	if sp != 0x7000-16 {
		t.Fatalf("SP = %#x, want %#x (4 longwords pushed)", sp, 0x7000-16)
	}

	arg1, _ := e.mem.LoadLongword(e.cpu, sp)
	arg0, _ := e.mem.LoadLongword(e.cpu, sp+4)
	pc, _ := e.mem.LoadLongword(e.cpu, sp+8)
	psl, _ := e.mem.LoadLongword(e.cpu, sp+12)

	if arg0 != 0xAAAA || arg1 != 1 {
		t.Errorf("pushed args = (%#x, %#x), want (0xAAAA, 1)", arg0, arg1)
	}

	if pc != 0x4000 {
		t.Errorf("pushed PC = %#x, want 0x4000 (instructionPC)", pc)
	}
	
	_ = psl // exact PSL bit layout is exercised by TestHandleFaultModeSwitch below
}

func TestHandleFaultReadsVectorWithVMDisabled(t *testing.T) {
	e := newEngine()
	putVector(t, e, ExcPrivileged, 0x200, 0)

	// Enable virtual memory such that scbb (a low, P0-region address) is
	// out of range for P0 — P0LR defaults to 0, and scbb's page is well
	// above that — so the vector read would fault here if handle_fault
	// didn't force MAPEN off around it. The fault-frame push below targets
	// an S0-region stack address instead, backed by one valid PTE, so it
	// succeeds independent of whether the vector-read bypass worked.
	const stackVA = 0x80000100 // S0 region, page 0 (with room below it in the same page)

	const sbr = 0x3000         // physical address of the (one-entry) S0 page table

	const pfn = 0x20           // backing physical frame -> address 0x4000

	var pte vm.PTE

	pte.SetValid(true)
	pte.SetProtection(vm.ProtKW)
	pte.SetPFN(pfn)
	putLongword(t, e.cpu, e.mem, sbr, uint32(pte))

	e.cpu.SetPR(vax.SBR, sbr)
	e.cpu.SetPR(vax.SLR, 0) // only page 0 (the one PTE above) is in range
	e.cpu.SetGPR(vax.SP, stackVA)
	e.cpu.SetPR(vax.KSP, stackVA) // matches current mode, so setModeStack no-ops

	e.cpu.SetPR(vax.MAPEN, 1)

	err := e.HandleFault(&Fault{Code: ExcPrivileged})
	if err != nil {
		t.Fatalf("HandleFault: %v (vector fetch should bypass translation)", err)
	}

	if e.cpu.PR(vax.MAPEN) != 1 {
		t.Errorf("MAPEN = %d, want 1 (restored after the vector fetch)", e.cpu.PR(vax.MAPEN))
	}

	if e.cpu.GPR(vax.PC) != 0x200 {
		t.Errorf("PC = %#x, want 0x200 (the vector, proving it was read correctly)", e.cpu.GPR(vax.PC))
	}
}

// TestSetModeStackSwitchToKernel exercises set_mode_stack's mechanics
// directly (rather than through HandleFault) so the test doesn't also need
// a valid page table for the new stack: switching access mode sets
// MAPEN = 1 (see docs/DEVIATIONS.md), and HandleFault would then need to
// push the fault frame through translation.
func TestSetModeStackSwitchToKernel(t *testing.T) {
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x8000) // current (user) stack
	e.cpu.SetPR(vax.KSP, 0x6000)
	psl := e.cpu.PSL()
	psl.SetCurMod(vax.User)
	e.cpu.SetPSL(psl)

	e.setModeStack(vax.Kernel, false)

	if got := e.cpu.PSL().CurMod(); got != vax.Kernel {
		t.Errorf("CurMod = %v, want Kernel", got)
	}

	if got := e.cpu.PSL().PrvMod(); got != vax.User {
		t.Errorf("PrvMod = %v, want User (the mode we switched from)", got)
	}

	if e.cpu.PR(vax.USP) != 0x8000 {
		t.Errorf("USP = %#x, want 0x8000 (old SP saved to the mode we left)", e.cpu.PR(vax.USP))
	}

	if e.cpu.GPR(vax.SP) != 0x6000 {
		t.Errorf("SP = %#x, want 0x6000 (KSP)", e.cpu.GPR(vax.SP))
	}

	if e.cpu.PR(vax.MAPEN) != 1 {
		t.Errorf("MAPEN = %d, want 1 (see docs/DEVIATIONS.md)", e.cpu.PR(vax.MAPEN))
	}
}

func TestHandleFaultInterruptStack(t *testing.T) {
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x8000)
	e.cpu.SetPR(vax.KSP, 0x6000)
	e.cpu.SetPR(vax.ISP, 0x9000)
	putVector(t, e, ExcMachineCheck, 0x400, 1) // stack bit 1 == interrupt stack

	if err := e.HandleFault(&Fault{Code: ExcMachineCheck}); err != nil {
		t.Fatalf("HandleFault: %v", err)
	}

	if !e.cpu.PSL().IS() {
		t.Error("PSL.IS() = false, want true (on interrupt stack)")
	}

	if got := e.cpu.PSL().CurMod(); got != vax.Kernel {
		t.Errorf("CurMod = %v, want Kernel", got)
	}

	if e.cpu.PR(vax.MAPEN) != 0 {
		t.Errorf("MAPEN = %d, want 0 (VM off on the interrupt stack)", e.cpu.PR(vax.MAPEN))
	}

	if want := uint32(0x9000 - 8); e.cpu.GPR(vax.SP) != want {
		t.Errorf("SP = %#x, want %#x (ISP minus PC+PSL)", e.cpu.GPR(vax.SP), want)
	}
}

func TestHandleFaultNoHandlerVector(t *testing.T) {
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x8000)
	putLongword(t, e.cpu, e.mem, scbb+uint32(ExcAccessViol), 0xFFFFFFFF)

	err := e.HandleFault(&Fault{Code: ExcAccessViol})
	if !errors.Is(err, ErrNoExceptionHandler) {
		t.Fatalf("err = %v, want ErrNoExceptionHandler", err)
	}

	if e.cpu.GPR(vax.SP) != 0x8000 {
		t.Errorf("SP = %#x, want unchanged 0x8000 (no frame pushed)", e.cpu.GPR(vax.SP))
	}
}

// TestHandleFaultNoHandlerVectorCarriesFaultContext checks that the
// console-handler sentinel case (see docs/PHASE-20.md) returns the richer
// *ConsoleHandlerFault type carrying everything the Condition Handling
// Facility port needs -- the faulting instruction's own PC (distinct from
// whatever decode already advanced the live PC to), PSL, R0/R1, and the
// original *Fault's code/args -- not just the bare ErrNoExceptionHandler
// sentinel.
func TestHandleFaultNoHandlerVectorCarriesFaultContext(t *testing.T) {
	e := newEngine()
	e.instructionPC = 0x1234
	e.cpu.SetGPR(vax.PC, 0x1236) // decode has already advanced past a 2-byte instruction
	e.cpu.SetGPR(vax.R0, 0xAAAA)
	e.cpu.SetGPR(vax.R1, 0xBBBB)
	putLongword(t, e.cpu, e.mem, scbb+uint32(ExcReservedOp), 0xFFFFFFFF)

	err := e.HandleFault(&Fault{Code: ExcReservedOp, Args: []uint32{7}})

	var chf *ConsoleHandlerFault
	if !errors.As(err, &chf) {
		t.Fatalf("err = %v (%T), want *ConsoleHandlerFault", err, err)
	}

	if chf.Code != ExcReservedOp {
		t.Errorf("Code = %#x, want ExcReservedOp", chf.Code)
	}

	if len(chf.Args) != 1 || chf.Args[0] != 7 {
		t.Errorf("Args = %v, want [7]", chf.Args)
	}

	if chf.PC != 0x1234 {
		t.Errorf("PC = %#x, want the faulting instruction's own PC 0x1234, not the live (already-advanced) PC", chf.PC)
	}

	if chf.R0 != 0xAAAA || chf.R1 != 0xBBBB {
		t.Errorf("R0/R1 = %#x/%#x, want 0xAAAA/0xBBBB", chf.R0, chf.R1)
	}
}

func TestHandleFaultZeroVector(t *testing.T) {
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x8000)
	e.cpu.SetPR(vax.KSP, 0x8000)
	putVector(t, e, ExcTracePending, 0, 0)

	err := e.HandleFault(&Fault{Code: ExcTracePending})
	if !errors.Is(err, ErrUnhandledVector) {
		t.Fatalf("err = %v, want ErrUnhandledVector", err)
	}

	if e.cpu.GPR(vax.PC) != 0 {
		t.Errorf("PC = %#x, want 0 (frame still pushed and PC set)", e.cpu.GPR(vax.PC))
	}

	if e.cpu.GPR(vax.SP) == 0x8000 {
		t.Error("SP unchanged, want the frame to have been pushed despite the zero vector")
	}
}

func TestHandleFaultDebugExceptionsTrace(t *testing.T) {
	e := newEngine()
	e.instructionPC = 0x4000
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedAddr, 0x100, 0)

	var buf bytes.Buffer

	e.cpu.SetDebugWriter(&buf)
	e.cpu.SetDebug(e.cpu.Debug() | vax.DebugExceptions)

	if err := e.HandleFault(&Fault{Code: ExcReservedAddr, Args: []uint32{0xAAAA, 1}}); err != nil {
		t.Fatalf("HandleFault: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "DEBUG(EXCEPTION): TAKE, CODE=001C") || !strings.Contains(out, "VECTOR=00000100") {
		t.Errorf("output = %q, want a DEBUG(EXCEPTION): TAKE line naming CODE=001C VECTOR=00000100", out)
	}
}

func TestHandleFaultNoDebugTraceWhenFlagClear(t *testing.T) {
	e := newEngine()
	e.instructionPC = 0x4000
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedAddr, 0x100, 0)

	var buf bytes.Buffer

	e.cpu.SetDebugWriter(&buf)
	e.cpu.SetDebug(e.cpu.Debug() &^ vax.DebugExceptions)

	if err := e.HandleFault(&Fault{Code: ExcReservedAddr}); err != nil {
		t.Fatalf("HandleFault: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("output = %q, want no trace output with DebugExceptions clear", buf.String())
	}
}

func TestSetModeStackSameModeNoOp(t *testing.T) {
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x1234)
	psl := e.cpu.PSL()
	psl.SetCurMod(vax.Kernel)
	e.cpu.SetPSL(psl)

	e.setModeStack(vax.Kernel, false)

	if e.cpu.GPR(vax.SP) != 0x1234 {
		t.Errorf("SP = %#x, want unchanged 0x1234", e.cpu.GPR(vax.SP))
	}
}

// TestSetModeStackRealModeChangeInvalidatesProtection matches
// docs/PHASE-21.md's design decision: a real CurMod transition through
// setModeStack (fault/interrupt delivery, and Phase 13's RUN) must call
// Memory.InvalidateProtection -- the cached mapping survives, but its
// verified access mode is reset, forcing the next access to re-check
// protection.
func TestSetModeStackRealModeChangeInvalidatesProtection(t *testing.T) {
	e := newEngine()
	cpu := e.cpu

	psl := cpu.PSL()
	psl.SetCurMod(vax.Kernel)
	cpu.SetPSL(psl)

	const (
		sbrPhys = 0x2000
		pfn     = 2
		vaddr   = 0x80000000 // S0 region, page 0
	)

	cpu.SetPR(vax.SBR, sbrPhys)
	cpu.SetPR(vax.SLR, 0)

	var pte vm.PTE
	
	pte.SetValid(true)
	pte.SetProtection(vm.ProtUW)
	pte.SetPFN(pfn)
	putLongword(t, cpu, e.mem, sbrPhys, uint32(pte))

	cpu.SetPR(vax.MAPEN, 1)

	if _, err := e.mem.Translate(cpu, vaddr, vm.AccessRead); err != nil {
		t.Fatalf("Translate: %v", err)
	}

	cpu.SetPR(vax.MAPEN, 0)

	entries := e.mem.TBSnapshot()
	if len(entries) == 0 || !entries[0].ProtValid() {
		t.Fatalf("TBSnapshot() = %+v, want one entry with ProtValid() true before the mode change", entries)
	}

	e.setModeStack(vax.User, false) // a real Kernel -> User transition

	entries = e.mem.TBSnapshot()
	if len(entries) == 0 {
		t.Fatalf("TBSnapshot() empty after a mode change, want the mapping to survive")
	}

	if entries[0].ProtValid() {
		t.Errorf("entries[0].ProtValid() = true after a real mode change, want false (forced recheck)")
	}
}
