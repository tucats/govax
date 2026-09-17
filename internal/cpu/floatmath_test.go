package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// setFloatReg stores value as size-byte VAX F/D-floating bits into cpu's
// register r (and r+1 for size 8), via the already-verified fpuStore
// (fpu_test.go) -- this file's tests focus on operand order/condition
// codes/opcode wiring, not re-deriving raw bit patterns by hand.
func setFloatReg(t *testing.T, cpu *vax.CPU, size int, r vax.Reg, value float64) {
	t.Helper()

	raw, err := fpuStore(cpu, size, value)
	if err != nil {
		t.Fatalf("fpuStore(%v): %v", value, err)
	}

	cpu.SetGPR(r, uint32(raw))

	if size == 8 {
		cpu.SetGPR(r+1, uint32(raw>>32))
	}
}

// getFloatReg is setFloatReg's inverse, via the already-verified fpuLoad.
func getFloatReg(t *testing.T, cpu *vax.CPU, size int, r vax.Reg) float64 {
	t.Helper()
	
	raw := uint64(cpu.GPR(r))
	if size == 8 {
		raw |= uint64(cpu.GPR(r+1)) << 32
	}

	v, err := fpuLoad(raw, size)
	if err != nil {
		t.Fatalf("fpuLoad: %v", err)
	}

	return v
}

func TestEmulFAdd(t *testing.T) {
	for _, size := range []int{4, 8} {
		opcode := byte(0x40) // ADDF2
		if size == 8 {
			opcode = 0x60 // ADDD2
		}

		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		// R1:R2 and R3:R4 -- non-overlapping register pairs, needed for the
		// size-8 (D_floating) case since a register operand there occupies
		// two consecutive registers.
		setFloatReg(t, cpu, size, vax.R1, 2.5)
		setFloatReg(t, cpu, size, vax.R3, 4.0)

		stepInstruction(t, e, opcode, regMode(vax.R1), regMode(vax.R3))

		if got := getFloatReg(t, cpu, size, vax.R3); got != 6.5 {
			t.Errorf("size %d: R3 = %v, want 6.5", size, got)
		}

		psl := cpu.PSL()
		if psl.N() || psl.Z() || psl.V() || psl.C() {
			t.Errorf("size %d: N=%v Z=%v V=%v C=%v, want all false", size, psl.N(), psl.Z(), psl.V(), psl.C())
		}
	}
}

func TestEmulFAdd3(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	setFloatReg(t, cpu, 4, vax.R1, 2.5)
	setFloatReg(t, cpu, 4, vax.R2, 4.0)

	stepInstruction(t, e, 0x41, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3)) // ADDF3

	if got := getFloatReg(t, cpu, 4, vax.R3); got != 6.5 {
		t.Errorf("R3 = %v, want 6.5", got)
	}
}

func TestEmulFSubOperandOrder(t *testing.T) {
	// SUBF2 sub,dif: dif <- dif - sub -- op0 is the subtrahend, op1 the
	// minuend, for both the 2- and 3-operand forms, matching the integer
	// SUB family's convention.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	setFloatReg(t, cpu, 4, vax.R1, 1.5) // subtrahend
	setFloatReg(t, cpu, 4, vax.R2, 4.0) // minuend

	stepInstruction(t, e, 0x42, regMode(vax.R1), regMode(vax.R2)) // SUBF2

	if got := getFloatReg(t, cpu, 4, vax.R2); got != 2.5 {
		t.Errorf("R2 = %v, want 2.5 (4.0 - 1.5)", got)
	}
}

func TestEmulFMul(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	setFloatReg(t, cpu, 8, vax.R1, 2.5)
	setFloatReg(t, cpu, 8, vax.R3, -2.0) // R3:R4 pair (avoid overlapping R1:R2)

	stepInstruction(t, e, 0x64, regMode(vax.R1), regMode(vax.R3)) // MULD2

	if got := getFloatReg(t, cpu, 8, vax.R3); got != -5.0 {
		t.Errorf("R3:R4 = %v, want -5.0", got)
	}

	psl := cpu.PSL()
	if !psl.N() {
		t.Error("N = false, want true (-5.0 is negative)")
	}
}

func TestEmulFDivOperandOrder(t *testing.T) {
	// DIVF2 div,quo: quo <- quo / div -- op0 is the divisor.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	setFloatReg(t, cpu, 4, vax.R1, 2.0)  // divisor
	setFloatReg(t, cpu, 4, vax.R2, 10.0) // dividend

	stepInstruction(t, e, 0x46, regMode(vax.R1), regMode(vax.R2)) // DIVF2

	if got := getFloatReg(t, cpu, 4, vax.R2); got != 5.0 {
		t.Errorf("R2 = %v, want 5.0 (10.0 / 2.0)", got)
	}
}

func TestEmulFAddZeroSetsZ(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	setFloatReg(t, cpu, 4, vax.R1, 3.0)
	setFloatReg(t, cpu, 4, vax.R2, -3.0)
	setC(cpu, true) // confirm C is cleared, not left stale

	stepInstruction(t, e, 0x40, regMode(vax.R1), regMode(vax.R2)) // ADDF2

	if got := getFloatReg(t, cpu, 4, vax.R2); got != 0.0 {
		t.Errorf("R2 = %v, want 0.0", got)
	}

	psl := cpu.PSL()
	if !psl.Z() || psl.N() || psl.V() || psl.C() {
		t.Errorf("N=%v Z=%v V=%v C=%v, want Z=true, rest false", psl.N(), psl.Z(), psl.V(), psl.C())
	}
}

func TestEmulFAddOverflowFaults(t *testing.T) {
	// Calls the handler directly rather than through Engine.Step: a real
	// end-to-end fault delivery needs a System Control Block set up in
	// memory (Engine.raise/HandleFault), which is Phase 07/08 territory and
	// not what this test is about -- see control_test.go's identical
	// reasoning for emulHalt.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	setFloatReg(t, cpu, 4, vax.R1, 1.6e38)
	setFloatReg(t, cpu, 4, vax.R2, 1.6e38)
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x40, regMode(vax.R1), regMode(vax.R2)) // ADDF2

	d, err := decodeInstruction(cpu, mem, instructionTable)
	if err != nil {
		t.Fatalf("decodeInstruction: %v", err)
	}

	err = emulFAdd(e, &d)

	var f *Fault

	if !errors.As(err, &f) || f.Code != ExcArithmetic {
		t.Fatalf("emulFAdd() = %v, want *Fault{Code: ExcArithmetic} (result exceeds F_floating range)", err)
	}
}

func TestEmulFAddShortLiteralSource(t *testing.T) {
	// A short-literal float source (mode 0-3) arrives pre-decoded as IEEE
	// double bits, not VAX F_floating bits -- see fpu.go's loadFloat. 0.5 is
	// exactly representable in shortDouble[64].
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	
	setFloatReg(t, cpu, 4, vax.R2, 1.0)

	stepInstruction(t, e, 0x40, 0x00, regMode(vax.R2)) // ADDF2 S^#0.5, R2

	if got := getFloatReg(t, cpu, 4, vax.R2); got != 1.5 {
		t.Errorf("R2 = %v, want 1.5 (1.0 + short-literal 0.5)", got)
	}
}
