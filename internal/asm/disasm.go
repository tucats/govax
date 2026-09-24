package asm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// ByteReader supplies bytes for disassembly by VAX virtual address.
// *Assembler satisfies it via ByteAt, so a program's own output can be
// disassembled directly; SliceReader adapts a plain []byte for anything
// else (a loaded .exe image, a console's live memory snapshot, ...).
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

var regNames = [16]string{
	"R0", "R1", "R2", "R3", "R4", "R5", "R6", "R7",
	"R8", "R9", "R10", "R11", "AP", "FP", "SP", "PC",
}

// Decoded is one disassembled instruction: its mnemonic and formatted
// operand list, plus the number of bytes it occupied in the instruction
// stream.
type Decoded struct {
	Mnemonic string
	Operands []string
	Values   []uint32
	Length   uint32
}

// String renders dec in the syntax this package's own Assemble can parse
// back in: "MNEMONIC OP1,OP2,...". This is what makes the round-trip
// property in docs/PHASE-11.md's deliverables checkable: assemble a
// fixture, disassemble each instruction, reassemble the disassembly, and
// compare bytes.
func (dec Decoded) String() string {
	s := dec.Mnemonic

	for i, op := range dec.Operands {
		if i == 0 {
			s += " "
		} else {
			s += ","
		}

		s += op
	}

	return s
}

// Disassemble decodes one instruction from r at pc, matching
// decode_opcode.c/disasm_operand.c's combined algorithm — reusing the same
// internal/cpu instruction table Assemble does rather than a duplicate copy
// (see docs/PHASE-11.md). Unlike internal/cpu's own decodeOperand (used at
// execution time), this never reads or writes register state: autoincrement/
// autodecrement addressing modes are formatted as text, never performed.
//
// Symbolic operand formatting — showing a matching label's name instead of
// a raw hex address — is intentionally not implemented: it's a pure display
// nicety in the reference tool (disasm_operand.c consults the symbol table
// only when printing, never when reparsing its own output), and adding it
// here would mean every call accepting and searching a symbol table whether
// or not the caller wants that. A caller that wants symbolic output can
// post-process Decoded.Operands itself against its own symbol table.
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

	dec := Decoded{Mnemonic: inst.Name}

	for i := 0; i < inst.OperandCount; i++ {
		text, value, err := formatOperand(r, &pc, inst.Access[i], inst.Scale[i], inst.Type, false)
		if err != nil {
			return Decoded{}, err
		}

		dec.Operands = append(dec.Operands, text)
		dec.Values = append(dec.Values, value)
	}

	dec.Length = pc - start

	return dec, nil
}

