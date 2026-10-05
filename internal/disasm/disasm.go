package disasm

import (
	"fmt"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// ByteReader supplies bytes for disassembly by VAX virtual address.
// internal/asm's *Assembler satisfies it via ByteAt, so a program's own
// output can be disassembled directly; SliceReader adapts a plain []byte
// for anything else (a loaded .exe image, a console's live memory
// snapshot, ...).
type ByteReader interface {
	ByteAt(addr uint32) byte
}

// SliceReader adapts a []byte, addressed from 0, to ByteReader. An address
// past the end of the slice reads back as 0.
type SliceReader []byte

func (s SliceReader) ByteAt(addr uint32) byte {
	if addr >= uint32(len(s)) {
		return 0
	}

	return s[addr]
}

// Decoded is one disassembled instruction: its mnemonic and operands, and
// the number of bytes it occupied in the instruction stream. A routine's
// entry mask (EntryMask) is a Decoded too, with IsMask set.
type Decoded struct {
	Mnemonic string
	Operands []Operand
	Length   uint32

	// Address is where the instruction is: what a symbolizer's
	// ConstantNamer takes as the current module.
	Address uint32

	// IsMask marks a routine's register-save mask word, decoded by
	// EntryMask rather than as an instruction: Mask is the word, and Name
	// the routine's name ("" if the caller doesn't know it).
	IsMask bool
	Mask   uint16
	Name   string
}

// Disassemble decodes one instruction from r at pc, matching
// decode_opcode.c/disasm_operand.c's combined algorithm — reusing the
// same internal/cpu instruction table internal/asm's Assemble does rather
// than a duplicate copy (see docs/PHASE-11.md). Unlike internal/cpu's own
// decodeOperand (used at execution time), this never reads or writes
// register state: autoincrement and autodecrement modes are recorded,
// never performed.
//
// Each operand comes back in parts (Operand), with the address it refers
// to when that's known (Target). Naming those addresses is the caller's:
// it sets an operand's Symbol from its own symbol table, and String shows
// the name in place of the number.
func Disassemble(r ByteReader, pc uint32) (Decoded, error) {
	start := pc
	f := r.ByteAt(pc)
	pc++

	var op cpu.Opcode
	if f > 0xFC {
		op.Extended = f
		op.Function = r.ByteAt(pc)
		pc++
	} else {
		op.Function = f
	}

	inst := cpu.Instructions().Lookup(op)
	if inst == nil {
		return Decoded{}, vmserrors.New(vmserrors.VAX_BADOPCODEAT, start)
	}

	dec := Decoded{Mnemonic: inst.Name, Address: start}

	for i := 0; i < inst.OperandCount; i++ {
		operand, err := decodeOperand(r, &pc, inst.Access[i], inst.Scale[i], inst.DataType[i], false)
		if err != nil {
			return Decoded{}, err
		}

		dec.Operands = append(dec.Operands, operand)
	}

	dec.Length = pc - start

	return dec, nil
}

// EntryMask decodes the word at pc as a routine's register-save mask
// (the word a CALLS or CALLG to the routine reads, which its .ENTRY
// directive wrote), not as an instruction. name is the routine's name, if
// the caller knows it.
func EntryMask(r ByteReader, pc uint32, name string) Decoded {
	mask := uint16(r.ByteAt(pc)) | uint16(r.ByteAt(pc+1))<<8

	return Decoded{Mnemonic: ".ENTRY", Length: 2, Address: pc, IsMask: true, Mask: mask, Name: name}
}

// loadSized reads a 1, 2, or 4-byte little-endian value at addr.
func loadSized(r ByteReader, addr uint32, size int) uint32 {
	switch size {
	case 1:
		return uint32(r.ByteAt(addr))

	case 2:
		return uint32(r.ByteAt(addr)) | uint32(r.ByteAt(addr+1))<<8

	default:
		return uint32(r.ByteAt(addr)) | uint32(r.ByteAt(addr+1))<<8 |
			uint32(r.ByteAt(addr+2))<<16 | uint32(r.ByteAt(addr+3))<<24
	}
}

// loadBytes reads size bytes at addr.
func loadBytes(r ByteReader, addr uint32, size int) []byte {
	b := make([]byte, size)

	for i := range b {
		b[i] = r.ByteAt(addr + uint32(i))
	}

	return b
}

func signExtend(raw uint32, size int) int32 {
	switch size {
	case 1:
		return int32(int8(raw))

	case 2:
		return int32(int16(raw))

	default:
		return int32(raw)
	}
}

// decodeOperand decodes one operand at *pc, advancing it past whatever it
// reads — matching disasm_operand.c. indexed is true only for the
// recursive call decoding Indexed mode's own base operand, to reject
// Indexed mode nested inside itself the same way internal/cpu's
// decodeOperand does.
func decodeOperand(r ByteReader, pc *uint32, access cpu.AccessKind, size int, dtype cpu.DataType, indexed bool) (Operand, error) {
	op := Operand{Register: -1, Index: -1, Access: access, Type: dtype, Size: size}

	switch access {
	case cpu.AccessImmediate:
		op.Mode = ModeInline
		op.Width = size
		op.Value = loadSized(r, *pc, size)
		op.Bytes = loadBytes(r, *pc, size)
		*pc += uint32(size)

		return op, nil

	case cpu.AccessBranch:
		raw := loadSized(r, *pc, size)
		op.Mode = ModeBranch
		op.Width = size
		op.Displacement = signExtend(raw, size)
		*pc += uint32(size)
		op.Target = uint32(int32(*pc) + op.Displacement)
		op.HasTarget = true

		return op, nil
	}

	specifier := r.ByteAt(*pc)
	*pc++
	mode := specifier >> 4
	reg := specifier & 0x0F

	switch {
	case mode < 4:
		op.Mode = ModeLiteral
		op.Value = uint32(specifier)

		return op, nil

	case mode == 5:
		op.Mode = ModeRegister
		op.Register = int(reg)

		return op, nil

	case mode >= 8 && reg == 0x0F:
		return decodePCRelative(r, pc, mode, op)

	default:
		return decodeGeneral(r, pc, mode, reg, op, indexed)
	}
}

// decodePCRelative decodes the PC-relative addressing modes: Immediate,
// Absolute, and Byte/Word/Long Relative (direct and deferred), selected
// by using the PC as the specifier's register field.
//
// A relative operand's Target is its resolved absolute address, not the
// raw displacement the reference tool's disasm_operand.c prints: PC's
// value is exactly known at disassembly time, so the address is both more
// readable and, unlike the reference tool's choice, round-trips through
// internal/asm's assembleDisplacement, which parses "B^address" as an
// absolute address and computes the displacement itself (see
// docs/PHASE-11.md).
func decodePCRelative(r ByteReader, pc *uint32, mode byte, op Operand) (Operand, error) {
	switch mode {
	case 0x08: // Immediate: I^#n
		op.Mode = ModeImmediate
		op.Width = op.Size
		op.Value = loadSized(r, *pc, min(op.Size, 4))
		op.Bytes = loadBytes(r, *pc, op.Size)
		*pc += uint32(op.Size)

		return op, nil

	case 0x09: // Absolute: @#addr
		op.Mode = ModeAbsolute
		op.Target = loadSized(r, *pc, 4)
		op.HasTarget = true
		*pc += 4

		return op, nil

	case 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F: // Byte, word, long relative [deferred]
		op.Mode = ModeRelative
		op.Deferred = mode&1 != 0
		op.Width = displacementWidth(mode)
		op.Displacement = signExtend(loadSized(r, *pc, op.Width), op.Width)
		*pc += uint32(op.Width)
		op.Target = uint32(int32(*pc) + op.Displacement)
		op.HasTarget = true

		return op, nil
	}

	return Operand{}, vmserrors.New(vmserrors.VAX_INTERNAL, fmt.Sprintf("unreachable PC-relative mode %X", mode))
}

// displacementWidth is the displacement size of a displacement mode
// (0x0A to 0x0F): a byte for A and B, a word for C and D, a longword for
// E and F.
func displacementWidth(mode byte) int {
	switch mode {
	case 0x0A, 0x0B:
		return 1
	case 0x0C, 0x0D:
		return 2
	default:
		return 4
	}
}

// decodeGeneral decodes the general-register addressing modes: Indexed,
// Register deferred, Autodecrement, Autoincrement [deferred], and Byte/
// Word/Long displacement (direct and deferred).
func decodeGeneral(r ByteReader, pc *uint32, mode, reg byte, op Operand, indexed bool) (Operand, error) {
	op.Register = int(reg)

	switch mode {
	case 0x04: // Indexed: base[Rx]
		if indexed {
			return Operand{}, vmserrors.New(vmserrors.VAX_INDEXNEST)
		}

		base, err := decodeOperand(r, pc, op.Access, op.Size, op.Type, true)
		if err != nil {
			return Operand{}, err
		}

		base.Index = int(reg)

		return base, nil

	case 0x06: // Register deferred: (Rn)
		op.Mode = ModeRegisterDeferred

	case 0x07: // Autodecrement: -(Rn)
		op.Mode = ModeAutodecrement

	case 0x08: // Autoincrement: (Rn)+
		op.Mode = ModeAutoincrement

	case 0x09: // Autoincrement deferred: @(Rn)+
		op.Mode = ModeAutoincrementDeferred

	case 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F: // Displacement [deferred]: B^n(Rn) / @B^n(Rn)
		op.Mode = ModeDisplacement
		op.Deferred = mode&1 != 0
		op.Width = displacementWidth(mode)
		op.Displacement = signExtend(loadSized(r, *pc, op.Width), op.Width)
		*pc += uint32(op.Width)

	default:
		return Operand{}, vmserrors.New(vmserrors.VAX_INTERNAL, fmt.Sprintf("unreachable addressing mode %X", mode))
	}

	return op, nil
}
