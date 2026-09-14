package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
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

func TestEmulMtprSirrQueuesSoftwareInterrupt(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu

	cpu.SetGPR(vax.R1, 0xF3) // low 4 bits: 3
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.SIRR))...)

	if got := cpu.PR(vax.SIRR); got != 3 {
		t.Errorf("PR(SIRR) = %d, want 3", got)
	}
	if got := cpu.PR(vax.SISR); got&(1<<3) == 0 {
		t.Errorf("PR(SISR) = %#x, want bit 3 set", got)
	}
}

func TestEmulMtprTbiaTbisAreNoOps(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu

	cpu.SetGPR(vax.R1, 0x12345678)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.TBIA))...)
	if got := cpu.PR(vax.TBIA); got != 0 {
		t.Errorf("PR(TBIA) = %#x, want 0 (no-op, not stored)", got)
	}

	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.TBIS))...)
	if got := cpu.PR(vax.TBIS); got != 0 {
		t.Errorf("PR(TBIS) = %#x, want 0 (no-op, not stored)", got)
	}
}
