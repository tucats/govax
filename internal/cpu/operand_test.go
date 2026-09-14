package cpu

import (
	"errors"
	"math"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// fixture returns a CPU (virtual memory disabled, so addresses are used
// directly) and a 1MB memory, matching internal/vm's test convention.
func fixture() (*vax.CPU, *vm.Memory) {
	return vax.New(), vm.NewMemory(1 << 20)
}

func putBytes(t *testing.T, cpu *vax.CPU, mem *vm.Memory, addr uint32, bytes ...byte) {
	t.Helper()
	for i, b := range bytes {
		if err := mem.StoreByte(cpu, addr+uint32(i), b); err != nil {
			t.Fatalf("StoreByte(%#x): %v", addr+uint32(i), err)
		}
	}
}

func putLongword(t *testing.T, cpu *vax.CPU, mem *vm.Memory, addr uint32, v uint32) {
	t.Helper()
	if err := mem.StoreLongword(cpu, addr, v); err != nil {
		t.Fatalf("StoreLongword(%#x): %v", addr, err)
	}
}

const base = 0x1000

func TestDecodeOperandRegisterDirect(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x53) // mode 5, reg 3

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandRegister || op.Reg != vax.R3 {
		t.Errorf("op = %+v, want Kind=Register Reg=R3", op)
	}
	if pc != base+1 {
		t.Errorf("pc = %#x, want %#x", pc, base+1)
	}
}

func TestDecodeOperandRegisterDirectPCAlias(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.PC, 0xABCD1234)
	putBytes(t, cpu, mem, base, 0x5F) // mode 5, reg 15 == PC

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandRegister || op.Reg != vax.PC {
		t.Fatalf("op = %+v, want Kind=Register Reg=PC", op)
	}
	if cpu.GPR(op.Reg) != 0xABCD1234 {
		t.Errorf("GPR(op.Reg) = %#x, want 0xABCD1234 (R15/PC alias)", cpu.GPR(op.Reg))
	}
}

func TestDecodeOperandShortLiteralInt(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x05) // mode 0, value 5

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandImmediate || op.Value != 5 {
		t.Errorf("op = %+v, want Kind=Immediate Value=5", op)
	}
	if pc != base+1 {
		t.Errorf("pc = %#x, want %#x", pc, base+1)
	}
}

func TestDecodeOperandShortLiteralFloat(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x00) // mode 0, index 0 -> short_double[0] == 0.5

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralFloat, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandImmediate {
		t.Fatalf("op.Kind = %v, want Immediate", op.Kind)
	}
	if got := math.Float64frombits(op.Value); got != 0.5 {
		t.Errorf("float value = %v, want 0.5", got)
	}
}

func TestDecodeOperandShortLiteralWriteFaults(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x05)

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessWrite, 4, ShortLiteralInt, false)

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcReservedAddr {
		t.Fatalf("err = %v, want *Fault{Code: ExcReservedAddr}", err)
	}
	// The operand is still populated, matching decode_operand.c (which sets
	// decode_rc but keeps building the operand rather than bailing out).
	if op.Kind != OperandImmediate || op.Value != 5 {
		t.Errorf("op = %+v, want Kind=Immediate Value=5 despite the fault", op)
	}
}

func TestDecodeOperandRegisterDeferred(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x2000)
	putBytes(t, cpu, mem, base, 0x62) // mode 6, reg 2

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x2000 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x2000", op)
	}
}

func TestDecodeOperandAutodecrement(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x2010)
	putBytes(t, cpu, mem, base, 0x72) // mode 7, reg 2

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessModify, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x200C {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x200C", op)
	}
	if cpu.GPR(vax.R2) != 0x200C {
		t.Errorf("R2 = %#x, want 0x200C (decremented)", cpu.GPR(vax.R2))
	}
}

