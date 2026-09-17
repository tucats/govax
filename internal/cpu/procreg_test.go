package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// mtprBytes returns the byte sequence for MTPR src, #reg where src is a
// register-mode operand and reg is a short-literal register number.
func mtprBytes(src vax.Reg, reg uint32) []byte {
	return []byte{0xDA, regMode(src), shortLiteral(reg)}
}

// mfprBytes returns the byte sequence for MFPR #reg, dst.
func mfprBytes(reg uint32, dst vax.Reg) []byte {
	return []byte{0xDB, shortLiteral(reg), regMode(dst)}
}

func kernelEngine() *Engine {
	e := newEngine()
	psl := e.cpu.PSL()
	psl.SetCurMod(vax.Kernel)
	e.cpu.SetPSL(psl)
	
	return e
}

func TestEmulMtprMfprDefaultRegisterRoundTrip(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu

	cpu.SetGPR(vax.R1, 0xDEADBEEF)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.P0BR))...)

	if got := cpu.PR(vax.P0BR); got != 0xDEADBEEF {
		t.Fatalf("PR(P0BR) = %#x, want 0xDEADBEEF", got)
	}

	stepInstruction(t, e, mfprBytes(uint32(vax.P0BR), vax.R2)...)

	if got := cpu.GPR(vax.R2); got != 0xDEADBEEF {
		t.Errorf("R2 after MFPR = %#x, want 0xDEADBEEF", got)
	}
}

// TestEmulMtprRequiresKernelMode calls emulMtpr directly, rather than
// through Engine.Step, so the test doesn't also need a page table for the
// kernel stack: raising the fault from User mode would drive HandleFault's
// User->Kernel mode switch, which (see docs/DEVIATIONS.md on setModeStack)
// sets MAPEN = 1 as a side effect, and this test only cares about MTPR's own
// mode check, already covered end-to-end for other faults below via
// kernelEngine (no actual mode switch, so no page table needed).
func TestEmulMtprRequiresKernelMode(t *testing.T) {
	cpu, mem := fixture()
	psl := cpu.PSL()
	psl.SetCurMod(vax.User)
	cpu.SetPSL(psl)
	cpu.SetGPR(vax.R1, 1)

	d := &Decoded{Operands: [6]Operand{
		{Kind: OperandRegister, Reg: vax.R1, Size: 4},
		{Kind: OperandImmediate, Value: uint64(vax.P0BR), Size: 4},
	}}

	err := emulMtpr(NewEngine(cpu, mem), d)

	var f *Fault

	if !errors.As(err, &f) {
		t.Fatalf("emulMtpr err = %v, want *Fault", err)
	}
	
	if f.Code != ExcPrivileged {
		t.Errorf("fault code = %#x, want ExcPrivileged", f.Code)
	}
}

func TestEmulMtprOutOfRangeRegisterFaults(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	cpu.SetGPR(vax.R1, 1)
	// #200 doesn't fit a short literal (max 63); use an immediate longword
	// instead: I^#200.
	stepInstruction(t, e, 0xDA, regMode(vax.R1), 0x8F, 200, 0, 0, 0)

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

func TestEmulMtprNoAccessRegisterFaults(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	cpu.SetGPR(vax.R1, 1)
	// Register 44 (WCSA) has no defined access at all.
	stepInstruction(t, e, mtprBytes(vax.R1, 44)...)

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

func TestEmulMfprWriteOnlyRegisterFaults(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	// SIRR (20) is write-only.
	stepInstruction(t, e, mfprBytes(uint32(vax.SIRR), vax.R2)...)

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

func TestEmulMtprIPLUpdatesPSLAndTruncates(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu

	cpu.SetGPR(vax.R1, 0xFFFFFFFF) // masked to the low 5 bits (0x1F)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.IPL))...)

	if got := cpu.PR(vax.IPL); got != 0x1F {
		t.Errorf("PR(IPL) = %#x, want 0x1F", got)
	}

	if got := cpu.PSL().IPL(); got != 0x1F {
		t.Errorf("PSL.IPL() = %#x, want 0x1F", got)
	}
}

// TestEmulMtprIPLAdmitsLatchedSoftwareInterruptOnceExposed covers set_priv_reg's
// case 18 SISR scan: a software interrupt latched while IPL was too high to
// take it immediately is admitted once a later MTPR IPL lowers the IPL back
// below it.
func TestEmulMtprIPLAdmitsLatchedSoftwareInterruptOnceExposed(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	psl := cpu.PSL()
	psl.SetIPL(5)
	cpu.SetPSL(psl)

	cpu.SetGPR(vax.R1, 3) // latched: 3 <= current IPL (5)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.SIRR))...)

	if e.interruptPending {
		t.Fatal("expected the SIRR request to be latched, not delivered, while IPL is 5")
	}

	cpu.SetGPR(vax.R1, 0) // lower IPL to 0, exposing the latched level-3 request
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.IPL))...)

	if !e.interruptPending {
		t.Fatal("expected lowering IPL below the latched SISR bit to admit it")
	}

	if e.interruptCode != ExcSoftware1+2*4 || e.interruptIPL != 3 {
		t.Errorf("interruptCode/IPL = %#x/%d, want %#x/3", e.interruptCode, e.interruptIPL, ExcSoftware1+2*4)
	}

	if got := cpu.PR(vax.SISR); got&(1<<3) != 0 {
		t.Errorf("PR(SISR) bit 3 still set after admission, want cleared")
	}
}

