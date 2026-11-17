package vaxfloat

import "math/big"

// Fortran 90's model of a real number (the standard's 13.7.1) is
//
//	x = s × b**e × (f1 × b**-1 + f2 × b**-2 + ... + fp × b**-p)
//
// with radix b, p digits, f1 not 0, and an exponent e from emin to emax. Each
// VAX format is such a model exactly with b = 2: its hidden bit is f1, so a
// value is 0.1fff... × 2**e, and the exponent field's bias makes e run from
// 1 - bias to the largest field less the bias. The model's numbers are the
// format's numbers, so the inquiry functions FORTRAN asks of a kind (DIGITS,
// MINEXPONENT, MAXEXPONENT, HUGE, TINY, EPSILON) have exact answers here.

// Digits is the model's p, the significant bits counting the hidden one:
// 24, 56, 53, or 113 (Fortran 90's DIGITS).
func (f Format) Digits() int { return int(f.Precision()) }

// MinExponent is the model's emin, the least exponent e of a value
// 0.1fff... × 2**e: -127, -127, -1023, or -16383 (MINEXPONENT).
func (f Format) MinExponent() int { return f.minExp() }

// MaxExponent is the model's emax: 127, 127, 1023, or 16383 (MAXEXPONENT).
func (f Format) MaxExponent() int { return f.maxExp() }

// Huge is the format's largest value, (1 - 2**-p) × 2**emax: every
// fraction bit set and the largest exponent (HUGE).
func (f Format) Huge() Value {
	p := f.Digits()

	// 1 - 2**-p exactly, at p bits. (SetMantExp would take its operand's
	// precision, 53 bits, so the 1 is set as an integer.)
	x := new(big.Float).SetPrec(uint(p)).SetInt64(1)
	x.Sub(x, new(big.Float).SetMantExp(big.NewFloat(1), -p))

	return newValue(x.SetMantExp(x, f.maxExp()))
}

// Tiny is the format's least positive value, 2**(emin - 1): a fraction of
// 0.1 and the least exponent (TINY). The VAX has no denormalized numbers.
func (f Format) Tiny() Value {
	return newValue(new(big.Float).SetMantExp(big.NewFloat(1), f.minExp()-1))
}

// Epsilon is 2**(1 - p), the difference between 1 and the next value above
// it in the format (EPSILON).
func (f Format) Epsilon() Value {
	return newValue(new(big.Float).SetMantExp(big.NewFloat(1), 1-f.Digits()))
}
