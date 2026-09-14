package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulAddByte(t *testing.T) {
	cases := []struct {
		name         string
		addend, sum  uint32
		wantResult   byte
		wantV, wantC bool
	}{
		{"ordinary", 3, 4, 7, false, false},
		// Two positives summing to a negative byte: signed overflow, no
		// unsigned carry (150 still fits in 8 bits).
		{"signed overflow, no carry", 50, 100, 150, true, false},
		// A negative and a positive byte: unsigned carry out of the top
		// bit, but no signed overflow (operands have different signs).
		{"carry, no overflow", 100, 200, 44, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.addend)
			cpu.SetGPR(vax.R2, tc.sum)

			stepInstruction(t, e, 0x80, regMode(vax.R1), regMode(vax.R2)) // ADDB2

			if got := byte(cpu.GPR(vax.R2)); got != tc.wantResult {
				t.Errorf("result = %#x, want %#x", got, tc.wantResult)
			}
			psl := cpu.PSL()
			if psl.V() != tc.wantV || psl.C() != tc.wantC {
				t.Errorf("V=%v C=%v, want V=%v C=%v", psl.V(), psl.C(), tc.wantV, tc.wantC)
			}
		})
	}
}

func TestEmulAddb3(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 3)
	cpu.SetGPR(vax.R2, 4)

	stepInstruction(t, e, 0x81, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3)) // ADDB3

	if got := byte(cpu.GPR(vax.R3)); got != 7 {
		t.Errorf("R3 = %#x, want 7", got)
	}
}

func TestEmulSubByteOperandOrder(t *testing.T) {
	// SUBB2 sub,dif: dif <- dif - sub. Getting the operand order backwards
	// is an easy mistake, so this pins it down explicitly.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 3)  // subtrahend
	cpu.SetGPR(vax.R2, 10) // minuend / dst

	stepInstruction(t, e, 0x82, regMode(vax.R1), regMode(vax.R2)) // SUBB2

	if got := byte(cpu.GPR(vax.R2)); got != 7 {
		t.Errorf("R2 = %#x, want 7 (10 - 3)", got)
	}
}

func TestEmulSubb3OperandOrder(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 3)  // sub
	cpu.SetGPR(vax.R2, 10) // min

	stepInstruction(t, e, 0x83, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3)) // SUBB3

	if got := byte(cpu.GPR(vax.R3)); got != 7 {
		t.Errorf("R3 = %#x, want 7 (10 - 3)", got)
	}
}

func TestEmulSubByteBorrowAndOverflow(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 5) // subtrahend
	cpu.SetGPR(vax.R2, 3) // minuend, smaller -> borrow

	stepInstruction(t, e, 0x82, regMode(vax.R1), regMode(vax.R2))

	if !cpu.PSL().C() {
		t.Error("C = false, want true (3 - 5 borrows)")
	}

	// 0x80 (-128) - 1 overflows (can't represent +129 in a signed byte... wait
	// -128 - 1 = -129, doesn't fit either): minuend at the negative boundary
	// minus a positive subtrahend.
	cpu.SetGPR(vax.R1, 1)
	cpu.SetGPR(vax.R2, 0x80)
	stepInstruction(t, e, 0x82, regMode(vax.R1), regMode(vax.R2))
	if !cpu.PSL().V() {
		t.Error("V = false, want true (largest negative byte minus 1 overflows)")
	}
}

func TestEmulMulByte(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 5)
	cpu.SetGPR(vax.R2, 3)
	stepInstruction(t, e, 0x84, regMode(vax.R1), regMode(vax.R2)) // MULB2
	if got := byte(cpu.GPR(vax.R2)); got != 15 {
		t.Errorf("result = %d, want 15", got)
	}
	if cpu.PSL().V() {
		t.Error("V = true, want false")
	}

	// -56 * -56 = 3136, which doesn't fit in a signed byte.
	cpu.SetGPR(vax.R1, 200)
	cpu.SetGPR(vax.R2, 200)
	stepInstruction(t, e, 0x84, regMode(vax.R1), regMode(vax.R2))
	if !cpu.PSL().V() {
		t.Error("V = false, want true (product doesn't fit a byte)")
	}
	if cpu.PSL().C() {
		t.Error("C = true, want false (MUL always clears C)")
	}
}

func TestEmulDivByte(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 3) // divisor
	cpu.SetGPR(vax.R2, 0xF7 /* -9 */)
	stepInstruction(t, e, 0x86, regMode(vax.R1), regMode(vax.R2)) // DIVB2
	if got := int8(cpu.GPR(vax.R2)); got != -3 {
		t.Errorf("result = %d, want -3 (truncated toward zero)", got)
	}
	if cpu.PSL().V() {
		t.Error("V = true, want false")
	}
}

func TestEmulDivByZeroGuarded(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0)  // divisor: zero
	cpu.SetGPR(vax.R2, 10) // dividend

	stepInstruction(t, e, 0x86, regMode(vax.R1), regMode(vax.R2)) // must not panic

	if got := cpu.GPR(vax.R2); got != 10 {
		t.Errorf("result = %d, want 10 (dividend left unchanged)", got)
	}
	if !cpu.PSL().V() {
		t.Error("V = false, want true (divide by zero)")
	}
}

