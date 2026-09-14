package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulCvtNoOverflow(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xFF) // byte: -1
	setC(cpu, false)

	stepInstruction(t, e, 0x98, regMode(vax.R1), regMode(vax.R2)) // CVTBL

	if got := cpu.GPR(vax.R2); got != 0xFFFFFFFF {
		t.Errorf("R2 = %#x, want 0xffffffff (sign-extended)", got)
	}
	psl := cpu.PSL()
	if !psl.N() || psl.Z() || psl.V() || psl.C() {
		t.Errorf("N=%v Z=%v V=%v C=%v, want N=true Z=false V=false C=false", psl.N(), psl.Z(), psl.V(), psl.C())
	}
}

func TestEmulCvtOverflowNZFromDestination(t *testing.T) {
	// emul_integer_cvt.c computes N/Z from the pre-truncation source value;
	// the manual specifies the post-truncation destination value. These
	// disagree exactly when truncation flips the sign, as here: 128 (long,
	// positive) truncates to byte 0x80 (-128, negative).
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x00000080) // long: 128
	setC(cpu, true)

	stepInstruction(t, e, 0xF6, regMode(vax.R1), regMode(vax.R2)) // CVTLB

	if got := byte(cpu.GPR(vax.R2)); got != 0x80 {
		t.Errorf("result = %#x, want 0x80 (truncated)", got)
	}
	psl := cpu.PSL()
	if !psl.N() {
		t.Error("N = false, want true (destination value 0x80 is negative, even though the source 128 was positive)")
	}
	if psl.Z() {
		t.Error("Z = true, want false")
	}
	if !psl.V() {
		t.Error("V = false, want true (128 doesn't fit in a signed byte)")
	}
	if psl.C() {
		t.Error("C = true, want false (CVT always clears C)")
	}
}

func TestEmulCvtByteOverflowRangeCheck(t *testing.T) {
	// emul_integer_cvt.c's byte-destination V check uses the same wrong
	// constants (255/-256) already found in emul_increment.c/
	// emul_integer_math.c; 300 doesn't fit a signed byte (-128..127) even
	// though it's well within [-256, 255].
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 300) // word

	stepInstruction(t, e, 0x33, regMode(vax.R1), regMode(vax.R2)) // CVTWB

	if !cpu.PSL().V() {
		t.Error("V = false, want true (300 doesn't fit in a signed byte)")
	}
	if got := byte(cpu.GPR(vax.R2)); got != 44 { // 300 mod 256
		t.Errorf("result = %d, want 44", got)
	}
}

func TestEmulCvtNoOverflowPositive(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 100) // byte

	stepInstruction(t, e, 0x99, regMode(vax.R1), regMode(vax.R2)) // CVTBW

	if got := uint16(cpu.GPR(vax.R2)); got != 100 {
		t.Errorf("result = %d, want 100", got)
	}
	psl := cpu.PSL()
	if psl.N() || psl.Z() || psl.V() {
		t.Errorf("N=%v Z=%v V=%v, want all false", psl.N(), psl.Z(), psl.V())
	}
}
