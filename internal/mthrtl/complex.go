package mthrtl

import (
	"errors"
	"math/big"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The complex functions. A complex number (r, i) is two values of one format,
// the real part r first, one after the other in memory. There are F, D, and G
// complex numbers, and no H. The formats also differ in how a result comes back:
//
//	F complex    in R0 and R1, r in R0
//	D, G complex through the routine's first argument, the others following
//
// The routines' pages give each a formula of real functions. Each part here is
// that formula's exact value, rounded once, like every other result in this
// library. What's left of the pages is when a routine gives up, and those are the
// comments below. No page lists MTH$_FLOUNDMAT, so a part too small for the format
// is 0.0, whatever the caller's FU, as for MTH$ATAN2; only MTH$CEXP's factor has
// an underflow of its own.

// complexValue is a complex number.
type complexValue struct{ re, im vaxfloat.Value }

// complexFunction computes a function from a complex number to a complex number,
// rounded to format f. Its errors are a function's.
type complexFunction func(f vaxfloat.Format, z complexValue) (complexValue, error)

// complexLoader reads a routine's complex argument, from args, the argument list
// without any result's address.
type complexLoader func(env *corevms.Environment, f vaxfloat.Format, args []uint32) (complexValue, error)

// complexAt is the usual complexLoader: one argument, the address of a complex
// number.
func complexAt(env *corevms.Environment, f vaxfloat.Format, args []uint32) (complexValue, error) {
	addr := arg(args, 0)

	re, err := loadArg(env, f, addr)
	if err != nil {
		return complexValue{}, err
	}

	im, err := loadArg(env, f, addr+uint32(f.Size()))

	return complexValue{re, im}, err
}

// complexFromParts is MTH$CMPLX's complexLoader: two arguments, the addresses of
// the parts.
func complexFromParts(env *corevms.Environment, f vaxfloat.Format, args []uint32) (complexValue, error) {
	re, err := loadArg(env, f, arg(args, 0))
	if err != nil {
		return complexValue{}, err
	}

	im, err := loadArg(env, f, arg(args, 1))

	return complexValue{re, im}, err
}

// complexEntry makes a routine reached by CALLS or CALLG with a complex result in
// format f, from the complex number load reads.
//
// A D or G result goes through memory, where a handler that continues can't
// correct it, so what's there is what the condition leaves: each part the reserved
// operand. An F result gets what any routine's does, R0 the reserved operand and
// R1 clear: section 1.4 tells a handler to correct the two registers as a
// double-precision value, for single and double alike. Where VMS 7.1 goes on using
// those values and faults (testdata/mth77), this returns them: the phase doc's list.
func complexEntry(f vaxfloat.Format, load complexLoader, fn complexFunction) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		args := argv
		if f != vaxfloat.F && len(args) > 0 {
			args = args[1:]
		}

		z, err := load(env, f, args)
		if err != nil {
			return fail(env, err)
		}

		z, err = gatedComplex(env, f, fn, z)
		if err != nil {
			if f != vaxfloat.F {
				if serr := storeComplexFailed(env, f, arg(argv, 0), err); serr != nil {
					return fail(env, serr)
				}
			}

			return signalFailure(env, err)
		}

		if f == vaxfloat.F {
			env.CPU().SetGPR(registers.R1, registerWords(f, z.im)[0])

			return registerWords(f, z.re)[0], nil
		}

		addr := arg(argv, 0)
		if err := storeResult(env, f, addr, z.re); err != nil {
			return fail(env, err)
		}

		if err := storeResult(env, f, addr+uint32(f.Size()), z.im); err != nil {
			return fail(env, err)
		}

		return 0, nil
	}
}

// gatedComplex runs fn on z, for a caller whose FU decides what an underflow is.
func gatedComplex(env *corevms.Environment, f vaxfloat.Format, fn complexFunction, z complexValue) (complexValue, error) {
	z, err := fn(f, z)

	return gateComplexUnderflow(env, z, err)
}

// gateComplexUnderflow is gateUnderflow for a complex result: the result is 0.0, and
// the error is gone unless the caller set FU.
func gateComplexUnderflow(env *corevms.Environment, z complexValue, err error) (complexValue, error) {
	if !errors.Is(err, vaxfloat.ErrUnderflow) {
		return z, err
	}

	_, err = gateUnderflow(env, vaxfloat.Value{}, err)

	return complexValue{}, err
}

// storeComplexFailed is storeFailed for both parts of a result at addr.
func storeComplexFailed(env *corevms.Environment, f vaxfloat.Format, addr uint32, err error) error {
	if serr := storeFailed(env, f, addr, err); serr != nil {
		return serr
	}

	return storeFailed(env, f, addr+uint32(f.Size()), err)
}

