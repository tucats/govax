package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/registers"
)

func TestEmulEmulPositive(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	cpu.SetGPR(registers.R1, 1000000)
	cpu.SetGPR(registers.R2, 1000000)
	cpu.SetGPR(registers.R3, 500)

	// EMUL R1, R2, R3, @#0x4000
	bytes := []byte{0x7A, regMode(registers.R1), regMode(registers.R2), regMode(registers.R3)}
	bytes = append(bytes, absoluteMode(0x4000)...)
	stepInstruction(t, e, bytes...)

	got, err := mem.LoadQuadword(cpu, 0x4000)
	if err != nil {
		t.Fatalf("LoadQuadword: %v", err)
	}

	const want = uint64(1000000)*1000000 + 500

	if got != want {
		t.Errorf("product = %d, want %d", got, want)
	}

	psl := cpu.PSL()
	if psl.N() || psl.Z() || psl.V() || psl.C() {
		t.Errorf("condition codes = N=%v Z=%v V=%v C=%v, want all clear", psl.N(), psl.Z(), psl.V(), psl.C())
	}
}

func TestEmulEmulNegative(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	mulr := int32(-1000)
	cpu.SetGPR(registers.R1, uint32(mulr))
	cpu.SetGPR(registers.R2, 2000)
	cpu.SetGPR(registers.R3, 0)

	bytes := []byte{0x7A, regMode(registers.R1), regMode(registers.R2), regMode(registers.R3)}
	bytes = append(bytes, absoluteMode(0x4000)...)
	stepInstruction(t, e, bytes...)

	got, err := mem.LoadQuadword(cpu, 0x4000)
	if err != nil {
		t.Fatalf("LoadQuadword: %v", err)
	}

	wantProduct := int64(-2000000)
	if want := uint64(wantProduct); got != want {
		t.Errorf("product = %#x, want %#x (-2000000)", got, want)
	}

	if !cpu.PSL().N() {
		t.Error("N = false, want true (negative product)")
	}
}

func TestEmulEmulZeroSetsZ(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	cpu.SetGPR(registers.R1, 0)
	cpu.SetGPR(registers.R2, 12345)
	cpu.SetGPR(registers.R3, 0)

	bytes := []byte{0x7A, regMode(registers.R1), regMode(registers.R2), regMode(registers.R3)}
	bytes = append(bytes, absoluteMode(0x4000)...)
	stepInstruction(t, e, bytes...)

	if !cpu.PSL().Z() {
		t.Error("Z = false, want true")
	}
}

func TestEmulEdivNormal(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	if err := mem.StoreQuadword(cpu, 0x4000, 100); err != nil {
		t.Fatalf("StoreQuadword: %v", err)
	}

	cpu.SetGPR(registers.R1, 7) // divisor

	// EDIV R1, @#0x4000, R2, R3
	bytes := []byte{0x7B, regMode(registers.R1)}
	bytes = append(bytes, absoluteMode(0x4000)...)
	bytes = append(bytes, regMode(registers.R2), regMode(registers.R3))
	stepInstruction(t, e, bytes...)

	if got := int32(cpu.GPR(registers.R2)); got != 14 {
		t.Errorf("quotient = %d, want 14", got)
	}

	if got := int32(cpu.GPR(registers.R3)); got != 2 {
		t.Errorf("remainder = %d, want 2", got)
	}

	psl := cpu.PSL()
	if psl.N() || psl.Z() || psl.V() {
		t.Errorf("condition codes = N=%v Z=%v V=%v, want all clear", psl.N(), psl.Z(), psl.V())
	}
}

// TestEmulEdivNegativeDividendRemainderSign checks Note 1: the remainder has
// the same sign as the dividend.
func TestEmulEdivNegativeDividendRemainderSign(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	dividend := int64(-100)
	if err := mem.StoreQuadword(cpu, 0x4000, uint64(dividend)); err != nil {
		t.Fatalf("StoreQuadword: %v", err)
	}

	cpu.SetGPR(registers.R1, 7)

	bytes := []byte{0x7B, regMode(registers.R1)}
	bytes = append(bytes, absoluteMode(0x4000)...)
	bytes = append(bytes, regMode(registers.R2), regMode(registers.R3))
	stepInstruction(t, e, bytes...)

	if got := int32(cpu.GPR(registers.R2)); got != -14 {
		t.Errorf("quotient = %d, want -14", got)
	}

	if got := int32(cpu.GPR(registers.R3)); got != -2 {
		t.Errorf("remainder = %d, want -2 (same sign as the dividend)", got)
	}

	if !cpu.PSL().N() {
		t.Error("N = false, want true (negative quotient)")
	}
}

// TestEmulEdivQuotientOverflow checks the manual's second V condition (a
// genuine quotient overflow, distinct from divide-by-zero): a large
// quadword dividend divided by a small divisor whose true quotient doesn't
// fit in 32 bits falls back to the same Note 3 behavior divide-by-zero
// uses (quotient <- bits 31:0 of the dividend, remainder <- 0, V set) --
// fixed in Phase 12, see docs/DEVIATIONS.md.
func TestEmulEdivQuotientOverflow(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	if err := mem.StoreQuadword(cpu, 0x4000, 0x0000000200000000); err != nil {
		t.Fatalf("StoreQuadword: %v", err)
	}
	
	cpu.SetGPR(registers.R1, 1) // divisor: true quotient is 0x200000000, doesn't fit in 32 bits

	bytes := []byte{0x7B, regMode(registers.R1)}
	bytes = append(bytes, absoluteMode(0x4000)...)
	bytes = append(bytes, regMode(registers.R2), regMode(registers.R3))
	stepInstruction(t, e, bytes...)

	if got := cpu.GPR(registers.R2); got != 0 {
		t.Errorf("quotient = %#x, want 0 (bits 31:0 of the dividend)", got)
	}

	if got := cpu.GPR(registers.R3); got != 0 {
		t.Errorf("remainder = %#x, want 0", got)
	}
	
	if !cpu.PSL().V() {
		t.Error("V = false, want true (quotient overflow)")
	}
}
