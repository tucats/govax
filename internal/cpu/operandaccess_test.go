package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestOperandLoadImmediate(t *testing.T) {
	cpu, mem := fixture()
	op := Operand{Kind: OperandImmediate, Value: 0xDEADBEEF, Size: 4}

	v, err := op.Load(cpu, mem)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v != 0xDEADBEEF {
		t.Errorf("Load() = %#x, want 0xDEADBEEF", v)
	}
}

func TestOperandStoreImmediateFails(t *testing.T) {
	cpu, mem := fixture()
	op := Operand{Kind: OperandImmediate, Value: 5, Size: 4}

	if err := op.Store(cpu, mem, 9); !errors.Is(err, ErrImmutableOperand) {
		t.Fatalf("Store() err = %v, want ErrImmutableOperand", err)
	}
}

func TestOperandRegisterRoundTripLongword(t *testing.T) {
	cpu, mem := fixture()
	op := Operand{Kind: OperandRegister, Reg: vax.R3, Size: 4}

	if err := op.Store(cpu, mem, 0x11223344); err != nil {
		t.Fatalf("Store: %v", err)
	}
	v, err := op.Load(cpu, mem)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v != 0x11223344 {
		t.Errorf("Load() = %#x, want 0x11223344", v)
	}
}

func TestOperandRegisterBytePreservesUpperBits(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0xAABBCCDD)
	op := Operand{Kind: OperandRegister, Reg: vax.R2, Size: 1}

	if err := op.Store(cpu, mem, 0xFF); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if got := cpu.GPR(vax.R2); got != 0xAABBCCFF {
		t.Errorf("R2 = %#x, want 0xAABBCCFF (only the low byte replaced)", got)
	}

	v, err := op.Load(cpu, mem)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v != 0xFF {
		t.Errorf("Load() = %#x, want 0xFF (only the low byte read)", v)
	}
}

func TestOperandRegisterWordPreservesUpperBits(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.R2, 0xAABBCCDD)
	op := Operand{Kind: OperandRegister, Reg: vax.R2, Size: 2}

	if err := op.Store(cpu, mem, 0x1234); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if got := cpu.GPR(vax.R2); got != 0xAABB1234 {
		t.Errorf("R2 = %#x, want 0xAABB1234", got)
	}
}

func TestOperandRegisterQuadwordPair(t *testing.T) {
	cpu, mem := fixture()
	op := Operand{Kind: OperandRegister, Reg: vax.R2, Size: 8}

	if err := op.Store(cpu, mem, 0x1122334455667788); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if got := cpu.GPR(vax.R2); got != 0x55667788 {
		t.Errorf("R2 (low longword) = %#x, want 0x55667788", got)
	}
	if got := cpu.GPR(vax.R3); got != 0x11223344 {
		t.Errorf("R3 (high longword) = %#x, want 0x11223344", got)
	}

	v, err := op.Load(cpu, mem)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v != 0x1122334455667788 {
		t.Errorf("Load() = %#x, want 0x1122334455667788", v)
	}
}

func TestOperandMemoryRoundTrip(t *testing.T) {
	cases := []struct {
		size int
		val  uint64
	}{
		{1, 0xAB},
		{2, 0xBEEF},
		{4, 0xDEADBEEF},
		{8, 0x0123456789ABCDEF},
	}

	for _, c := range cases {
		cpu, mem := fixture()
		op := Operand{Kind: OperandMemory, Addr: 0x500, Size: c.size}

		if err := op.Store(cpu, mem, c.val); err != nil {
			t.Fatalf("size %d: Store: %v", c.size, err)
		}
		got, err := op.Load(cpu, mem)
		if err != nil {
			t.Fatalf("size %d: Load: %v", c.size, err)
		}
		if got != c.val {
			t.Errorf("size %d: Load() = %#x, want %#x", c.size, got, c.val)
		}
	}
}

func TestOperandMemoryDoesNotDisturbNeighbors(t *testing.T) {
	cpu, mem := fixture()
	putLongword(t, cpu, mem, 0x500-4, 0x11111111)
	putLongword(t, cpu, mem, 0x500+1, 0x22222222)

	op := Operand{Kind: OperandMemory, Addr: 0x500, Size: 1}
	if err := op.Store(cpu, mem, 0xFF); err != nil {
		t.Fatalf("Store: %v", err)
	}

	before, _ := mem.LoadLongword(cpu, 0x500-4)
	after, _ := mem.LoadLongword(cpu, 0x500+1)
	if before != 0x11111111 || after != 0x22222222 {
		t.Errorf("neighboring memory disturbed: before=%#x after=%#x", before, after)
	}
}