func TestDecodeOperandAutoincrement(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x3000)
	putBytes(t, cpu, mem, base, 0x82) // mode 8, reg 2

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x3000 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x3000", op)
	}
	if cpu.GPR(vax.R2) != 0x3004 {
		t.Errorf("R2 = %#x, want 0x3004 (incremented)", cpu.GPR(vax.R2))
	}
}

// TestDecodeOperandAutoincrementDeferred also guards against the bug
// documented in docs/DEVIATIONS.md: decode_operand.c's @(Rn)+ eagerly loads
// the operand's *value* into a scratch register during decode instead of
// resolving its address for later access, which would silently discard any
// write through this addressing mode. This asserts the fixed behavior: the
// dereferenced pointer becomes the operand's address, not its value.
func TestDecodeOperandAutoincrementDeferred(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x4000)
	putLongword(t, cpu, mem, 0x4000, 0x5000) // *R2 == 0x5000, the operand's real address
	putLongword(t, cpu, mem, 0x5000, 0xCAFEBABE)
	putBytes(t, cpu, mem, base, 0x92) // mode 9, reg 2

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessModify, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x5000 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x5000 (single dereference)", op)
	}
	if cpu.GPR(vax.R2) != 0x4004 {
		t.Errorf("R2 = %#x, want 0x4004 (pointer advanced by 4)", cpu.GPR(vax.R2))
	}
}

func TestDecodeOperandByteDisplacement(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x1000)
	putBytes(t, cpu, mem, base, 0xA2, 0x10) // mode A, reg 2, disp +16

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x1010 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x1010", op)
	}
	if pc != base+2 {
		t.Errorf("pc = %#x, want %#x", pc, base+2)
	}
}

func TestDecodeOperandByteDisplacementNegative(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x1000)
	putBytes(t, cpu, mem, base, 0xA2, 0xF0) // disp -16

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Addr != 0x0FF0 {
		t.Errorf("Addr = %#x, want 0xFF0", op.Addr)
	}
}

func TestDecodeOperandByteDisplacementDeferred(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x1000)
	putLongword(t, cpu, mem, 0x1004, 0x9999)
	putBytes(t, cpu, mem, base, 0xB2, 0x04) // mode B, reg 2, disp +4

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x9999 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x9999", op)
	}
}

func TestDecodeOperandWordDisplacement(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x2000)
	putBytes(t, cpu, mem, base, 0xC2, 0x00, 0x01) // mode C, reg 2, disp +0x0100

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x2100 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x2100", op)
	}
	if pc != base+3 {
		t.Errorf("pc = %#x, want %#x", pc, base+3)
	}
}

func TestDecodeOperandWordDisplacementDeferred(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x2000)
	putLongword(t, cpu, mem, 0x2100, 0xAAAA)
	putBytes(t, cpu, mem, base, 0xD2, 0x00, 0x01)

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0xAAAA {
		t.Errorf("op = %+v, want Kind=Memory Addr=0xAAAA", op)
	}
}

func TestDecodeOperandLongDisplacement(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x2000)
	putBytes(t, cpu, mem, base, 0xE2)
	putLongword(t, cpu, mem, base+1, 0x00010000)

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x00012000 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x12000", op)
	}
	if pc != base+5 {
		t.Errorf("pc = %#x, want %#x", pc, base+5)
	}
}

func TestDecodeOperandLongDisplacementDeferred(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0x2000)
	putLongword(t, cpu, mem, 0x00012000, 0x77777777)
	putBytes(t, cpu, mem, base, 0xF2)
	putLongword(t, cpu, mem, base+1, 0x00010000)

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x77777777 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x77777777", op)
	}
}

func TestDecodeOperandIndexed(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R3, 2)                   // index register: index 2
	cpu.SetGPR(vax.R5, 0x8000)              // base register (register deferred)
	putBytes(t, cpu, mem, base, 0x43, 0x65) // mode 4 reg 3 (index), then mode 6 reg 5 (base)

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x8008 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x8008 (0x8000 + 2*4)", op)
	}
	if pc != base+2 {
		t.Errorf("pc = %#x, want %#x", pc, base+2)
	}
}

