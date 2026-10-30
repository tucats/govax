package mthrtl

import (
	"math/big"
)

// The hyperbolic functions on arbitrary-precision numbers, in the engine's way
// (engine.go): each takes a precision, works above it, and leaves the one
// rounding to the routine that called it.

// sinhCosh returns the hyperbolic sine and cosine of x, to prec bits. They're
//
//	sinh x = (e^x - e^-x) / 2
//	cosh x = (e^x + e^-x) / 2
//
// and for |x| of at least 1 that's how they're computed, on |x|: the two
// exponentials differ by a factor of at least e^2, so the difference cancels
// less than a bit. Below 1 it would cancel more the smaller x is, down to
// everything, so there they come from their Taylor series, as a sine and cosine
// do but with every term added. The manual's other cases (1 or x for a tiny x,
// a single exponential for a large one) are what these come to when rounded.
//
// An x beyond every format's limit gets exponential's value beyond every format's
// range, so the sum and difference are too, and the rounding reports it.
func sinhCosh(x *big.Float, prec uint) (sinh, cosh *big.Float) {
	one := big.NewFloat(1)
	magnitude := new(big.Float).Abs(x)

	if magnitude.Cmp(one) < 0 {
		s, c := factorialSeries(x, false, prec)

		return newFloat(prec).Set(s), newFloat(prec).Set(c)
	}

	w := prec + scratchBits
	e := exponential(magnitude, w)
	inverse := newFloat(w).Quo(one, e)

	sinh = newFloat(w).Sub(e, inverse)
	cosh = newFloat(w).Add(e, inverse)

	// A hyperbolic sine is odd and a cosine even, as the circular ones are.
	if x.Signbit() {
		sinh.Neg(sinh)
	}

	// SetMantExp(z, -1) is z / 2, exactly.
	sinh.SetMantExp(sinh, -1)
	cosh.SetMantExp(cosh, -1)

	return newFloat(prec).Set(sinh), newFloat(prec).Set(cosh)
}

// arcTanh returns the number whose hyperbolic tangent is x, to prec bits, for
// |x| below 1: the manual's
//
//	atanh x = ln((1 + x) / (1 - x)) / 2
//
// For |x| below 1/8 the ratio is so near 1 that it's better to sum atanhSeries,
// which gains at least 6 bits a term and whose first term is x itself, so the
// result is as good however small x is. Elsewhere the logarithm is well away from
// 0. And 1 - x is exact however close x comes to 1, since x is at most a format's
// length and the working precision is longer than that.
func arcTanh(x *big.Float, prec uint) *big.Float {
	w := prec + scratchBits

	// MantExp gives x's exponent e, with |x| below 2^e and at least 2^(e-1).
	if x.MantExp(nil) <= -3 {
		return newFloat(prec).Set(atanhSeries(x, w))
	}

	one := big.NewFloat(1)
	ratio := newFloat(w).Add(one, x)
	ratio.Quo(ratio, newFloat(w).Sub(one, x))

	result := logarithm(ratio, prec)

	return result.SetMantExp(result, -1)
}
