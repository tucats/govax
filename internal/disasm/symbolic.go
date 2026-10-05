package disasm

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Symbolizer names addresses: a debugger's symbol table, the console's,
// or a chain of them. internal/dbgsym's Names names them as the VMS
// debugger does, from an image's debug symbol table.
type Symbolizer interface {
	// Symbolize returns the name for addr ("DBGDIS\COUNT",
	// "GLIMIT+24D", "DBGDIS\START\%LINE 82"), or ok false when nothing
	// names it and the operand should show the number.
	Symbolize(addr uint32) (name string, ok bool)
}

// ConstantNamer names a constant: an option beyond what the VMS debugger
// shows (docs/PHASE-41.md, Decision 5), for a short literal's or an
// integer immediate's value.
type ConstantNamer interface {
	// Constant returns the name of the one constant whose value is value
	// in the module that holds pc, the instruction's address; ok is false
	// when there's none, or more than one.
	Constant(pc, value uint32) (name string, ok bool)
}

// Style is how Format writes an instruction.
type Style int

const (
	// StyleAssembler is String's text: what internal/asm reassembles,
	// with numbers in ^X hexadecimal and a short literal in decimal.
	StyleAssembler Style = iota

	// StyleDebugger is the VMS debugger's EXAMINE/INSTRUCTION text
	// (docs/PHASE-41.md, subtask 1's results): the mnemonic padded to 8
	// columns, numbers in hexadecimal without a radix operator, a short
	// literal in two digits (S^#0A), an immediate in as many digits as
	// its size (I^#000003E8, I^#9F16), an address in eight (L^00000200),
	// and a routine's entry mask as "entry mask ^M<R2,R3,R4>".
	StyleDebugger
)

// Options are Format's choices: the style, the symbolizer that names the
// addresses operands refer to (nil for none), and the namer of constants
// (nil, the default, for none: the debugger shows a constant's value).
type Options struct {
	Style      Style
	Symbolizer Symbolizer
	Constants  ConstantNamer
}

// Format renders dec as opts say. An operand that refers to an address
// (Target) and has no Symbol of its own gets the symbolizer's name for
// it. The debugger names no immediate, short literal, or register
// displacement (a constant's value stays a number: MOVL #LIMIT,R2 is
// MOVL S^#0A,R2); with Constants set, a short literal or integer
// immediate that's exactly one constant's value is shown by its name
// (MOVL S^#DBGDIS\LIMIT,R2). With StyleAssembler and neither namer,
// Format is String.
func (dec Decoded) Format(opts Options) string {
	if opts.Symbolizer != nil || opts.Constants != nil {
		dec = dec.symbolized(opts)
	}

	if opts.Style != StyleDebugger {
		return dec.String()
	}

	if dec.IsMask {
		return "entry mask " + debuggerMask(dec.Mask)
	}

	var b strings.Builder

	// The mnemonic is left-justified in 8 columns, then a space, then the
	// operands: "MOVL     S^#0A,R2". With no operands the line ends in
	// the padding ("RET     "), as the debugger's does.
	mnemonic := dec.Mnemonic
	if alt, ok := debuggerMnemonics[mnemonic]; ok {
		mnemonic = alt
	}

	fmt.Fprintf(&b, "%-8s", mnemonic)

	for i, op := range dec.Operands {
		if i == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteByte(',')
		}

		b.WriteString(op.debuggerString())
	}

	return b.String()
}

// debuggerMnemonics are the opcodes the debugger names otherwise than
// internal/cpu's table: 1E is BCC to the table and BGEQU to the debugger
// (FORTH's BGEQU FORTH\%LINE 1280), and so, by the same pairing, 1F's
// BCS is BLSSU (unconfirmed: no probe line has one).
var debuggerMnemonics = map[string]string{
	"BCC": "BGEQU",
	"BCS": "BLSSU",
}

// debuggerMask renders a register-save mask as the debugger does: the
// registers, then IV and DV, in that order (^M<R2,...,R11,IV,DV>), where
// FormatMask goes by bit number and puts DV first.
func debuggerMask(mask uint16) string {
	var names []string

	for n := 0; n < 14; n++ {
		if mask&(1<<uint(n)) != 0 {
			names = append(names, fmt.Sprintf("R%d", n))
		}
	}

	if mask&0x8000 != 0 {
		names = append(names, "IV")
	}

	if mask&0x4000 != 0 {
		names = append(names, "DV")
	}

	return "^M<" + strings.Join(names, ",") + ">"
}