// FormatMask renders a 16-bit register-set mask as "^M<...>" text, matching
// console_disasm.c's format_mask() — used to display a .ENTRY's saved-
// register mask word during disassembly (see the console's own
// entry-mask detection, which looks up SymbolInfo.Entry). Bit 15 is IV
// (integer overflow trap enable), bit 14 is DV (decimal overflow trap
// enable); bits 0-13 print as "R<n>", matching the reference tool's own
// formatting even though only R0-R11 are ever settable through this
// package's maskLiteral parser (see maskBits).
func FormatMask(mask uint16) string {
	var names []string

	for n := 0; n < 16; n++ {
		if mask&(1<<uint(n)) == 0 {
			continue
		}

		switch n {
		case 15:
			names = append(names, "IV")
		case 14:
			names = append(names, "DV")
		default:
			names = append(names, fmt.Sprintf("R%d", n))
		}
	}

	return "^M<" + strings.Join(names, ",") + ">"
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

func formatIntHex(v uint32, size int) string {
	switch size {
	case 1:
		return fmt.Sprintf("%02X", v)

	case 2:
		return fmt.Sprintf("%04X", v)

	default:
		return fmt.Sprintf("%08X", v)
	}
}

// formatFloatValue renders f in plain decimal (never exponent) notation, so
// it round-trips through this package's own parseFloat (which only accepts
// digits, '.', a sign, and 'E' — Go's %g form can emit a bare exponent with
// no 'E' for some values, which parseFloat wouldn't recognize).
func formatFloatValue(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// formatOperand formats one operand at *pc, advancing it past whatever it
// reads — matching disasm_operand.c. indexed is true only for the
// recursive call formatting Indexed mode's own base operand, to reject
// Indexed mode nested inside itself the same way internal/cpu's
// decodeOperand does.
func formatOperand(r ByteReader, pc *uint32, access cpu.AccessKind, size int, litType cpu.ShortLiteralType, indexed bool) (string, uint32, error) {
	switch access {
	case cpu.AccessImmediate:
		v := loadSized(r, *pc, size)
		*pc += uint32(size)

		return "#" + formatIntHex(v, size), v, nil

	case cpu.AccessBranch:
		raw := loadSized(r, *pc, size)
		disp := signExtend(raw, size)
		*pc += uint32(size)
		dest := uint32(int32(*pc) + disp)

		return fmt.Sprintf("%08X", dest), dest, nil
	}

	optype := r.ByteAt(*pc)
	*pc++
	mode := optype >> 4
	reg := optype & 0x0F

	switch {
	case mode < 4:
		if litType == cpu.ShortLiteralFloat {
			return "S^#" + formatFloatValue(cpu.ShortFloat(int(optype))), uint32(optype), nil
		}

		return fmt.Sprintf("S^#%02X", optype), uint32(optype), nil

	case mode == 5:
		return regNames[reg], uint32(reg), nil

	case mode >= 8 && reg == 0x0F:
		return formatPCRelative(r, pc, mode, size, litType)

	default:
		return formatGeneral(r, pc, mode, reg, access, size, litType, indexed)
	}
}

// formatPCRelative formats the PC-relative addressing modes: Immediate,
// Absolute, and Byte/Word/Long Relative (direct and deferred) — selected by
// using the PC as the addressing-mode byte's register field.
//
// Relative (0x0A/0x0C/0x0E, and their deferred forms) deliberately show the
// resolved absolute destination address rather than the raw displacement
// byte/word/longword the reference tool's disasm_operand.c prints: PC's
// value is exactly known at disassembly time, so showing the destination is
// both more readable and — unlike the reference tool's own choice here —
// actually round-trips through this package's own assembleDisplacement,
// which parses "B^address" as an absolute address and computes the
// relative displacement itself. Printing the raw displacement byte instead
// would silently reassemble to a wrong target unless it happened to also be
// a valid absolute address, so this is treated as a fixable disassembler
// issue rather than reference behavior worth replicating — see
// docs/PHASE-11.md.
func formatPCRelative(r ByteReader, pc *uint32, mode byte, size int, litType cpu.ShortLiteralType) (string, uint32, error) {
	switch mode {
	case 0x08: // Immediate: I^#n
		if litType == cpu.ShortLiteralFloat && (size == 4 || size == 8) {
			var bits uint64

			if size == 4 {
				bits = uint64(loadSized(r, *pc, 4))
			} else {
				lo := uint64(loadSized(r, *pc, 4))
				hi := uint64(loadSized(r, *pc+4, 4))
				bits = lo | hi<<32
			}

			*pc += uint32(size)

			return "I^#" + formatFloatValue(cpu.DecodeFloat(bits, size)), 0, nil
		}

		v := loadSized(r, *pc, size)
		*pc += uint32(size)

		return "I^#" + formatIntHex(v, size), v, nil

	case 0x09: // Absolute: @#addr
		v := loadSized(r, *pc, 4)
		*pc += 4

		return fmt.Sprintf("@#%08X", v), v, nil

	case 0x0A, 0x0B: // Byte relative [deferred]
		raw := loadSized(r, *pc, 1)
		*pc++

		return formatPCRelTarget(*pc, signExtend(raw, 1), "B^", mode == 0x0B), raw, nil

	case 0x0C, 0x0D: // Word relative [deferred]
		raw := loadSized(r, *pc, 2)
		*pc += 2

		return formatPCRelTarget(*pc, signExtend(raw, 2), "W^", mode == 0x0D), raw, nil

	case 0x0E, 0x0F: // Long relative [deferred]
		raw := loadSized(r, *pc, 4)
		*pc += 4

		return formatPCRelTarget(*pc, signExtend(raw, 4), "L^", mode == 0x0F), raw, nil
	}

	return "", 0, vmserrors.New(vmserrors.VAX_INTERNAL, fmt.Sprintf("unreachable PC-relative mode %X", mode))
}

func formatPCRelTarget(pc uint32, disp int32, prefix string, deferred bool) string {
	dest := uint32(int32(pc) + disp)

	if deferred {
		prefix = "@" + prefix
	}

	return fmt.Sprintf("%s%08X", prefix, dest)
}

// formatGeneral formats the general-register addressing modes: Indexed,
// Register deferred, Autodecrement, Autoincrement [deferred], and Byte/
// Word/Long displacement (direct and deferred).
func formatGeneral(r ByteReader, pc *uint32, mode, reg byte, access cpu.AccessKind, size int, litType cpu.ShortLiteralType, indexed bool) (string, uint32, error) {
	rn := regNames[reg]

	switch mode {
	case 0x04: // Indexed: base[Rx]
		if indexed {
			return "", 0, vmserrors.New(vmserrors.VAX_INDEXNEST)
		}

		base, value, err := formatOperand(r, pc, access, size, litType, true)
		if err != nil {
			return "", 0, err
		}

		return base + "[" + rn + "]", value, nil

	case 0x06: // Register deferred: (Rn)
		return "(" + rn + ")", uint32(reg), nil

	case 0x07: // Autodecrement: -(Rn)
		return "-(" + rn + ")", uint32(reg), nil

	case 0x08: // Autoincrement: (Rn)+
		return "(" + rn + ")+", uint32(reg), nil

	case 0x09: // Autoincrement deferred: @(Rn)+
		return "@(" + rn + ")+", uint32(reg), nil

	case 0x0A, 0x0B: // Byte displacement [deferred]: B^n(Rn) / @B^n(Rn)
		raw := loadSized(r, *pc, 1)
		*pc++

		return formatDisplacement("B^", raw, 1, rn, mode == 0x0B), raw, nil

	case 0x0C, 0x0D: // Word displacement [deferred]
		raw := loadSized(r, *pc, 2)
		*pc += 2

		return formatDisplacement("W^", raw, 2, rn, mode == 0x0D), raw, nil

	case 0x0E, 0x0F: // Long displacement [deferred]
		raw := loadSized(r, *pc, 4)
		*pc += 4

		return formatDisplacement("L^", raw, 4, rn, mode == 0x0F), raw, nil
	}

	return "", 0, vmserrors.New(vmserrors.VAX_INTERNAL, fmt.Sprintf("unreachable addressing mode %X", mode))
}

func formatDisplacement(prefix string, raw uint32, size int, rn string, deferred bool) string {
	if deferred {
		prefix = "@" + prefix
	}

	return fmt.Sprintf("%s%s(%s)", prefix, formatIntHex(raw, size), rn)
}
