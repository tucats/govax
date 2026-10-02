package vaxfloat

import "math/big"

// POLY evaluates a polynomial by Horner's method, one step per
// coefficient. The VAX Architecture Reference Manual (1987, EY-3459E-DP)
// gives each step's arithmetic in its POLY "Operation":
//
//	tmp4 <- arg * tmp3    ! keep the 31 (F), 63 (D, G), or 127 (H) most
//	                      ! significant bits of the product's fraction,
//	                      ! truncating the unnormalized product
//	tmp4 <- tmp4 + coef   ! align, add, and keep 31, 63, or 127 bits,
//	                      ! truncating the unnormalized sum; then
//	                      ! normalize and round to the format
//
// with overflow and underflow checked only after the whole step. POLYStep
// is one such step; the CPU walks the table (internal/cpu/poly.go).

// polyBits returns the fraction width format f's POLY keeps between its
// multiply and add.
func polyBits(f Format) int {
	switch f {
	case F:
		return 31
	case D, G:
		return 63
	}

	return 127
}

// POLYStep returns arg * acc + coef as POLY computes it in format f:
// the product and the sum each truncated, unnormalized, to the format's
// POLY width, then rounded to f. It returns ErrOverflow, or ErrUnderflow
// with zero, as Round does.
func POLYStep(f Format, arg, acc, coef Value) (Value, error) {
	n := polyBits(f)

	// The product, truncated: its fraction (the product of the two
	// operands' fractions, in [0.25, 1)) cut to n bits, relative to
	// 2^(ea+et).
	var (
		product    *big.Float
		productExp int
	)

	if !arg.IsZero() && !acc.IsZero() {
		fa, fb := new(big.Float), new(big.Float)
		ea := arg.x.MantExp(fa)
		eb := acc.x.MantExp(fb)

		frac := new(big.Float).SetPrec(fa.MinPrec() + fb.MinPrec()).Mul(fa, fb)
		product = truncateFraction(frac, n)
		productExp = ea + eb
		product.SetMantExp(product, productExp)
	}

	// The sum, truncated: aligned to the larger of the two exponents (the
	// product's unnormalized one, or the coefficient's), and cut to n
	// bits after that binary point.
	var sum *big.Float

	switch {
	case product == nil && coef.IsZero():
		return Value{}, nil
	case product == nil:
		sum = new(big.Float).Set(coef.x)
		return Round(f, newValue(sum))
	case coef.IsZero():
		sum = product
	default:
		ec := coef.x.MantExp(nil)
		top := max(productExp, ec)

		exact := AddExact(newValue(product), coef)
		if exact.IsZero() {
			return Value{}, nil
		}

		s := new(big.Float).SetMantExp(exact.x, -top) // |s| < 2
		sum = truncateFraction(s, n)
		sum.SetMantExp(sum, top)
	}

	return Round(f, newValue(sum))
}

// truncateFraction returns x with its bits below 2^-n dropped (toward
// zero).
func truncateFraction(x *big.Float, n int) *big.Float {
	scaled := new(big.Float).SetPrec(x.MinPrec() + uint(n) + 2).SetMantExp(x, n)
	i, _ := scaled.Int(nil)

	r := new(big.Float).SetPrec(uint(max(i.BitLen(), 1))).SetInt(i)

	return r.SetMantExp(r, -n)
}
