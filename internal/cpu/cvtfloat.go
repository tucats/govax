package cpu

import (
	"math/big"

	"github.com/tucats/govax/internal/vaxfloat"
)

// CVT between the floating formats and integers, and between floating
// formats (originally the port of emul_float_math.c's conversion paths,
// for F and D). Every handler works for any format: the operands' data
// types say which.
//
// The manual's CVT: N and Z from the destination, V set on integer
// overflow, C cleared. A conversion to an integer truncates toward zero
// (CVTRxL rounds, half away from zero); on overflow the destination gets
// the low-order bits of the true result, V is set, and, if PSL<IV> is set,
// an integer overflow trap follows. A conversion to a floating format is
// exact or rounded as the formats allow; one that narrows can overflow (a
// fault) or underflow (zero, or a fault with PSL<FU>).

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}

	for _, fn := range []byte{0x48, 0x49, 0x4A, 0x68, 0x69, 0x6A} { // CVTFB/W/L, CVTDB/W/L
		reg(fn, emulCvtFloatToInt)
	}

	for _, fn := range []byte{0x4B, 0x6B} { // CVTRFL, CVTRDL
		reg(fn, emulCvtRoundFloatToInt)
	}

	for _, fn := range []byte{0x4C, 0x4D, 0x4E, 0x6C, 0x6D, 0x6E} { // CVTBF/W/L, CVTBD/W/L
		reg(fn, emulCvtIntToFloat)
	}

	reg(0x56, emulCvtFloatToFloat) // CVTFD
	reg(0x76, emulCvtFloatToFloat) // CVTDF

	// G_floating and H_floating.
	for _, fn := range []byte{0x48, 0x49, 0x4A, 0x68, 0x69, 0x6A} { // CVTGB/W/L, CVTHB/W/L
		regFD(fn, emulCvtFloatToInt)
	}

	regFD(0x4B, emulCvtRoundFloatToInt) // CVTRGL
	regFD(0x6B, emulCvtRoundFloatToInt) // CVTRHL

	for _, fn := range []byte{0x4C, 0x4D, 0x4E, 0x6C, 0x6D, 0x6E} { // CVTBG/WG/LG, CVTBH/WH/LH
		regFD(fn, emulCvtIntToFloat)
	}

	for _, fn := range []byte{
		0x33, // CVTGF
		0x99, // CVTFG
		0x98, // CVTFH
		0xF6, // CVTHF
		0x32, // CVTDH
		0xF7, // CVTHD
		0x56, // CVTGH
		0x76, // CVTHG
	} {
		regFD(fn, emulCvtFloatToFloat)
	}
}

// cvtFloatToInt is CVTxB/W/L and CVTRxL's shared body.
func cvtFloatToInt(e *Engine, d *Decoded, round bool) error {
	value, err := e.loadFloat(d, 0)
	if err != nil {
		return err
	}

	dst := d.Operands[1]
	exact := value.Int(round)
	result, overflow := lowOrderBits(exact, int(dst.Size))

	setArithPSL(e.cpu, result, overflow, false, int(dst.Size))

	if err := dst.Store(e.cpu, e.mem, result); err != nil {
		return err
	}

	if overflow && e.cpu.PSL().IV() {
		return e.arithmeticTrap(trapIntOvf)
	}

	return nil
}

// lowOrderBits returns the low size bytes of the two's complement integer
// i, and whether i is outside the signed range of that size (an integer
// overflow).
func lowOrderBits(i *big.Int, size int) (uint64, bool) {
	bits := uint(8 * size)
	limit := new(big.Int).Lsh(big.NewInt(1), bits-1)
	overflow := i.Cmp(limit) >= 0 || i.Cmp(new(big.Int).Neg(limit)) < 0

	// And with a mask works on a negative big.Int as on its infinite
	// two's complement form, so this is the low-order bits either way.
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), bits), big.NewInt(1))

	return new(big.Int).And(i, mask).Uint64(), overflow
}

// emulCvtFloatToInt is CVTxB, CVTxW, and CVTxL: truncate toward zero.
func emulCvtFloatToInt(e *Engine, d *Decoded) error {
	return cvtFloatToInt(e, d, false)
}

// emulCvtRoundFloatToInt is CVTRxL: round to the nearest integer, half
// away from zero.
func emulCvtRoundFloatToInt(e *Engine, d *Decoded) error {
	return cvtFloatToInt(e, d, true)
}

// emulCvtIntToFloat is CVTBx, CVTWx, and CVTLx: the sign-extended integer,
// rounded to the destination's format (only CVTLF can need rounding: a
// longword has more bits than F_floating's 24).
func emulCvtIntToFloat(e *Engine, d *Decoded) error {
	src := d.Operands[0]

	raw, err := src.Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	result, err := e.roundFloat(d, 1, vaxfloat.FromInt(signExtend(raw, int(src.Size))))
	if err != nil {
		return err
	}

	setFloatCC(e.cpu, result, false)

	return e.storeFloat(d, 1, result)
}

// emulCvtFloatToFloat is a conversion between floating formats. Widening
// ones are exact (CVTFD, CVTFG, CVTFH, CVTDH, CVTGH). Narrowing ones round
// half away from zero, and can overflow (CVTDF, when a value just under
// 2^127 rounds up, and CVTGF, CVTHF, CVTHD, and CVTHG, from a wider
// exponent range) or underflow (CVTGF, CVTHF, CVTHD, and CVTHG).
func emulCvtFloatToFloat(e *Engine, d *Decoded) error {
	value, err := e.loadFloat(d, 0)
	if err != nil {
		return err
	}

	result, err := e.roundFloat(d, 1, value)
	if err != nil {
		return err
	}

	setFloatCC(e.cpu, result, false)

	return e.storeFloat(d, 1, result)
}