func TestEmulDivMinIntOverflowGuarded(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xFF) // divisor: -1
	cpu.SetGPR(vax.R2, 0x80) // dividend: -128, the size's minSigned

	stepInstruction(t, e, 0x86, regMode(vax.R1), regMode(vax.R2)) // must not panic

	if got := byte(cpu.GPR(vax.R2)); got != 0x80 {
		t.Errorf("result = %#x, want 0x80 (unchanged)", got)
	}
	if !cpu.PSL().V() {
		t.Error("V = false, want true (MinInt / -1 overflows)")
	}
}

func TestEmulLogicalOpsLeaveCarryUnaffected(t *testing.T) {
	// emul_integer_math.c force-clears C for BIS and computes a meaningless
	// arithmetic-carry value for BIC; the manual specifies C unaffected for
	// both (and XOR). See docs/DEVIATIONS.md.
	cases := []struct {
		name   string
		opcode byte
	}{
		{"BISB2", 0x88},
		{"BICB2", 0x8A},
		{"XORB2", 0x8C},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, 0x0F)
			cpu.SetGPR(vax.R2, 0xFF)
			setC(cpu, true)

			stepInstruction(t, e, tc.opcode, regMode(vax.R1), regMode(vax.R2))

			if !cpu.PSL().C() {
				t.Error("C = false, want unaffected (true)")
			}
			if cpu.PSL().V() {
				t.Error("V = true, want false")
			}
		})
	}
}

func TestEmulBis(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x0F)
	cpu.SetGPR(vax.R2, 0xF0)
	stepInstruction(t, e, 0x88, regMode(vax.R1), regMode(vax.R2)) // BISB2
	if got := byte(cpu.GPR(vax.R2)); got != 0xFF {
		t.Errorf("result = %#x, want 0xff", got)
	}
}

func TestEmulBic(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x0F)                                      // mask
	cpu.SetGPR(vax.R2, 0xFF)                                      // dst
	stepInstruction(t, e, 0x8A, regMode(vax.R1), regMode(vax.R2)) // BICB2
	if got := byte(cpu.GPR(vax.R2)); got != 0xF0 {
		t.Errorf("result = %#x, want 0xf0", got)
	}
}

func TestEmulXor(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xFF)
	cpu.SetGPR(vax.R2, 0x0F)
	stepInstruction(t, e, 0x8C, regMode(vax.R1), regMode(vax.R2)) // XORB2
	if got := byte(cpu.GPR(vax.R2)); got != 0xF0 {
		t.Errorf("result = %#x, want 0xf0", got)
	}
}

// TestEmulBisb3DestinationScaleDeviation documents and pins down a known,
// deferred deviation: instruction_table.h declares BISB3's destination
// operand as longword-scaled (a transcription error -- every sibling Bxx3
// form uses a byte destination), replicated as-is in the generated Go
// table. A register destination therefore gets fully overwritten (the
// byte result zero-extended to 4 bytes) rather than only its low byte
// updated. See docs/DEVIATIONS.md.
func TestEmulBisb3DestinationScaleDeviation(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x0F)
	cpu.SetGPR(vax.R2, 0xF0)
	cpu.SetGPR(vax.R3, 0xAAAAAAAA)

	stepInstruction(t, e, 0x89, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3)) // BISB3

	if got := cpu.GPR(vax.R3); got != 0x000000FF {
		t.Errorf("R3 = %#x, want 0x000000ff (all 4 bytes written, per the table deviation)", got)
	}
}

func TestEmulAdwcCarryPropagation(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	// Simulate a multi-word add: low words overflow, high-word ADWC should
	// see the carry.
	cpu.SetGPR(vax.R1, 0xFFFF)
	cpu.SetGPR(vax.R2, 1)
	stepInstruction(t, e, 0xA0, regMode(vax.R1), regMode(vax.R2)) // ADDW2: 0xFFFF+1 -> 0, C=true
	if got := uint16(cpu.GPR(vax.R2)); got != 0 {
		t.Fatalf("low word = %#x, want 0", got)
	}
	if !cpu.PSL().C() {
		t.Fatal("C = false after low-word add, want true")
	}

	cpu.SetGPR(vax.R3, 0)
	cpu.SetGPR(vax.R4, 0)
	stepInstruction(t, e, 0xD8, regMode(vax.R3), regMode(vax.R4)) // ADWC: 0+0+C

	if got := uint16(cpu.GPR(vax.R4)); got != 1 {
		t.Errorf("high word = %#x, want 1 (carry propagated)", got)
	}
	if cpu.PSL().V() {
		t.Error("V = true, want false")
	}
}

func TestEmulSbwcBorrowPropagation(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 1)
	cpu.SetGPR(vax.R2, 0)
	stepInstruction(t, e, 0xA2, regMode(vax.R1), regMode(vax.R2)) // SUBW2: 0-1 -> borrow
	if !cpu.PSL().C() {
		t.Fatal("C = false after low-word sub, want true")
	}

	cpu.SetGPR(vax.R3, 0)
	cpu.SetGPR(vax.R4, 1)
	stepInstruction(t, e, 0xD9, regMode(vax.R3), regMode(vax.R4)) // SBWC: 1-0-C

	if got := uint16(cpu.GPR(vax.R4)); got != 0 {
		t.Errorf("high word = %#x, want 0 (borrow propagated)", got)
	}
}
