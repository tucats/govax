package vaxfloat

import "math/big"

// EMOD (Extended Multiply and Integerize) multiplies with a few more bits
// of multiplier than the format holds, and splits the product into an
// integer and a fraction: the first step of argument reduction for the
// trigonometric and exponential functions, where the fraction part must
// keep every bit it can.
//
// From the VAX Architecture Reference Manual's EMOD:
//
//   - The multiplier extension operand is concatenated with the multiplier
//     as further low-order fraction bits: all 8 bits of a byte for F and D,
//     the high 11 of a word for G (its low 5 are ignored), the high 15 for
//     H (its low bit is ignored).
//   - The product of the extended multiplier and the multiplicand is the
//     exact product truncated, before normalization, to a fraction of 32
//     bits for F, 64 for D and G, and 128 for H.
//   - That product is the sum of an integer and a fraction of the same
//     sign. The fraction is rounded to the format; because it is rounded
//     after the integer part is taken out, it can be 1.

// emodBits returns format f's multiplier extension width and the width
// its product is truncated to.
func emodBits(f Format) (ext, product int) {
	switch f {
	case F:
		return 8, 32
	case D:
		return 8, 64
	case G:
		return 11, 64
	}

	return 15, 128
}

// emodExtension returns the extension bits of operand ext in format f:
// the byte for F and D, or the high bits of the word for G and H.
func emodExtension(f Format, ext uint16) uint64 {
	switch f {
	case F, D:
		return uint64(ext & 0xFF)
	case G:
		return uint64(ext >> 5)
	}

	return uint64(ext >> 1)
}

// EMOD returns EMOD's results for multiplier mulr extended by ext,
// times multiplicand muld, in format f: the integer part, exact (the
// caller keeps its low 32 bits and checks it for overflow), and the
// fraction part, rounded to f. If the fraction underflows it returns
// ErrUnderflow with zero for both, as the instruction stores them when
// PSL<FU> is clear.
func EMOD(f Format, mulr Value, ext uint16, muld Value) (*big.Int, Value, error) {
	product := emodProduct(f, mulr, ext, muld)
	integer, fraction := product.Split()

	fract, err := Round(f, fraction)
	if err != nil {
		return new(big.Int), Value{}, err
	}

	return integer, fract, nil
}

// emodProduct returns the extended, truncated product.
func emodProduct(f Format, mulr Value, ext uint16, muld Value) Value {
	if mulr.IsZero() || muld.IsZero() {
		return Value{}
	}

	extBits, width := emodBits(f)
	p := int(f.Precision())

	// Each value is m * 2^e, 0.5 <= |m| < 1. The extended multiplier's
	// fraction is m1 plus the extension bits just below m1's last bit.
	m1 := new(big.Float)
	e1 := mulr.x.MantExp(m1)
	m1.Abs(m1)

	extension := new(big.Float).SetUint64(emodExtension(f, ext))
	extension.SetMantExp(extension, -(p + extBits))

	f1 := new(big.Float).SetPrec(uint(p + extBits)).Add(m1, extension)

	m2 := new(big.Float)
	e2 := muld.x.MantExp(m2)
	m2.Abs(m2)

	// The product of the fractions is in [0.25, 1). Truncate it, as it
	// stands (before normalizing it), to width bits after the binary
	// point.
	prod := new(big.Float).SetPrec(uint(2*p + extBits)).Mul(f1, m2)
	prod.SetMantExp(prod, width)

	truncated, _ := prod.Int(nil)

	result := new(big.Float).SetPrec(uint(max(truncated.BitLen(), 1))).SetInt(truncated)
	result.SetMantExp(result, e1+e2-width)

	if mulr.Sign() != muld.Sign() {
		result.Neg(result)
	}

	return newValue(result)
}
