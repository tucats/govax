package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulAcbFloatPositiveAddendBoundary(t *testing.T) {
	// index lands exactly on limit -- the same off-by-one the integer ACB
	// family had (docs/DEVIATIONS.md), fixed identically here: the manual
	// specifies branching on index <= limit for a non-negative addend.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setFloatReg(t, cpu, 4, vax.R1, 10.0) // limit
	setFloatReg(t, cpu, 4, vax.R2, 1.0)  // addend
	setFloatReg(t, cpu, 4, vax.R3, 9.0)  // index -> 10.0 after add, == limit
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x4F, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3), 0x10, 0x00) // ACBF limit,addend,index,+16

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	
	if got := getFloatReg(t, cpu, 4, vax.R3); got != 10.0 {
		t.Errorf("index = %v, want 10.0", got)
	}
	
	want := uint32(base + 6 + 16) // taken
	if got := cpu.GPR(vax.PC); got != want {
		t.Errorf("PC = %#x, want %#x (branch taken: index reached limit exactly)", got, want)
	}
}

func TestEmulAcbFloatNotTaken(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setFloatReg(t, cpu, 4, vax.R1, 10.0) // limit
	setFloatReg(t, cpu, 4, vax.R2, 1.0)  // addend
	setFloatReg(t, cpu, 4, vax.R3, 10.0) // index -> 11.0 after add, exceeds limit
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x4F, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3), 0x10, 0x00)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := getFloatReg(t, cpu, 4, vax.R3); got != 11.0 {
		t.Errorf("index = %v, want 11.0", got)
	}

	want := uint32(base + 6) // not taken
	if got := cpu.GPR(vax.PC); got != want {
		t.Errorf("PC = %#x, want %#x (loop exits)", got, want)
	}
}

func TestEmulAcbFloatNegativeAddend(t *testing.T) {
	// negative addend: branch when index >= limit (a counting-down loop).
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setFloatReg(t, cpu, 4, vax.R1, 0.0)  // limit
	setFloatReg(t, cpu, 4, vax.R2, -1.0) // addend
	setFloatReg(t, cpu, 4, vax.R3, 1.0)  // index -> 0.0 after add, == limit
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x4F, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3), 0x10, 0x00)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	want := uint32(base + 6 + 16)
	if got := cpu.GPR(vax.PC); got != want {
		t.Errorf("PC = %#x, want %#x (branch taken: index still >= limit)", got, want)
	}
}

func TestEmulAcbDouble(t *testing.T) {
	// ACBD: unimplemented in the C reference (no case, no dispatch) --
	// implemented fresh here by direct analogy to ACBF.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setFloatReg(t, cpu, 8, vax.R1, 5.0) // limit (R1:R2)
	setFloatReg(t, cpu, 8, vax.R3, 1.0) // addend (R3:R4)
	setFloatReg(t, cpu, 8, vax.R5, 3.0) // index (R5:R6) -> 4.0 after add
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x6F, regMode(vax.R1), regMode(vax.R3), regMode(vax.R5), 0x10, 0x00) // ACBD

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := getFloatReg(t, cpu, 8, vax.R5); got != 4.0 {
		t.Errorf("index = %v, want 4.0", got)
	}

	want := uint32(base + 6 + 16) // taken (4.0 < limit 5.0)
	if got := cpu.GPR(vax.PC); got != want {
		t.Errorf("PC = %#x, want %#x", got, want)
	}
}

func TestEmulAcbFloatCUnaffected(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setC(cpu, true)
	setFloatReg(t, cpu, 4, vax.R1, 10.0)
	setFloatReg(t, cpu, 4, vax.R2, 1.0)
	setFloatReg(t, cpu, 4, vax.R3, 1.0)
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x4F, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3), 0x10, 0x00)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if !cpu.PSL().C() {
		t.Error("C = false, want true (ACBF/ACBD leave C unaffected)")
	}
}

// TestEmulAcbFloatShortLiterals runs ACBF with its limit and addend as
// short literals, the usual way a loop is written ("ACBF #10.0, #1.0, R3,
// LOOP"). A short literal is a 6-bit constant inside the operand
// specifier byte itself (addressing modes 0-3, so the byte is 0x00-0x3F);
// for a floating operand it selects one of 64 values from a table, so
// literal 0x08 is 1.0 and 0x22 is 10.0. ACBF's table entry once said its
// literals were integers, which made these the integers 8 and 34, then
// read as floating bit patterns (tiny values near zero), so the loop
// branched the wrong way. See docs/DEVIATIONS.md, Phase 35.
func TestEmulAcbFloatShortLiterals(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setFloatReg(t, cpu, 4, vax.R3, 8.0) // index -> 9.0 after adding 1.0, <= 10.0
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x4F, 0x22, 0x08, regMode(vax.R3), 0x10, 0x00) // ACBF S^#10.0,S^#1.0,R3,+16

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := getFloatReg(t, cpu, 4, vax.R3); got != 9.0 {
		t.Errorf("index = %v, want 9.0", got)
	}

	want := uint32(base + 6 + 16) // taken: 9.0 hasn't passed 10.0
	if got := cpu.GPR(vax.PC); got != want {
		t.Errorf("PC = %#x, want %#x (branch taken)", got, want)
	}
}
