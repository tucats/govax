package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// immediate returns an immediate mode (I^#) operand specifier: the 0x8F
// mode byte, then data.
func immediate(data ...byte) []byte {
	return append([]byte{0x8F}, data...)
}

// TestImmediateQuadword: MOVQ reads all eight bytes of a quadword
// immediate. The decoder used to panic on an 8-byte immediate.
func TestImmediateQuadword(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	bytes := append([]byte{0x7D}, immediate(0x89, 0x67, 0x45, 0x23, 0x01, 0, 0, 0x80)...)
	stepInstruction(t, e, append(bytes, regMode(vax.R4))...)

	if cpu.GPR(vax.R4) != 0x23456789 || cpu.GPR(vax.R5) != 0x80000001 {
		t.Errorf("R4:R5 = %#x:%#x, want 0x23456789:0x80000001", cpu.GPR(vax.R4), cpu.GPR(vax.R5))
	}

	if psl := cpu.PSL(); !psl.N() || psl.Z() {
		t.Errorf("N=%v Z=%v, want N set (the quadword is negative)", psl.N(), psl.Z())
	}
}

// TestImmediateFloat: an F_floating or D_floating immediate is the VAX
// value it encodes. The decoder used to leave the raw VAX bits where
// loadFloat expects IEEE bits, so every float immediate loaded as zero.
func TestImmediateFloat(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	one := []byte{0x80, 0x40, 0, 0} // 1.0: F_floating's (and D_floating's) first word ^X4080

	stepInstruction(t, e, append(append([]byte{0x50}, immediate(one...)...), regMode(vax.R4))...) // MOVF I^#1.0,R4

	if got := cpu.GPR(vax.R4); got != 0x4080 {
		t.Errorf("MOVF I^#1.0: R4 = %#x, want 0x4080", got)
	}

	stepInstruction(t, e, append(append([]byte{0x40}, immediate(one...)...), regMode(vax.R4))...) // ADDF2 I^#1.0,R4

	if got := cpu.GPR(vax.R4); got != 0x4100 {
		t.Errorf("ADDF2 I^#1.0: R4 = %#x, want 0x4100 (2.0)", got)
	}

	cpu.SetGPR(vax.R5, 0xFFFFFFFF)
	stepInstruction(t, e, append(append([]byte{0x70}, immediate(append(one, 0, 0, 0, 0)...)...), regMode(vax.R4))...) // MOVD I^#1.0,R4

	if cpu.GPR(vax.R4) != 0x4080 || cpu.GPR(vax.R5) != 0 {
		t.Errorf("MOVD I^#1.0: R4:R5 = %#x:%#x, want 0x4080:0", cpu.GPR(vax.R4), cpu.GPR(vax.R5))
	}
}

// TestImmediateReservedFloat: a reserved float immediate (sign set,
// exponent zero) is a reserved operand fault.
func TestImmediateReservedFloat(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, e.mem, base, append(append([]byte{0x50}, immediate(0x00, 0x80, 0, 0)...), regMode(vax.R4))...)
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if cpu.GPR(vax.PC) != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (fault vector)", cpu.GPR(vax.PC))
	}
}

// TestImmediateAddress: an address operand in immediate mode is the
// address of its data in the instruction stream (immediate mode is
// (PC)+). PUSHAL I^#5 used to push zero.
func TestImmediateAddress(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.SP, 0x7000)

	stepInstruction(t, e, append([]byte{0xDF}, immediate(5, 0, 0, 0)...)...) // PUSHAL I^#5

	addr, err := mem.LoadLongword(cpu, 0x7000-4)
	if err != nil {
		t.Fatal(err)
	}

	if addr != base+2 {
		t.Errorf("pushed %#x, want %#x (the literal's address)", addr, base+2)
	}

	if v, err := mem.LoadLongword(cpu, addr); err != nil || v != 5 {
		t.Errorf("longword at the pushed address = %d (%v), want 5", v, err)
	}
}

// TestImmediateFieldBase: a field's base in immediate mode is the field's
// address in the instruction stream, so EXTZV reads the field from there.
func TestImmediateFieldBase(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	stepInstruction(t, e, append(append([]byte{0xEF, 4, 4}, immediate(0xA5)...), regMode(vax.R2))...) // EXTZV #4,#4,I^#^XA5,R2

	if got := cpu.GPR(vax.R2); got != 0xA {
		t.Errorf("R2 = %#x, want 0xA", got)
	}
}