func TestEmulMtprAstlvlBoundsCheck(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	cpu.SetGPR(vax.R1, 5) // valid range is 0-4
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.ASTLVL))...)

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

func TestEmulMtprAstlvlValid(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu

	cpu.SetGPR(vax.R1, 4)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.ASTLVL))...)

	if got := cpu.PR(vax.ASTLVL); got != 4 {
		t.Errorf("PR(ASTLVL) = %d, want 4", got)
	}
}

// TestEmulMtprSirrLatchesWhenAtOrAboveCurrentIPL covers set_priv_reg's own
// case 20 "else" branch (value <= current IPL): the request is only latched
// into SISR/SIRR for later, not taken immediately.
func TestEmulMtprSirrLatchesWhenAtOrAboveCurrentIPL(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	psl := cpu.PSL()
	psl.SetIPL(5)
	cpu.SetPSL(psl)

	cpu.SetGPR(vax.R1, 0xF3) // low 4 bits: 3, at/below the current IPL (5)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.SIRR))...)

	if got := cpu.PR(vax.SIRR); got != 3 {
		t.Errorf("PR(SIRR) = %d, want 3", got)
	}

	if got := cpu.PR(vax.SISR); got&(1<<3) == 0 {
		t.Errorf("PR(SISR) = %#x, want bit 3 set", got)
	}

	if e.interruptPending {
		t.Error("expected no immediate delivery when the request is at or below the current IPL")
	}
}

// TestEmulMtprSirrDeliversImmediatelyWhenAboveCurrentIPL covers set_priv_reg's
// own case 20 "if" branch: a request above the current IPL is taken right
// away (via Engine.Interrupt), and SISR/SIRR are left untouched.
func TestEmulMtprSirrDeliversImmediatelyWhenAboveCurrentIPL(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu // current IPL defaults to 0

	cpu.SetGPR(vax.R1, 0xF3) // low 4 bits: 3, above the current IPL (0)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.SIRR))...)

	if !e.interruptPending {
		t.Fatal("expected immediate delivery when the request exceeds the current IPL")
	}

	if e.interruptCode != ExcSoftware1+2*4 || e.interruptIPL != 3 {
		t.Errorf("interruptCode/IPL = %#x/%d, want %#x/3", e.interruptCode, e.interruptIPL, ExcSoftware1+2*4)
	}

	if got := cpu.PR(vax.SISR); got != 0 {
		t.Errorf("PR(SISR) = %#x, want 0 (not latched on the immediate-delivery path)", got)
	}
}

// TestEmulMtprTbiaInvalidatesTB matches emul_procreg.c's own TBIA case
// (invalidate_tb()): a real full flush of the translation buffer, wired up
// as of Phase 21 (see docs/PHASE-21.md) -- TBIA/TBIS are not stored into
// the register array either way (the C source's switch case has no
// fallthrough to its default `vax.preg[reg] = value`), which this test
// keeps checking alongside the real invalidation.
func TestEmulMtprTbiaInvalidatesTB(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu

	_, _, flushesBefore, _ := e.mem.TBStats()

	cpu.SetGPR(vax.R1, 0x12345678)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.TBIA))...)

	if got := cpu.PR(vax.TBIA); got != 0 {
		t.Errorf("PR(TBIA) = %#x, want 0 (not stored, same as the C source)", got)
	}

	_, _, flushesAfter, _ := e.mem.TBStats()
	if flushesAfter != flushesBefore+1 {
		t.Errorf("TBStats() flushes = %d, want %d (MTPR TBIA must call InvalidateTB)", flushesAfter, flushesBefore+1)
	}
}

// TestEmulMtprTbisInvalidatesPage matches emul_procreg.c's own TBIS case
// (invalidate_page(value)): flushes only the one TB slot the given address
// maps to, using the MTPR operand's own value as that address.
func TestEmulMtprTbisInvalidatesPage(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu

	// A minimal S0 identity setup: SBR/SLR covering one page, whose PTE
	// (physical page 1) maps virtual page 0 of S0 onto physical page 2.
	const (
		sbrPhys = 0x1000
		pfn     = 2
	)

	cpu.SetPR(vax.SBR, sbrPhys)
	cpu.SetPR(vax.SLR, 0)

	var pte vm.PTE
	pte.SetValid(true)
	pte.SetProtection(vm.ProtUW)
	pte.SetPFN(pfn)

	// MAPEN is still off here, so this is a direct physical-address write.
	putLongword(t, cpu, e.mem, sbrPhys, uint32(pte))

	cpu.SetPR(vax.MAPEN, 1)

	const vaddr = 0x80000000 // S0 region, page 0

	if _, err := e.mem.Translate(cpu, vaddr, vm.AccessRead); err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(e.mem.TBSnapshot()) == 0 {
		t.Fatalf("TBSnapshot() empty before MTPR TBIS, want the slot populated")
	}

	// MAPEN back off: stepInstruction fetches the MTPR instruction itself
	// from a fixed physical-style test address (base, 0x1000) that has no
	// P0 page table backing it here, and MTPR's own register-mode operand
	// needs no translation regardless.
	cpu.SetPR(vax.MAPEN, 0)

	cpu.SetGPR(vax.R1, vaddr)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.TBIS))...)

	if got := cpu.PR(vax.TBIS); got != 0 {
		t.Errorf("PR(TBIS) = %#x, want 0 (not stored, same as the C source)", got)
	}

	if got := len(e.mem.TBSnapshot()); got != 0 {
		t.Errorf("TBSnapshot() len = %d after MTPR TBIS, want 0 (its own slot invalidated)", got)
	}
}
