package cpu

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// ErrImmutableOperand is returned by Operand.Store when called on an
// OperandImmediate operand. Decode already raises a reserved-addressing-
// mode Fault for any operand whose Access requires a write but which
// resolved to an immediate (see decodeOperand and decodePCRelative), so a
// well-behaved instruction Handler should never reach this — it exists as a
// defensive backstop, not a path real VAX code can trigger.
var ErrImmutableOperand = errors.New("cpu: cannot store to an immediate operand")

// Load resolves op's current value, sized to op.Size bytes (1, 2, 4, or 8),
// zero-extended into the result. This is the Go port of storage.c's
// get_operand, minus the pointer/scratch-register mechanism it uses to give
// register, memory, and literal operands a uniform pointer interface — see
// docs/PHASE-03.md's design notes. Sign-extension, when an instruction
// needs it, is the caller's job, same as in the C source.
//
// A register operand sized 8 bytes reads a register pair (op.Reg holds the
// low-order longword, op.Reg+1 the high-order longword) — the VAX quadword
// register-operand convention, which the C source gets "for free" from
// vax.reg[]'s contiguous layout and a raw 8-byte pointer read; this port
// makes it explicit since Operand has no pointer to alias through.
func (op Operand) Load(cpu *vax.CPU, mem *vm.Memory) (uint64, error) {
	switch op.Kind {
	case OperandImmediate:
		return op.Value, nil

	case OperandRegister:
		if op.Size == 8 {
			lo := uint64(cpu.GPR(op.Reg))
			hi := uint64(cpu.GPR(op.Reg + 1))
			return lo | hi<<32, nil
		}
		return uint64(maskLow(cpu.GPR(op.Reg), op.Size)), nil

	case OperandMemory:
		return loadValue(cpu, mem, op.Addr, op.Size)
	}
	panic(fmt.Sprintf("cpu: invalid OperandKind %d", op.Kind))
}

// Store writes value (only its low op.Size bytes are used) back to op.
// This is the Go port of storage.c's put_operand.
//
// A register operand smaller than a longword only overwrites its low
// op.Size bytes, leaving the rest of the register undisturbed — matching
// put_operand's byte-limited copy through its resolved pointer (and
// vm.Memory.LoadRegister's identical "architected register preserves upper
// bytes" behavior from Phase 02). A register operand sized 8 bytes writes
// the register pair op.Reg/op.Reg+1, the Store-side counterpart of Load's
// quadword register-pair handling above.
func (op Operand) Store(cpu *vax.CPU, mem *vm.Memory, value uint64) error {
	switch op.Kind {
	case OperandImmediate:
		return ErrImmutableOperand

	case OperandRegister:
		if op.Size == 8 {
			cpu.SetGPR(op.Reg, uint32(value))
			cpu.SetGPR(op.Reg+1, uint32(value>>32))
			return nil
		}
		cpu.SetGPR(op.Reg, mergeLow(cpu.GPR(op.Reg), uint32(value), op.Size))
		return nil

	case OperandMemory:
		return storeValue(cpu, mem, op.Addr, op.Size, value)
	}
	panic(fmt.Sprintf("cpu: invalid OperandKind %d", op.Kind))
}

// maskLow zero-extends the low size bytes of v, discarding the rest.
func maskLow(v uint32, size int) uint32 {
	if size >= 4 {
		return v
	}
	return v & (1<<(uint(size)*8) - 1)
}

// mergeLow replaces the low size bytes of orig with the low size bytes of
// v, leaving the remaining (higher) bytes of orig untouched.
func mergeLow(orig, v uint32, size int) uint32 {
	if size >= 4 {
		return v
	}
	mask := uint32(1<<(uint(size)*8) - 1)
	return (orig &^ mask) | (v & mask)
}

func loadValue(cpu *vax.CPU, mem *vm.Memory, addr uint32, size int) (uint64, error) {
	if size == 8 {
		return mem.LoadQuadword(cpu, addr)
	}
	v, err := loadSized(cpu, mem, addr, size)
	return uint64(v), err
}

func storeValue(cpu *vax.CPU, mem *vm.Memory, addr uint32, size int, value uint64) error {
	switch size {
	case 1:
		return mem.StoreByte(cpu, addr, byte(value))
	case 2:
		return mem.StoreWord(cpu, addr, uint16(value))
	case 4:
		return mem.StoreLongword(cpu, addr, uint32(value))
	case 8:
		return mem.StoreQuadword(cpu, addr, value)
	default:
		panic(fmt.Sprintf("cpu: unsupported operand size %d", size))
	}
}