// symbolized returns a copy of dec with operands' Symbols filled in as
// opts says: an addressed operand's from the symbolizer, a constant's
// from the constant namer. An operand that already has one keeps it. The
// copy has its own operand slice, so the caller's Decoded is unchanged.
func (dec Decoded) symbolized(opts Options) Decoded {
	ops := make([]Operand, len(dec.Operands))
	copy(ops, dec.Operands)

	for i := range ops {
		op := &ops[i]

		switch {
		case op.Symbol != "":
			continue

		case op.HasTarget && opts.Symbolizer != nil:
			if name, ok := opts.Symbolizer.Symbolize(op.Target); ok {
				op.Symbol = name
			}

		case opts.Constants != nil && op.isConstant():
			if name, ok := opts.Constants.Constant(dec.Address, op.Value); ok {
				op.Symbol = name
			}
		}
	}

	dec.Operands = ops

	return dec
}

// isConstant reports whether the operand is a value a constant could
// name: a short literal or an immediate, of an integer type no wider
// than a longword (Value holds it whole).
func (op Operand) isConstant() bool {
	return (op.Mode == ModeLiteral || (op.Mode == ModeImmediate && op.Width <= 4)) && !op.Type.IsFloat()
}

// debuggerString renders one operand in StyleDebugger.
func (op Operand) debuggerString() string {
	text := op.debuggerBase()

	if op.Indexed() {
		text += "[" + cpu.RegisterName(op.Index) + "]"
	}

	return text
}

// debuggerBase renders the operand without its index register, as the
// debugger writes it.
//
// Unconfirmed (no probe line shows them): a negative register
// displacement is shown as its raw bytes (B^FC(FP)), XFC's and BUGx's
// inline data as #value, and floating values other than F_floating short
// literals with the significant digits of their format.
func (op Operand) debuggerBase() string {
	rn := ""
	if op.Register >= 0 {
		rn = cpu.RegisterName(op.Register)
	}

	switch op.Mode {
	case ModeInline:
		return "#" + hexDigits(op.Value, op.Width)

	case ModeBranch:
		return op.debuggerTarget()

	case ModeLiteral:
		if op.Symbol != "" {
			return "S^#" + op.Symbol
		}

		if op.Type.IsFloat() {
			return "S^#" + debuggerFloat(op.Type.FloatFormat(), vaxfloat.ShortLiteral(byte(op.Value)))
		}

		return "S^#" + hexDigits(op.Value, 1)

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

		return op.deferral() + widthPrefix(op.Width) + hexDigits(raw, op.Width) + "(" + rn + ")"

	case ModeImmediate:
		if op.Symbol != "" {
			return "I^#" + op.Symbol
		}

		if op.Type.IsFloat() {
			v, _ := vaxfloat.Unpack(op.Type.FloatFormat(), op.floatBits())

			return "I^#" + debuggerFloat(op.Type.FloatFormat(), v)
		}

		if op.Width > 4 {
			return "I^#" + strings.TrimPrefix(formatWideHex(op.Bytes), "^X")
		}

		return "I^#" + hexDigits(op.Value, op.Width)

	case ModeAbsolute:
		return "@#" + op.debuggerTarget()

	case ModeRelative:
		return op.deferral() + widthPrefix(op.Width) + op.debuggerTarget()
	}

	return "?"
}

// debuggerTarget is the operand's Target as the debugger writes it: its
// Symbol, or the address in eight hexadecimal digits, whatever the
// displacement's width (W^00000200).
func (op Operand) debuggerTarget() string {
	if op.Symbol != "" {
		return op.Symbol
	}

	return hexDigits(op.Target, 4)
}

// floatBits gathers an immediate floating operand's bytes into the bits
// vaxfloat.Unpack reads.
func (op Operand) floatBits() vaxfloat.Bits {
	var bits vaxfloat.Bits

	r := SliceReader(op.Bytes)
	bits.Lo = uint64(loadSized(r, 0, 4))

	if op.Width >= 8 {
		bits.Lo |= uint64(loadSized(r, 4, 4)) << 32
	}

	if op.Width == 16 {
		bits.Hi = uint64(loadSized(r, 8, 4)) | uint64(loadSized(r, 12, 4))<<32
	}

	return bits
}

// hexDigits formats v in hexadecimal, zero-padded to two digits per
// byte of size, with no radix operator.
func hexDigits(v uint32, size int) string {
	return fmt.Sprintf("%0*X", 2*size, v)
}

// debuggerFloat writes a floating value with the significant digits the
// debugger shows for its format, trailing zeros kept: F_floating's 7
// (S^#1.500000, as the probe shows), D_floating's 16, G_floating's 15.
// H_floating's 33 are more than a float64 holds, so it gets 16
// (unconfirmed).
func debuggerFloat(f vaxfloat.Format, v vaxfloat.Value) string {
	digits := 7

	switch f {
	case vaxfloat.D:
		digits = 16
	case vaxfloat.G:
		digits = 15
	case vaxfloat.H:
		digits = 16
	}

	return fmt.Sprintf("%#.*g", digits, v.Float64())
}