func TestDecodeOperandDoubleIndexedFaults(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x43, 0x45) // mode 4 reg 3, then mode 4 reg 5 (illegal nesting)

	pc := uint32(base)
	_, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcReservedAddr {
		t.Fatalf("err = %v, want *Fault{Code: ExcReservedAddr}", err)
	}
}

func TestDecodeOperandPCImmediate(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x8F) // mode 8, reg 15 == PC
	putLongword(t, cpu, mem, base+1, 0xDEADBEEF)

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandImmediate || uint32(op.Value) != 0xDEADBEEF {
		t.Errorf("op = %+v, want Kind=Immediate Value=0xDEADBEEF", op)
	}
	if pc != base+5 {
		t.Errorf("pc = %#x, want %#x", pc, base+5)
	}
}

func TestDecodeOperandPCImmediateSignExtends(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x8F, 0xFF) // one-byte immediate, value -1

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 1, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if uint32(op.Value) != 0xFFFFFFFF {
		t.Errorf("Value = %#x, want 0xFFFFFFFF (sign-extended -1)", uint32(op.Value))
	}
}

func TestDecodeOperandPCImmediateWriteFaults(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x8F)
	putLongword(t, cpu, mem, base+1, 0)

	pc := uint32(base)
	_, err := decodeOperand(cpu, mem, &pc, AccessModify, 4, ShortLiteralInt, false)

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcReservedAddr {
		t.Fatalf("err = %v, want *Fault{Code: ExcReservedAddr}", err)
	}
}

func TestDecodeOperandPCAbsolute(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x9F) // mode 9, reg 15 == PC
	putLongword(t, cpu, mem, base+1, 0x12345678)

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x12345678 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x12345678", op)
	}
	if pc != base+5 {
		t.Errorf("pc = %#x, want %#x", pc, base+5)
	}
}

func TestDecodeOperandPCByteRelative(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0xAF, 0x10) // mode A, reg 15 == PC, disp +16

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	want := uint32(base + 2 + 16) // relative to the PC just past the displacement byte
	if op.Kind != OperandMemory || op.Addr != want {
		t.Errorf("op = %+v, want Kind=Memory Addr=%#x", op, want)
	}
}

func TestDecodeOperandPCByteRelativeDeferred(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0xBF, 0x10) // mode B, reg 15 == PC, disp +16
	putLongword(t, cpu, mem, base+2+16, 0x42424242)

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessRead, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandMemory || op.Addr != 0x42424242 {
		t.Errorf("op = %+v, want Kind=Memory Addr=0x42424242", op)
	}
}

func TestDecodeOperandAccessBranch(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x10) // raw signed byte displacement, +16, no mode byte

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessBranch, 1, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	want := uint32(base + 1 + 16)
	if op.Kind != OperandMemory || op.Addr != want {
		t.Errorf("op = %+v, want Kind=Memory Addr=%#x", op, want)
	}
	if pc != base+1 {
		t.Errorf("pc = %#x, want %#x", pc, base+1)
	}
}

func TestDecodeOperandAccessImmediate(t *testing.T) {
	cpu, mem := fixture()
	putLongword(t, cpu, mem, base, 0x11223344) // raw literal, no mode byte

	pc := uint32(base)
	op, err := decodeOperand(cpu, mem, &pc, AccessImmediate, 4, ShortLiteralInt, false)
	if err != nil {
		t.Fatalf("decodeOperand: %v", err)
	}
	if op.Kind != OperandImmediate || uint32(op.Value) != 0x11223344 {
		t.Errorf("op = %+v, want Kind=Immediate Value=0x11223344", op)
	}
	if pc != base+4 {
		t.Errorf("pc = %#x, want %#x", pc, base+4)
	}
}
