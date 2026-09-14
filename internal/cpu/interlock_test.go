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

// TestEmulAdawiOverflowSetsV also documents a second replicated
// emul_interlock.c deviation: N/Z come from the *untruncated* 32-bit sum,
// not the word actually stored -- 32767+1 = 32768 is positive as a 32-bit
// sum (N clear) even though the stored word (-32768 once truncated) is
// negative. See docs/DEVIATIONS.md and interlock.go's doc comment.
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
	if cpu.PSL().N() {
		t.Error("N = true, want false (computed from the untruncated 32-bit sum 32768, which is positive)")
	}

	got, err := mem.LoadWord(cpu, 0x2000)
	if err != nil {
		t.Fatalf("LoadWord: %v", err)
	}
	if int16(got) != -32768 {
		t.Errorf("stored sum = %#x, want -32768 (32768 truncated to a word)", got)
	}
}

// TestEmulAdawiNeverSetsCarry checks the replicated emul_interlock.c
// deviation: C is always cleared, never the real carry out of the addition
// the manual describes -- see docs/DEVIATIONS.md and interlock.go's doc
// comment.
func TestEmulAdawiNeverSetsCarry(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, 0x2000, 0xFF, 0xFF) // sum = -1 as a signed word

	cpu.SetGPR(vax.R1, 1) // addend: -1 + 1 = 0, a real carry out of bit 15

	bytes := []byte{0x58, regMode(vax.R1)}
	bytes = append(bytes, absoluteMode(0x2000)...)
	stepInstruction(t, e, bytes...)

	if cpu.PSL().C() {
		t.Error("C = true, want false (emul_interlock.c's SETCONDITIONBITS(data,0L) can never set C)")
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
