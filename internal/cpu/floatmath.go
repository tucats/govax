package cpu

import "github.com/tucats/govax/internal/vaxfloat"

// ADD, SUB, MUL, and DIV for the floating formats, in their 2- and
// 3-operand forms (originally the port of emul_float_math.c's arithmetic
// paths, for F and D only). Each handler works for any format: the
// operands' data types in the instruction table say which, and the
// floating core (internal/vaxfloat) rounds the exact result once to the
// destination's format.
//
// Condition codes (the manual's ADD, SUB, MUL, and DIV): N and Z from the
// result, V and C cleared. Overflow, divide by zero, and (with PSL<FU>)
// underflow are faults: the destination is unaffected. With FU clear an
// underflowing result is stored as zero.

func init() {
	reg := func(op Opcode, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(op), h)
	}

	for _, fn := range []byte{0x40, 0x41, 0x60, 0x61} { // ADDF2/3, ADDD2/3
		reg(Opcode{Function: fn}, floatArithmetic(vaxfloat.Add))
	}

	for _, fn := range []byte{0x42, 0x43, 0x62, 0x63} { // SUBF2/3, SUBD2/3
		reg(Opcode{Function: fn}, floatArithmetic(vaxfloat.Sub))
	}

	for _, fn := range []byte{0x44, 0x45, 0x64, 0x65} { // MULF2/3, MULD2/3
		reg(Opcode{Function: fn}, floatArithmetic(vaxfloat.Mul))
	}

	for _, fn := range []byte{0x46, 0x47, 0x66, 0x67} { // DIVF2/3, DIVD2/3
		reg(Opcode{Function: fn}, floatArithmetic(vaxfloat.Div))
	}

	// G_floating and H_floating: the F and D opcodes after the FD prefix.
	for fn, op := range map[byte]floatOp{
		0x40: vaxfloat.Add, 0x41: vaxfloat.Add, // ADDG2/3
		0x42: vaxfloat.Sub, 0x43: vaxfloat.Sub, // SUBG2/3
		0x44: vaxfloat.Mul, 0x45: vaxfloat.Mul, // MULG2/3
		0x46: vaxfloat.Div, 0x47: vaxfloat.Div, // DIVG2/3
		0x60: vaxfloat.Add, 0x61: vaxfloat.Add, // ADDH2/3
		0x62: vaxfloat.Sub, 0x63: vaxfloat.Sub, // SUBH2/3
		0x64: vaxfloat.Mul, 0x65: vaxfloat.Mul, // MULH2/3
		0x66: vaxfloat.Div, 0x67: vaxfloat.Div, // DIVH2/3
	} {
		reg(Opcode{Extended: 0xFD, Function: fn}, floatArithmetic(op))
	}
}

// floatOp is one of the floating core's rounded operations.
type floatOp func(f vaxfloat.Format, a, b vaxfloat.Value) (vaxfloat.Value, error)

// floatArithmetic returns the handler for op. The instruction's first
// operand is the one the operation applies (the addend, subtrahend,
// multiplier, or divisor), its second is the other (the 2-operand form's
// sum, difference, product, or quotient, which it also replaces), and the
// last operand is the destination. So the result is op(second, first): SUBF3
// sub,min,dif stores min - sub, and DIVF2 divr,quo stores quo / divr.
func floatArithmetic(op floatOp) Handler {
	return func(e *Engine, d *Decoded) error {
		first, err := e.loadFloat(d, 0)
		if err != nil {
			return err
		}

		second, err := e.loadFloat(d, 1)
		if err != nil {
			return err
		}

		dst := d.Instruction.OperandCount - 1

		result, err := op(operandFormat(d, dst), second, first)
		if err = e.floatException(err); err != nil {
			return err
		}

		setFloatCC(e.cpu, result, false)

		return e.storeFloat(d, dst, result)
	}
}
