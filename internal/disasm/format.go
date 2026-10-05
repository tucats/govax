package disasm

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vaxfloat"
)

// String renders dec in the syntax internal/asm's Assemble can parse back
// in: "MNEMONIC OP1,OP2,...". This is what makes the round-trip property
// in docs/PHASE-11.md's deliverables checkable: assemble a fixture,
// disassemble each instruction, reassemble the disassembly, and compare
// bytes. An entry mask is ".ENTRY NAME,^M<...>".
func (dec Decoded) String() string {
	if dec.IsMask {
		if dec.Name == "" {
			return ".ENTRY " + FormatMask(dec.Mask)
		}

		return ".ENTRY " + dec.Name + "," + FormatMask(dec.Mask)
	}

	var b strings.Builder

	b.WriteString(dec.Mnemonic)

	for i, op := range dec.Operands {
		if i == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteByte(',')
		}

		b.WriteString(op.String())
	}

	return b.String()
}

// String renders one operand as internal/asm writes it: hexadecimal
// numbers with the ^X radix operator (the assembler's default radix is
// decimal, as in MACRO-32), a short literal in decimal, and Symbol in
// place of Target's number when the caller has set one.
func (op Operand) String() string {
	text := op.baseString()

	if op.Indexed() {
		text += "[" + cpu.RegisterName(op.Index) + "]"
	}

	return text
}

// baseString renders the operand without its index register.
func (op Operand) baseString() string {
	rn := ""
	if op.Register >= 0 {
		rn = cpu.RegisterName(op.Register)
	}

	switch op.Mode {
	case ModeInline:
		return "#" + formatIntHex(op.Value, op.Width)

	case ModeBranch:
		return op.target()

	case ModeLiteral:
		if op.Symbol != "" {
			return "S^#" + op.Symbol
		}

		if op.Type.IsFloat() {
			return "S^#" + vaxfloat.ShortLiteral(byte(op.Value)).Decimal()
		}

		// A short literal (0-63) is shown in decimal, as MACRO-32 writes
		// it.
		return fmt.Sprintf("S^#%d", op.Value)

	case ModeRegister:
		return rn

	case ModeRegisterDeferred:
		return "(" + rn + ")"

	case ModeAutodecrement:
		return "-(" + rn + ")"

	case ModeAutoincrement:
		return "(" + rn + ")+"

	case ModeAutoincrementDeferred:
		return "@(" + rn + ")+"

	case ModeDisplacement:
		raw := uint32(op.Displacement) & widthMask(op.Width)

		return op.deferral() + widthPrefix(op.Width) + formatIntHex(raw, op.Width) + "(" + rn + ")"

	case ModeImmediate:
		if op.Symbol != "" {
			return "I^#" + op.Symbol
		}

		return "I^#" + op.immediate()

	case ModeAbsolute:
		return "@#" + op.target()

	case ModeRelative:
		if op.Cell != "" {
			return "G^" + op.Cell
		}

		return op.deferral() + widthPrefix(op.Width) + op.target()
	}

	return "?"
}

// target is the operand's Target as text: its Symbol if the caller set
// one, or the address in hexadecimal.
func (op Operand) target() string {
	if op.Symbol != "" {
		return op.Symbol
	}

	return formatIntHex(op.Target, 4)
}

// deferral is "@" for a deferred operand, and "" otherwise.
func (op Operand) deferral() string {
	if op.Deferred {
		return "@"
	}

	return ""
}

// immediate is an immediate operand's value as text. A floating value is
// shown in its own format, with the fewest digits that reassemble to the
// same bits (a reserved operand or a "dirty zero" has no decimal form,
// and shows as 0). A quadword or octaword shows every byte, so the text
// reassembles to the same bytes. Anything else is hexadecimal.
func (op Operand) immediate() string {
	if op.Type.IsFloat() {
		v, _ := vaxfloat.Unpack(op.Type.FloatFormat(), op.floatBits())

		return v.Decimal()
	}

	if op.Width >= 8 {
		return formatWideHex(op.Bytes)
	}

	return formatIntHex(op.Value, op.Width)
}

// widthPrefix is the displacement size prefix MACRO writes: B^, W^, or
// L^.
func widthPrefix(width int) string {
	switch width {
	case 1:
		return "B^"
	case 2:
		return "W^"
	default:
		return "L^"
	}
}

// widthMask is the mask of a width-byte value's bits.
func widthMask(width int) uint32 {
	switch width {
	case 1:
		return 0xFF
	case 2:
		return 0xFFFF
	default:
		return 0xFFFFFFFF
	}
}

// FormatMask renders a 16-bit register-set mask as "^M<...>" text,
// matching console_disasm.c's format_mask(): a routine's entry mask (see
// EntryMask). Bit 15 is IV (integer overflow trap enable), bit 14 is DV
// (decimal overflow trap enable); bits 0-13 print as "R<n>", matching the
// reference tool's own formatting even though only R0-R11 are ever
// settable through internal/asm's mask parser.
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

// formatIntHex formats v in hexadecimal, zero-padded to size bytes, with
// the ^X radix operator the assembler needs to read it back (its default
// radix is decimal, as in MACRO-32), so disassembly can be reassembled.
func formatIntHex(v uint32, size int) string {
	switch size {
	case 1:
		return fmt.Sprintf("^X%02X", v)

	case 2:
		return fmt.Sprintf("^X%04X", v)

	default:
		return fmt.Sprintf("^X%08X", v)
	}
}

// formatWideHex formats b (8 or 16 bytes, a quadword or octaword) as one
// hexadecimal number with the ^X radix operator, every digit shown.
// Memory holds the value low-order byte first, so the bytes are printed
// from the last to the first to put the most significant digits on the
// left.
func formatWideHex(b []byte) string {
	var s strings.Builder

	s.WriteString("^X")

	for i := len(b) - 1; i >= 0; i-- {
		fmt.Fprintf(&s, "%02X", b[i])
	}

	return s.String()
}