// modulusEntry makes MTH$CABS and its kind, whose result is one value of the
// format, in R0 or R0:R1.
func modulusEntry(f vaxfloat.Format) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		z, err := complexAt(env, f, argv)
		if err != nil {
			return fail(env, err)
		}

		v, err := complexAbsolute(f, z)
		if err != nil {
			return signalFailure(env, err)
		}

		return returnFloat(env, f, v), nil
	}
}

// imaginaryArgs is the loader of a complex number's imaginary part in format f.
// MTH$AIMAG and MTH$REAL read only the part they return, so a reserved operand in
// the other doesn't fault. Unconfirmed (the probe, subtask 10).
func imaginaryArgs(f vaxfloat.Format) loader {
	return func(env *corevms.Environment, addr uint32) (vaxfloat.Value, error) {
		return loadArg(env, f, addr+uint32(f.Size()))
	}
}

// sumOfSquares returns |z|^2, exactly. It may take many bits, if the parts' sizes
// are far apart, but that's what makes MTH$CLOG right when z is near the unit
// circle, where the sum's difference from 1 is all the logarithm has.
func sumOfSquares(z complexValue) *big.Float {
	return vaxfloat.AddExact(vaxfloat.MulExact(z.re, z.re), vaxfloat.MulExact(z.im, z.im)).Big()
}

// modulus returns |z|, to prec bits. The manual's MAX * SQRT((MIN/MAX)^2 + 1) keeps
// a format's arithmetic from overflowing on the way, which sumOfSquares has no fear
// of.
func modulus(z complexValue, prec uint) *big.Float {
	return newFloat(prec).Sqrt(sumOfSquares(z))
}

// complexAbsolute is MTH$CABS and its kind. Its only failure is the page's "both r
// and i are large", MTH$_FLOOVEMAT, for a modulus past the format's range.
func complexAbsolute(f vaxfloat.Format, z complexValue) (vaxfloat.Value, error) {
	return rounded(f, modulus(z, workingPrecision(f)))
}

// bothParts rounds a result's two parts, a part too small for f being 0.0.
func bothParts(f vaxfloat.Format, re, im *big.Float) (complexValue, error) {
	r, err := noUnderflow(rounded(f, re))
	if err != nil {
		return complexValue{}, err
	}

	i, err := noUnderflow(rounded(f, im))
	if err != nil {
		return complexValue{}, err
	}

	return complexValue{r, i}, nil
}

// product returns a * b, to prec bits.
func product(a, b *big.Float, prec uint) *big.Float { return newFloat(prec).Mul(a, b) }

// overflows reports whether x is past format f's range.
func overflows(f vaxfloat.Format, x *big.Float) bool {
	_, err := rounded(f, x)

	return errors.Is(err, vaxfloat.ErrOverflow)
}

// complexExponential is MTH$CEXP and its kind: (EXP(r) * COS(i), EXP(r) * SIN(i)).
// Its page's overflow is the factor's, EXP(r), past about 88.029 in F and D and
// 709.089 in G, whatever the sine and cosine would make of it.
//
// That page says "the absolute value of r", but a large negative r has e^r near 0,
// which can't overflow. The factor too small for the format is MTH$EXP's underflow,
// which VMS 7.1 signals here too (testdata/mth77) for a caller with FU set, though the
// page lists no MTH$_FLOUNDMAT. Otherwise the result is 0.0.
func complexExponential(f vaxfloat.Format, z complexValue) (complexValue, error) {
	return exponentialParts(f, z.re.Big(), z.im.Big())
}

// exponentialParts is e to the (r, i), for parts that may have more bits than a
// format's: MTH$CEXP's work, and OTS$POWCC's.
func exponentialParts(f vaxfloat.Format, r, i *big.Float) (complexValue, error) {
	w := workingPrecision(f)

	e := exponential(r, w+scratchBits)
	if _, err := rounded(f, e); err != nil {
		return complexValue{}, err
	}

	s, c := sinCos(i, w+scratchBits)

	return bothParts(f, product(e, c, w), product(e, s, w))
}

// complexTrigFactors returns what MTH$CSIN and MTH$CCOS multiply: the sine and
// cosine of r and the hyperbolic sine and cosine of i.
//
// The pages' overflow is "the absolute value of i is greater than about 88.029" in
// F and D, or 709.089 in G. Those are EXP's limits, not COSH's and SINH's, which are
// about ln 2 higher (88.722), so the test is on e^|i|, which is cosh i + |sinh i|.
// Between the two limits a part could be in range and the routine still overflows,
// and past them whatever r is, as on VMS 7.1 (testdata/mth77).
func complexTrigFactors(f vaxfloat.Format, z complexValue, prec uint) (s, c, sh, ch *big.Float, err error) {
	sh, ch = sinhCosh(z.im.Big(), prec)

	if overflows(f, newFloat(prec).Add(ch, newFloat(prec).Abs(sh))) {
		return nil, nil, nil, nil, vaxfloat.ErrOverflow
	}

	s, c = sinCos(z.re.Big(), prec)

	return s, c, sh, ch, nil
}

