package cpu

import (
	"fmt"
	"math"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// OperandKind says where a decoded Operand's value lives.
type OperandKind int

const (
	// OperandRegister: the value lives in a general register (Reg).
	OperandRegister OperandKind = iota
	// OperandMemory: the value lives at a VAX virtual address (Addr). For
	// AccessAddress/AccessBranch/AccessVarField operands, Addr is the
	// operand's value itself (used directly, never dereferenced) rather
	// than a location to load from — matching the C source's VAXaddr[n]
	// serving both purposes depending on the instruction's own access mode.
	OperandMemory
	// OperandImmediate: the value was resolved at decode time (Value) and
	// has no register or memory location of its own — a short literal, a
	// mode-8/PC "Immediate" operand, or an OP_IM implicit-immediate
	// operand. Writing to an OperandImmediate operand is a reserved-
	// addressing-mode fault.
	OperandImmediate
)

// Operand is one decoded instruction operand. See docs/PHASE-03.md's design
// notes for why this is value-based rather than the C source's pointer/
// scratch-register mechanism (struct OPCODE's address[]/VAXaddr[]/regnum[]
// fields).
type Operand struct {
	Access AccessKind
	Kind   OperandKind
	Reg    vax.Reg
	Addr   uint32
	Value  uint64
	Size   int
}

func loadSized(cpu *vax.CPU, mem *vm.Memory, addr uint32, size int) (uint32, error) {
	switch size {
	case 1:
		b, err := mem.LoadByte(cpu, addr)
		return uint32(b), err
	case 2:
		w, err := mem.LoadWord(cpu, addr)
		return uint32(w), err
	case 4:
		return mem.LoadLongword(cpu, addr)
	default:
		panic(fmt.Sprintf("cpu: unsupported operand size %d", size))
	}
}

// signExtend32 sign-extends a size-byte (1, 2, or 4) value read from the
// instruction stream to a full 32 bits, matching the C source's SEXT macro.
func signExtend32(raw uint32, size int) int32 {
	switch size {
	case 1:
		return int32(int8(raw))
	case 2:
		return int32(int16(raw))
	default:
		return int32(raw)
	}
}

// decodeOperand parses one operand specifier from the instruction stream at
// *pc, advancing *pc past it (and past any addressing-mode-specific
// displacement/immediate data). cpu's general registers are read and, for
// addressing modes that compute a new register value (autoincrement/
// autodecrement, indexed-mode base resolution), written — same timing as
// decode_operand.c: register side effects happen during decode, exactly
// once per operand specifier evaluated.
//
// indexed is true only for the recursive call resolving Indexed mode's base
// operand specifier, to reject Indexed mode nested inside itself (see
// docs/DEVIATIONS.md).
//
// This is the Go port of decode_operand.c.
func decodeOperand(cpu *vax.CPU, mem *vm.Memory, pc *uint32, access AccessKind, size int, litType ShortLiteralType, indexed bool) (Operand, error) {
	op := Operand{Access: access, Size: size}

	// Branch and implicit-immediate operands are encoded directly in the
	// instruction stream with no addressing-mode byte at all — decode_
	// operand.c's `access >= OP_BR` fast path, checked before anything else
	// reads from the instruction stream.
	switch access {
	case AccessImmediate:
		raw, err := loadSized(cpu, mem, *pc, size)
		if err != nil {
			return op, err
		}
		op.Kind = OperandImmediate
		op.Value = uint64(uint32(signExtend32(raw, size)))
		*pc += uint32(size)
		return op, nil

	case AccessBranch:
		raw, err := loadSized(cpu, mem, *pc, size)
		if err != nil {
			return op, err
		}
		disp := signExtend32(raw, size)
		*pc += uint32(size)
		op.Kind = OperandMemory
		op.Addr = uint32(int32(*pc) + disp)
		return op, nil
	}

	optype, err := mem.LoadByte(cpu, *pc)
	if err != nil {
		return op, err
	}
	*pc++

	mode := optype >> 4
	reg := vax.Reg(optype & 0x0F)

	switch {
	case mode == 5: // Register direct: Rn. Fast path, matching the C
		// source's empirically-justified early special case for the most
		// common addressing mode.
		op.Kind = OperandRegister
		op.Reg = reg
		// A register has no VAX address, so an OP_AD/OP_VA operand (e.g.
		// MOVAL/PUSHAL's destination, a bitfield base) resolving to
		// Register mode has no address to report here -- but, per
		// decode_operand.c itself, decode does NOT fault this generically:
		// only a few individual handlers work around it themselves
		// (emul_mova.c and half of emul_push.c self-check and fault;
		// emul_call.c's CALLG/CALLS arglist operand instead uses the
		// register's own value as the address). A Phase 04 change made
		// this fault here, uniformly, for every OP_AD/OP_VA consumer at
		// once -- but Phase 12's first real end-to-end run of kernel.asm
		// (this project's own hand-written microkernel) found that its
		// CHMK dispatcher's `callg ap, (r0)` genuinely depends on the
		// old, per-handler behavior (reusing the caller's AP register
		// value as the new arglist address, a real tail-call idiom), and
		// bitfield.go's loadField/storeField already implement the
		// analogous OperandRegister case for a register-mode bitfield
		// base (a real, defined VAX feature -- the field spans adjacent
		// registers rather than memory) that this fault made unreachable.
		// Reverted per user direction (2026-09-15) rather than narrowed to
		// just CALLG, since neither of those other two consumers ever had
		// this check in the C source either. See docs/DEVIATIONS.md.
		return op, nil

	case mode < 4: // Short literal: S^#n (integer) or S^#f (float).
		op.Kind = OperandImmediate
		if litType == ShortLiteralFloat {
			op.Value = math.Float64bits(shortDouble[optype])
		} else {
			op.Value = uint64(optype)
		}
		if access != AccessRead {
			// A short literal has no address to write to — reserved
			// addressing mode fault, matching decode_operand.c. The
			// operand is still returned, same as the C source (which sets
			// decode_rc but keeps building the operand rather than
			// returning early).
			return op, &Fault{Code: ExcReservedAddr}
		}
		return op, nil

	case mode >= 8 && reg == vax.PC:
		return decodePCRelative(cpu, mem, pc, access, size, mode, op)

	default:
		return decodeGeneral(cpu, mem, pc, size, litType, mode, reg, indexed, op)
	}
}

// decodePCRelative handles the PC-relative addressing modes: Immediate,
// Absolute, and Byte/Word/Long Relative (direct and deferred) — the mode
// 0x08-0x0F forms selected by using the PC as the addressing-mode byte's
// register field.
func decodePCRelative(cpu *vax.CPU, mem *vm.Memory, pc *uint32, access AccessKind, size int, mode byte, op Operand) (Operand, error) {
	switch mode {
	case 0x08: // Immediate: I^#n
		raw, err := loadSized(cpu, mem, *pc, size)
		if err != nil {
			return op, err
		}
		op.Kind = OperandImmediate
		op.Value = uint64(uint32(signExtend32(raw, size)))
		*pc += uint32(size)
		if access == AccessModify || access == AccessWrite {
			return op, &Fault{Code: ExcReservedAddr}
		}
		return op, nil

	case 0x09: // Absolute: @#addr
		addr, err := mem.LoadLongword(cpu, *pc)
		if err != nil {
			return op, err
		}
		*pc += 4
		op.Kind = OperandMemory
		op.Addr = addr
		return op, nil

	case 0x0A, 0x0B: // Byte relative [deferred]
		raw, err := mem.LoadByte(cpu, *pc)
		if err != nil {
			return op, err
		}
		*pc++
		return pcRelativeTarget(cpu, mem, pc, mode == 0x0B, int32(int8(raw)), op)

	case 0x0C, 0x0D: // Word relative [deferred]
		raw, err := mem.LoadWord(cpu, *pc)
		if err != nil {
			return op, err
		}
		*pc += 2
		return pcRelativeTarget(cpu, mem, pc, mode == 0x0D, int32(int16(raw)), op)

	case 0x0E, 0x0F: // Long relative [deferred]
		raw, err := mem.LoadLongword(cpu, *pc)
		if err != nil {
			return op, err
		}
		*pc += 4
		return pcRelativeTarget(cpu, mem, pc, mode == 0x0F, int32(raw), op)
	}

	panic("cpu: unreachable PC-relative mode")
}

// pcRelativeTarget resolves a PC-relative displacement (already read, with
// *pc already advanced past it) to a memory operand: pc+disp directly, or,
// deferred, the longword pointed to by pc+disp — one dereference, done at
// decode time, matching decode_operand.c's byte/word/long relative deferred
// cases.
func pcRelativeTarget(cpu *vax.CPU, mem *vm.Memory, pc *uint32, deferred bool, disp int32, op Operand) (Operand, error) {
	target := uint32(int32(*pc) + disp)
	op.Kind = OperandMemory
	if !deferred {
		op.Addr = target
		return op, nil
	}
	addr, err := mem.LoadLongword(cpu, target)
	if err != nil {
		return op, err
	}
	op.Addr = addr
	return op, nil
}

// decodeGeneral handles the general-register addressing modes: Indexed,
// Register deferred, Autodecrement, Autoincrement [deferred], and Byte/
// Word/Long displacement (direct and deferred).
func decodeGeneral(cpu *vax.CPU, mem *vm.Memory, pc *uint32, size int, litType ShortLiteralType, mode byte, reg vax.Reg, indexed bool, op Operand) (Operand, error) {
	switch mode {
	case 0x04: // Indexed: base[Rx]
		if indexed {
			// Index mode may not be used as the base of another Index
			// mode operand specifier — reserved addressing mode fault.
			// decode_operand.c detects this (`if (idx) return
			// VAX_ILLADDRFAULT`) but with a sentinel that bypasses the
			// normal set_fault/vax.fault mechanism entirely, so the
			// caller ends up handling a stale or mismatched fault instead
			// of this one; see docs/DEVIATIONS.md.
			return op, &Fault{Code: ExcReservedAddr}
		}
		index := cpu.GPR(reg)
		base, err := decodeOperand(cpu, mem, pc, op.Access, size, litType, true)
		if err != nil {
			return op, err
		}
		op.Kind = OperandMemory
		op.Addr = base.Addr + index*uint32(size)
		return op, nil

	case 0x06: // Register deferred: (Rn)
		op.Kind = OperandMemory
		op.Addr = cpu.GPR(reg)
		return op, nil

	case 0x07: // Autodecrement: -(Rn)
		v := cpu.GPR(reg) - uint32(size)
		cpu.SetGPR(reg, v)
		op.Kind = OperandMemory
		op.Addr = v
		return op, nil

	case 0x08: // Autoincrement: (Rn)+
		v := cpu.GPR(reg)
		cpu.SetGPR(reg, v+uint32(size))
		op.Kind = OperandMemory
		op.Addr = v
		return op, nil

	case 0x09: // Autoincrement deferred: @(Rn)+
		// Rn holds the address of a longword containing the operand's
		// address; Rn is then advanced by 4 (always a longword pointer,
		// regardless of the operand's own size). One dereference resolves
		// the operand's address, left for later Operand.Load/Store to read
		// or write through — see docs/DEVIATIONS.md for how this departs
		// from decode_operand.c, which eagerly loads the operand's value
		// during decode instead, silently breaking write access through
		// this mode.
		ptr := cpu.GPR(reg)
		cpu.SetGPR(reg, ptr+4)
		addr, err := mem.LoadLongword(cpu, ptr)
		if err != nil {
			return op, err
		}
		op.Kind = OperandMemory
		op.Addr = addr
		return op, nil

	case 0x0A, 0x0B: // Byte displacement [deferred]: B^n(Rn) / @B^n(Rn)
		raw, err := mem.LoadByte(cpu, *pc)
		if err != nil {
			return op, err
		}
		*pc++
		return displacementTarget(cpu, mem, reg, mode == 0x0B, int32(int8(raw)), op)

	case 0x0C, 0x0D: // Word displacement [deferred]
		raw, err := mem.LoadWord(cpu, *pc)
		if err != nil {
			return op, err
		}
		*pc += 2
		return displacementTarget(cpu, mem, reg, mode == 0x0D, int32(int16(raw)), op)

	case 0x0E, 0x0F: // Long displacement [deferred]
		raw, err := mem.LoadLongword(cpu, *pc)
		if err != nil {
			return op, err
		}
		*pc += 4
		return displacementTarget(cpu, mem, reg, mode == 0x0F, int32(raw), op)
	}

	panic("cpu: unreachable addressing mode")
}

// displacementTarget resolves Rn+disp to a memory operand: that address
// directly, or, deferred, the longword pointed to by it.
func displacementTarget(cpu *vax.CPU, mem *vm.Memory, reg vax.Reg, deferred bool, disp int32, op Operand) (Operand, error) {
	target := uint32(int32(cpu.GPR(reg)) + disp)
	op.Kind = OperandMemory
	if !deferred {
		op.Addr = target
		return op, nil
	}
	addr, err := mem.LoadLongword(cpu, target)
	if err != nil {
		return op, err
	}
	op.Addr = addr
	return op, nil
}
