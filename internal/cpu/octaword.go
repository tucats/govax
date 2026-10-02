package cpu

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// Octaword is a VAX octaword: 128 bits, the widest integer data type the
// VAX has. Go has no 128-bit integer type, so it is held as two 64-bit
// halves. In memory, as with every VAX integer, the low-order byte comes
// first (little-endian): Lo is the 8 bytes at the operand's address, Hi the
// 8 bytes after them.
//
// The VAX has no octaword arithmetic, only moves (MOVO, CLRO, and the
// address instructions MOVAO and PUSHAO). The H_floating format is also 16
// bytes and is loaded and stored through the same paths.
type Octaword struct {
	Lo, Hi uint64
}

// IsZero reports whether all 128 bits are zero (the Z condition code of a
// move).
func (o Octaword) IsZero() bool { return o.Lo == 0 && o.Hi == 0 }

// Negative reports whether bit 127, the sign bit of an octaword read as a
// signed integer, is set (the N condition code of a move).
func (o Octaword) Negative() bool { return o.Hi>>63 != 0 }

// octawordRegisterLimit is the highest register an octaword register
// operand may start in. An octaword in a register spans four of them, Rn
// through Rn+3, so R11 (R11, AP, FP, SP) is the last start that doesn't
// run into the PC. The manual calls R12 and above UNPREDICTABLE; govax
// raises a reserved-addressing-mode fault, as decided in Phase 35
// (docs/PHASE-35.md, Decision 4).
const octawordRegisterLimit = vax.R11

// LoadOctaword reads op's full 16-byte value. Use it for an operand whose
// Size is 16; Load, which returns a uint64, would see only the low half.
//
// A register operand reads four consecutive registers, low-order longword
// first: Rn holds bits 0-31, Rn+1 bits 32-63, Rn+2 bits 64-95, and Rn+3
// bits 96-127 (as a quadword uses Rn and Rn+1).
func (op Operand) LoadOctaword(cpu *vax.CPU, mem *vm.Memory) (Octaword, error) {
	switch op.Kind {
	case OperandImmediate:
		return Octaword{Lo: op.Value, Hi: op.High}, nil

	case OperandRegister:
		return Octaword{
			Lo: uint64(cpu.GPR(op.Reg)) | uint64(cpu.GPR(op.Reg+1))<<32,
			Hi: uint64(cpu.GPR(op.Reg+2)) | uint64(cpu.GPR(op.Reg+3))<<32,
		}, nil

	default:
		return loadOctaword(cpu, mem, op.Addr)
	}
}

// StoreOctaword writes v to op, a 16-byte operand: to four consecutive
// registers, or 16 bytes of memory.
func (op Operand) StoreOctaword(cpu *vax.CPU, mem *vm.Memory, v Octaword) error {
	switch op.Kind {
	case OperandImmediate:
		return ErrImmutableOperand

	case OperandRegister:
		cpu.SetGPR(op.Reg, uint32(v.Lo))
		cpu.SetGPR(op.Reg+1, uint32(v.Lo>>32))
		cpu.SetGPR(op.Reg+2, uint32(v.Hi))
		cpu.SetGPR(op.Reg+3, uint32(v.Hi>>32))

		return nil

	default:
		return storeOctaword(cpu, mem, op.Addr, v)
	}
}

// loadOctaword reads the 16 bytes at addr as two quadwords.
func loadOctaword(cpu *vax.CPU, mem *vm.Memory, addr uint32) (Octaword, error) {
	lo, err := mem.LoadQuadword(cpu, addr)
	if err != nil {
		return Octaword{}, err
	}

	hi, err := mem.LoadQuadword(cpu, addr+8)
	if err != nil {
		return Octaword{}, err
	}

	return Octaword{Lo: lo, Hi: hi}, nil
}

// storeOctaword writes v to the 16 bytes at addr. It first checks that the
// first and last bytes can be written (16 bytes span at most two of the
// VAX's 512-byte pages, so those two cover every page involved). Otherwise
// a fault on the second half would leave the first half already changed,
// and an instruction that faults must leave memory as it found it, so that
// the operating system can fix the fault (page the memory in, say) and run
// the instruction again from the start.
func storeOctaword(cpu *vax.CPU, mem *vm.Memory, addr uint32, v Octaword) error {
	for _, a := range []uint32{addr, addr + 15} {
		if _, err := mem.Translate(cpu, a, vm.AccessWrite); err != nil {
			return err
		}
	}

	if err := mem.StoreQuadword(cpu, addr, v.Lo); err != nil {
		return err
	}

	return mem.StoreQuadword(cpu, addr+8, v.Hi)
}
