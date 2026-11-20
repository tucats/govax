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

// The model's functions of a value (Fortran 90's 13.13.33, .39, .74, .89,
// .94, .99, and .103), exact here because the format is the model. A nonzero
// value v is m × 2**e with 0.5 <= |m| < 1, the m and e big.Float's MantExp
// gives; the exponent field of v's bits is e plus the bias.

// Exponent is the model's e of v (EXPONENT): 0 for 0.
func Exponent(v Value) int {
	if v.IsZero() {
		return 0
	}

	return v.float().MantExp(nil)
}

// Fraction is v × 2**-e, its fraction with its sign (FRACTION): v's bits
// with the exponent field the bias's. 0 for 0.
func Fraction(v Value) Value {
	if v.IsZero() {
		return v
	}

	m := new(big.Float)
	v.float().MantExp(m)

	return newValue(m)
}

// Scale is v × 2**i in format f (SCALE): v's bits with i added to the
// exponent field. A result past the format's largest exponent is
// ErrOverflow; one below its least is 0, as the VAX's underflow is with
// PSL<FU> clear.
func (f Format) Scale(v Value, i int64) (Value, error) {
	if v.IsZero() {
		return v, nil
	}

	e := int64(Exponent(v)) + i

	switch {
	case e > int64(f.maxExp()):
		return Value{}, ErrOverflow
	case e < int64(f.minExp()):
		return Value{}, nil
	}

	return newValue(new(big.Float).SetMantExp(v.float(), int(i))), nil
}

// SetExponent is v's fraction × 2**i in format f (SET_EXPONENT): v's bits
// with the exponent field the bias plus i. Out of range as Scale's.
func (f Format) SetExponent(v Value, i int64) (Value, error) {
	return f.Scale(Fraction(v), i)
}

// Spacing is the distance between v and its neighbors in format f, 2**(e -
// p) (SPACING): one unit in the last place of v's fraction. For 0, or where
// that is below the format's range, it is TINY.
func (f Format) Spacing(v Value) Value {
	if v.IsZero() {
		return f.Tiny()
	}

	// 2**(e - p) is 0.1 × 2**(e - p + 1), whose exponent must be in range.
	e := Exponent(v) - f.Digits()
	if e+1 < f.minExp() {
		return f.Tiny()
	}

	return newValue(new(big.Float).SetMantExp(big.NewFloat(1), e))
}

// RRSpacing is the reciprocal of v's relative spacing in format f, |v ×
// 2**-e| × 2**p (RRSPACING): v's fraction bits as a whole number, the
// exponent field the bias plus p and the sign clear. 0 for 0.
func (f Format) RRSpacing(v Value) Value {
	if v.IsZero() {
		return v
	}

	return newValue(new(big.Float).SetMantExp(Fraction(v).Abs().float(), f.Digits()))
}

// Nearest is the value of format f next to v, above it if up, else below
// (NEAREST, up when S is positive): v's bits, sign apart, as a whole number
// one more (away from 0) or one less (toward it). The next from 0 is ±TINY;
// past HUGE is ErrOverflow; toward 0 from ±TINY is 0.
func (f Format) Nearest(v Value, up bool) (Value, error) {
	if v.IsZero() {
		if up {
			return f.Tiny(), nil
		}

		return f.Tiny().Neg(), nil
	}

	away := (v.Sign() > 0) == up
	e := Exponent(v)
	m := Fraction(v).Abs()
	ulp := new(big.Float).SetMantExp(big.NewFloat(1), e-f.Digits())

	if !away && m.Cmp(FromRat(big.NewRat(1, 2))) == 0 {
		// At a power of 2 the values below are twice as close.
		ulp.SetMantExp(ulp, -1)
	}

	prec := uint(f.Digits() + 2)
	x := new(big.Float).SetPrec(prec).Abs(v.float())

	if away {
		x.Add(x, ulp)
	} else {
		x.Sub(x, ulp)
	}

	switch r := newValue(x); {
	case r.IsZero():
		return r, nil
	case Exponent(r) > f.maxExp():
		return Value{}, ErrOverflow
	case Exponent(r) < f.minExp():
		return Value{}, nil
	default:
		if v.Sign() < 0 {
			return r.Neg(), nil
		}

		return r, nil
	}
}
