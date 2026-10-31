package vaxfloat

import (
	"errors"
	"math/big"
)

// The arithmetic that a FORTRAN compiler folding a constant and the OTS$ routines
// at run time must do alike, so a constant expression has the value the same
// expression would compute. It's here, the one package both use, so that they can't
// come to differ.

// PowerBySquaring returns base ** n, for any kind of number: the product of
// base ** (2 ** i) for each bit i set in |n|, each product computed by mul, and for a
// negative n the product of those squares' reciprocals, each by div. one is the type's
// 1.0, which is also the answer when n is 0.
//
// The order matters. A real or complex mul rounds, so each step's error is in the
// result. The OTS$ manual's tables read as 1.0 over the whole product, but VMS 7.1
// (testdata/mth77) has 2.0 ** (-127) in F_floating, whose product doesn't fit, and this
// order is the one that comes closest to its bits; they aren't always the same, and
// aren't a goal. A base is squared only as often as a bit of n still needs it, so a
// square that overflows is one the result would contain.
//
// An error from mul or div ends the computation.
func PowerBySquaring[T any](base, one T, n int64, mul, div func(a, b T) (T, error)) (T, error) {
	count := n
	if count < 0 {
		count = -count
	}

	result := one

	var err error

	for count > 0 {
		if count&1 != 0 {
			factor := base

			if n < 0 {
				if factor, err = div(one, base); err != nil {
					return result, err
				}
			}

			if result, err = mul(result, factor); err != nil {
				return result, err
			}
		}

		if count >>= 1; count > 0 {
			if base, err = mul(base, base); err != nil {
				return result, err
			}
		}
	}

	return result, nil
}

// ComplexMul returns (a, b) * (c, d) in format f: (ac - bd, ad + bc), each part
// computed exactly and rounded once. The VAX has no complex instructions, and VMS's
// code may differ in a part's last bits (testdata/mth77). A
// part too small for f is 0.0. Its only error is ErrOverflow.
func ComplexMul(f Format, a, b, c, d Value) (Value, Value, error) {
	return roundParts(f,
		AddExact(MulExact(a, c), MulExact(b, d).Neg()),
		AddExact(MulExact(a, d), MulExact(b, c)))
}

// ComplexDiv returns (a, b) / (c, d) in format f: the OTS$ manual's
// ((ac + bd) / (c^2 + d^2), (bc - ad) / (c^2 + d^2)), each part computed exactly
// and rounded once, a part too small for f being 0.0. Its errors are ErrOverflow,
// and ErrDivideByZero if c and d are both 0.
func ComplexDiv(f Format, a, b, c, d Value) (Value, Value, error) {
	denominator := AddExact(MulExact(c, c), MulExact(d, d))
	if denominator.IsZero() {
		return Value{}, Value{}, ErrDivideByZero
	}

	quotient := func(n Value) Value {
		return FromRat(new(big.Rat).Quo(n.Rat(), denominator.Rat()))
	}

	return roundParts(f,
		quotient(AddExact(MulExact(a, c), MulExact(b, d))),
		quotient(AddExact(MulExact(b, c), MulExact(a, d).Neg())))
}

// roundParts rounds a complex result's two parts to f, a part too small being 0.0,
// as it is on a VAX with floating underflow faults disabled.
func roundParts(f Format, re, im Value) (Value, Value, error) {
	r, err := roundToZero(f, re)
	if err != nil {
		return r, r, err
	}

	i, err := roundToZero(f, im)

	return r, i, err
}

func roundToZero(f Format, v Value) (Value, error) {
	r, err := Round(f, v)
	if errors.Is(err, ErrUnderflow) {
		return Value{}, nil
	}

	return r, err
}
