package mthrtl

import (
	"errors"
	"math/big"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The manual's Table 1-1 (section 1.6, and Appendix A for the arguments) is the
// routines too simple for a page of their own, kept mostly so that a FORTRAN
// program can pass an intrinsic function as an argument; a compiler otherwise
// does them in line. The table gives no algorithms, only footnotes saying what
// can go wrong, so what the routines compute comes from their names and the
// VAX FORTRAN manual's Table D-3, and every result here is the exact one, rounded
// once. The 5.0 and 7.3 manuals' footnotes disagree in places (MTH$DIM's
// underflow, MTH$AMOD's); these follow 7.3's, the nearer to VMS 7.1.

// absolute is MTH$ABS and its kind. It can't fail, a value's negative being in the
// format too. As with every routine, a reserved operand has already faulted.
func absolute(_ vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
	return x.Abs(), nil
}

// positiveDifference is MTH$DIM and its kind: a - b if a is the larger, else 0.
// The footnote gives it floating overflow, from subtracting opposite signs.
func positiveDifference(f vaxfloat.Format, a, b vaxfloat.Value) (vaxfloat.Value, error) {
	if a.Cmp(b) <= 0 {
		return vaxfloat.Value{}, nil
	}

	return trapping(noUnderflow(vaxfloat.Sub(f, a, b)))
}

// transferSign is MTH$SIGN and its kind: x's size with y's sign. A VAX zero has no
// sign, and counts as positive, as Table D-3 of the FORTRAN manual has it.
func transferSign(_ vaxfloat.Format, x, y vaxfloat.Value) (vaxfloat.Value, error) {
	if y.Sign() < 0 {
		return x.Abs().Neg(), nil
	}

	return x.Abs(), nil
}

// Three ways to make a whole number of a value, shared by the routines with
// floating results (MTH$FLOOR, MTH$AINT, MTH$ANINT) and those with integer ones
// (MTH$JINT, MTH$JNINT). Each is exact, however large the value, and its answer is
// in the value's format: a whole part has no more bits than the value, and rounding
// it up by one either stays within them or carries to a power of 2.

// truncated drops the fraction: toward zero.
func truncated(x vaxfloat.Value) *big.Int { return x.Int(false) }

// nearest rounds to the nearest whole number, a half going away from zero. The
// FORTRAN manual's INT(a + .5) would round the sum first, and so call the value
// just below 0.5 one; this gives the true nearest, which is 0.
func nearest(x vaxfloat.Value) *big.Int { return x.Int(true) }

// floorOf is the greatest whole number not above x: toward minus infinity.
func floorOf(x vaxfloat.Value) *big.Int {
	whole, fraction := x.Split()
	if fraction.Sign() < 0 {
		whole.Sub(whole, big.NewInt(1))
	}

	return whole
}

// wholeNumber makes the function of a routine with a floating result from one of
// the three.
func wholeNumber(whole func(vaxfloat.Value) *big.Int) function {
	return func(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
		return vaxfloat.Round(f, vaxfloat.FromBigInt(whole(x)))
	}
}

// remainder is MTH$AMOD and its kind: a - INT(a/b) * b, with a's sign. It's
// computed exactly, in integers and rationals, so it's right however large the
// quotient, and it needs no rounding: a remainder is smaller than b, and in units
// of b's last bit or a's, so it has no more bits than they do. It can still be
// too small for the format, whose range stops short of b's last bit.
//
// VMS 7.1's answers (testdata/mth77) take the place of the footnotes' floating
// overflow: a quotient past the format's range gives a result of 0.0 and no condition,
// and a zero divisor is MTH$_INVARGMAT. A quotient too small says nothing about the
// remainder, which is a, so only a remainder too small is an underflow here, and G's
// and H's are 0.0 quietly.
func remainder(f vaxfloat.Format, a, b vaxfloat.Value) (vaxfloat.Value, error) {
	if b.IsZero() {
		return vaxfloat.Value{}, errInvalidArg
	}

	if _, err := vaxfloat.Div(f, a, b); errors.Is(err, vaxfloat.ErrOverflow) {
		return vaxfloat.Value{}, nil
	}

	quotient := new(big.Rat).Quo(a.Rat(), b.Rat())
	whole := new(big.Int).Quo(quotient.Num(), quotient.Denom()) // truncates toward zero
	exact := vaxfloat.AddExact(a, vaxfloat.MulExact(vaxfloat.FromBigInt(whole).Neg(), b))

	if f == vaxfloat.G || f == vaxfloat.H {
		return noUnderflow(vaxfloat.Round(f, exact))
	}

	return trapping(vaxfloat.Round(f, exact))
}

// convert is a conversion with no exception of its own: MTH$DBLE and MTH$FLOATI,
// which can't fail, and MTH$CVT_D_G and MTH$CVT_G_D, whose page gives them the
// library's MTH$_FLOOVEMAT and MTH$_FLOUNDMAT, as any function's range errors are.
func convert(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
	return vaxfloat.Round(f, x)
}

// convertTrapping is a conversion with Table 1-1's footnotes: MTH$SNGL and
// MTH$SNGLG, which can overflow. A value too small for F_floating is 0.0, whatever
// the caller's FU, though the footnote gives MTH$SNGLG an underflow: so does VMS 7.1
// (testdata/mth77).
func convertTrapping(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
	return trapping(noUnderflow(vaxfloat.Round(f, x)))
}

// productEntry makes MTH$DPROD and MTH$GPROD, the product of two F_floating
// values in a longer format, so without rounding. D_floating's range is F's, so
// its product can leave the format, as an overflow the footnote names, or as an
// underflow it doesn't. G's range holds any product.
func productEntry(out vaxfloat.Format) corevms.ShimFunc {
	return callWithArguments(floatArgs(vaxfloat.F), out, 2, func(args []vaxfloat.Value) (vaxfloat.Value, error) {
		return trapping(noUnderflow(vaxfloat.Mul(out, args[0], args[1])))
	})
}

// convertArrayEntry makes MTH$CVT_DA_GA and MTH$CVT_GA_DA: an array at the first
// argument converted to an array at the second, the number of elements at the
// third, 1 if it isn't given. R0 is 0, as on VMS 7.1 (testdata/mth77), for all the
// page's "return the address of the output array".
//
// The Go routine can't return from a signal to go on with the loop, since the
// handlers run after it ends. So an element that's out of range is stored as a
// scalar routine leaves it (0.0 for an underflow, else the reserved operand), the
// loop goes on, and the first such element's condition is signaled at the end,
// once. A reserved operand is a fault, and ends the loop where it is. VMS 7.1
// signals in the middle of its loop, and R0 is 0 after it (testdata/mth77).
func convertArrayEntry(in, out vaxfloat.Format) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		count := int32(1)

		if addr := arg(argv, 2); addr != 0 {
			n, err := env.Memory().LoadLongword(env.CPU(), addr)
			if err != nil {
				return fail(env, err)
			}

			count = int32(n)
		}

		source, dest := arg(argv, 0), arg(argv, 1)

		var failure error

		for i := range max(count, 0) {
			x, err := loadArg(env, in, source+uint32(i)*uint32(in.Size()))
			if err != nil {
				return fail(env, err)
			}

			v, err := vaxfloat.Round(out, x)
			v, err = gateUnderflow(env, v, err)
			target := dest + uint32(i)*uint32(out.Size())

			if err != nil {
				if failure == nil {
					failure = err
				}

				err = storeFailed(env, out, target, err)
			} else {
				err = storeResult(env, out, target, v)
			}

			if err != nil {
				return fail(env, err)
			}
		}

		if failure != nil {
			return signalFailure(env, failure)
		}

		return 0, nil
	}
}