// complexSine is MTH$CSIN and its kind: (SIN(r) * COSH(i), COS(r) * SINH(i)).
func complexSine(f vaxfloat.Format, z complexValue) (complexValue, error) {
	w := workingPrecision(f)

	s, c, sh, ch, err := complexTrigFactors(f, z, w+scratchBits)
	if err != nil {
		return complexValue{}, err
	}

	return bothParts(f, product(s, ch, w), product(c, sh, w))
}

// complexCosine is MTH$CCOS and its kind: (COS(r) * COSH(i), -SIN(r) * SINH(i)).
func complexCosine(f vaxfloat.Format, z complexValue) (complexValue, error) {
	w := workingPrecision(f)

	s, c, sh, ch, err := complexTrigFactors(f, z, w+scratchBits)
	if err != nil {
		return complexValue{}, err
	}

	im := product(s, sh, w)

	return bothParts(f, product(c, ch, w), im.Neg(im))
}

// zeroComplexBase is what the logarithm of (0, 0) signals in format f.
func zeroComplexBase(f vaxfloat.Format) error {
	if f == vaxfloat.F {
		return errInvalidArg
	}

	return errLogZeroNeg
}

// complexLog is MTH$CLOG and its kind: (LOG(CABS(z)), ATAN2(i, r)).
//
// The real part is taken as ln(|z|^2) / 2, from sumOfSquares, because rounding the
// modulus first would lose all of it for z near the unit circle.
//
// The conditions are VMS 7.1's (testdata/mth77), which aren't the pages'. A modulus
// past the format's range is MTH$CxABS's own MTH$_FLOOVEMAT, in all three forms. At 0,
// the F form signals MTH$_INVARGMAT, and the D and G forms the logarithm's
// MTH$_LOGZERNEG (zeroComplexBase).
func complexLog(f vaxfloat.Format, z complexValue) (complexValue, error) {
	if z.re.IsZero() && z.im.IsZero() {
		return complexValue{}, zeroComplexBase(f)
	}

	w := workingPrecision(f)

	if overflows(f, modulus(z, w)) {
		return complexValue{}, vaxfloat.ErrOverflow
	}

	logSquare := logarithm(sumOfSquares(z), w)

	re, err := noUnderflow(rounded(f, logSquare.SetMantExp(logSquare, -1)))
	if err != nil {
		return complexValue{}, err
	}

	im, err := arcTangent2(f, z.im, z.re)
	if err != nil {
		return complexValue{}, err
	}

	return complexValue{re, im}, nil
}

// complexSquareRoot is MTH$CSQRT and its kind, the root with no negative real part.
// The page's steps are
//
//	ROOT = SQRT((ABS(r) + CABS(r, i)) / 2)
//	Q    = i / (2 * ROOT)
//
//	r >= 0          (ROOT, Q)
//	r < 0, i >= 0   (Q, ROOT)
//	r < 0, i < 0    (-Q, -ROOT)
//
// which add only positive numbers, so nothing cancels. A square root is much
// smaller than a value near the format's limit, so the result can't overflow. The
// page's SS$_FLTOVF_F has no argument here: VMS 7.1 has answers for (1.5E38, 0.0)
// and the like (testdata/mth77), where the page's sum would overflow, so this doesn't
// check it. Zero, on which Q has no value, has the root 0.
func complexSquareRoot(f vaxfloat.Format, z complexValue) (complexValue, error) {
	if z.re.IsZero() && z.im.IsZero() {
		return complexValue{}, nil
	}

	w := workingPrecision(f)
	inner := w + scratchBits

	sum := newFloat(inner).Add(z.re.Abs().Big(), modulus(z, inner))
	root := newFloat(inner).Sqrt(sum.SetMantExp(sum, -1))
	q := newFloat(inner).Quo(z.im.Big(), root)
	q.SetMantExp(q, -1)

	switch {
	case z.re.Sign() >= 0:
		return bothParts(f, root, q)
	case z.im.Sign() >= 0:
		return bothParts(f, q, root)
	}

	return bothParts(f, q.Neg(q), root.Neg(root))
}

// conjugate is MTH$CONJG and its kind: (r, -i).
func conjugate(_ vaxfloat.Format, z complexValue) (complexValue, error) {
	return complexValue{z.re, z.im.Neg()}, nil
}

// asComplex is MTH$CMPLX and its kind, whose result is its arguments.
func asComplex(_ vaxfloat.Format, z complexValue) (complexValue, error) { return z, nil }
