package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulAdawiNormal(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, 0x2000, 0x10, 0x00) // sum = 0x0010

	cpu.SetGPR(vax.R1, 5) // addend, register mode

	bytes := []byte{0x58, regMode(vax.R1)}
	bytes = append(bytes, absoluteMode(0x2000)...)
	stepInstruction(t, e, bytes...)

	got, err := mem.LoadWord(cpu, 0x2000)
	if err != nil {
		t.Fatalf("LoadWord: %v", err)
	}
	if got != 0x15 {
		t.Errorf("sum = %#x, want 0x15", got)
	}
	psl := cpu.PSL()
	if psl.N() || psl.Z() || psl.V() || psl.C() {
		t.Errorf("condition codes = N=%v Z=%v V=%v C=%v, want all clear", psl.N(), psl.Z(), psl.V(), psl.C())
	}
}

// TestEmulAdawiOverflowSetsV checks N/Z now come from the truncated word
// result actually stored, not the untruncated 32-bit sum: 32767+1 = 32768
// is positive as a 32-bit sum, but the stored word (-32768 once truncated)
// is negative, so N must be true -- fixed in Phase 12, see
// docs/DEVIATIONS.md.
func TestEmulAdawiOverflowSetsV(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, 0x2000, 0xFF, 0x7F) // sum = 32767

	cpu.SetGPR(vax.R1, 1) // addend

	bytes := []byte{0x58, regMode(vax.R1)}
	bytes = append(bytes, absoluteMode(0x2000)...)
	stepInstruction(t, e, bytes...)

	if !cpu.PSL().V() {
		t.Error("V = false, want true (32768 overflows a signed word)")
	}
	if !cpu.PSL().N() {
		t.Error("N = false, want true (computed from the truncated word -32768, which is negative)")
	}

	got, err := mem.LoadWord(cpu, 0x2000)
	if err != nil {
		t.Fatalf("LoadWord: %v", err)
	}
	if int16(got) != -32768 {
		t.Errorf("stored sum = %#x, want -32768 (32768 truncated to a word)", got)
	}
}

// TestEmulAdawiSetsRealCarry checks C now reflects a real carry out of bit
// 15, matching the manual, rather than always reading false -- fixed in
// Phase 12, see docs/DEVIATIONS.md.
func TestEmulAdawiSetsRealCarry(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, 0x2000, 0xFF, 0xFF) // sum = -1 as a signed word

	cpu.SetGPR(vax.R1, 1) // addend: -1 + 1 = 0, a real carry out of bit 15

	bytes := []byte{0x58, regMode(vax.R1)}
	bytes = append(bytes, absoluteMode(0x2000)...)
	stepInstruction(t, e, bytes...)

	if !cpu.PSL().C() {
		t.Error("C = false, want true (a real carry out of bit 15: 0xFFFF + 1 = 0x10000)")
	}
	if !cpu.PSL().Z() {
		t.Error("Z = false, want true")
	}
}

func TestEmulAdawiRegisterDestinationFaults(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	cpu.SetGPR(vax.R1, 1)
	cpu.SetGPR(vax.R2, 0)
	stepInstruction(t, e, 0x58, regMode(vax.R1), regMode(vax.R2))

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

func TestEmulAdawiOddAddressFaults(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	cpu.SetGPR(vax.R1, 1)
	bytes := []byte{0x58, regMode(vax.R1)}
	bytes = append(bytes, absoluteMode(0x2001)...) // odd address
	stepInstruction(t, e, bytes...)

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}
