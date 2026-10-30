package mthrtl

import (
	"errors"
	"math/big"
	"slices"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// The OTS$ routines VMS keeps in this library: powers, and complex multiplication
// and division. A compiler calls them for the ** operator and for the complex
// operations it doesn't write in line.
//
// Unlike the MTH$ routines they take their arguments by value, the values themselves
// in the argument list: an F_floating or a longword is one longword of it, a D, G, or
// F complex two (the first holding the sign and exponent), an H or a D or G complex
// four. Every result is in registers, from R0 up: R0, R0:R1, or for an H or a D or G
// complex R0:R3, which is what the _R3 in those names means.
//
// How each result is computed follows its page in the OTS$ manual:
//
//	real ** real        2 ** (exponent * LOG2(base)), the exact value rounded once
//	real ** integer     vaxfloat.PowerBySquaring, rounding at each product
//	complex ** complex  exp(exponent * log(base)), each part rounded once
//	complex ** integer  vaxfloat.PowerBySquaring, each product's parts rounded once
//
// A page's "internal calculations ... have the same precision as the base value"
// is taken at its word for the integer powers, since their tables spell out a
// product. For a floating exponent the library's rule holds (the phase doc's
// Decision 3), as it does for every other function here. That makes a floating
// exponent's result correctly rounded but for about one argument in 2^40, except that
// a power of a small integer may fall exactly halfway between two values, where it
// may round either way: 4097.0 ** 2.0 is exactly halfway in F_floating. Still within
// the plan's one ulp.
//
// Several pages list both SS$_FLTOVF and MTH$_FLOOVEMAT. Overflow is MTH$_FLOOVEMAT
// wherever the page has it, the two pages of the same algorithm that list only one
// (OTS$POWDD, OTS$POWxLU) naming that. Unconfirmed (the probe, subtask 10).

var errUndefinedPower = domainError(vmsdef.LibrarySymbols["MTH$_UNDEXP"])

// valueArguments is argv with at least count longwords. A call that passes too few
// reads past its argument list on VMS; here what's missing is 0.
func valueArguments(argv []uint32, count int) []uint32 {
	if len(argv) >= count {
		return argv
	}

	return append(slices.Clone(argv), make([]uint32, count-len(argv))...)
}

// longwords is how many longwords format f takes in an argument list or registers.
func longwords(f vaxfloat.Format) int { return f.Size() / 4 }

// returnWords puts a result's longwords in R0 and up, as returnFloat does, returning
// R0's for the routine to return.
func returnWords(env *corevms.Environment, words []uint32) uint32 {
	for i, w := range words[1:] {
		env.CPU().SetGPR(registers.Reg(i+1), w)
	}

	return words[0]
}

// failInRegisters signals err's condition for a routine with a result of count
// longwords in registers. A handler can correct only R0 and R1, so R2 and R3 are
// clear when it continues, what the reserved operand or an underflow's 0.0 has
// there. Unconfirmed (the probe, subtask 10).
func failInRegisters(env *corevms.Environment, count int, err error) (uint32, error) {
	if count > 2 {
		env.CPU().SetGPR(registers.R2, 0)
		env.CPU().SetGPR(registers.R3, 0)
	}

	return signalFailure(env, err)
}

// returnPower ends a routine with a result in format f, or its failure. An underflow
// is an error only for a caller that set PSW<FU>: OTS$POWxLU's page says so, and the
// others list the condition.
func returnPower(env *corevms.Environment, f vaxfloat.Format, v vaxfloat.Value, err error) (uint32, error) {
	v, err = gateUnderflow(env, v, err)
	if err != nil {
		return failInRegisters(env, longwords(f), err)
	}

	return returnFloat(env, f, v), nil
}

// power is base ** exponent for a floating exponent, in format f, its pages' table:
// a negative base has no power whatever the exponent, and zero only to a positive
// exponent. Otherwise it's exp(exponent * ln(base)), which is the pages'
// 2 ** (exponent * LOG2(base)).
//
// The product needs a good absolute accuracy, since it's what the exponential
// starts from. A product of size 2^exponentialOutOfRange or more is out of every
// format's range, so that many extra bits are all it takes, however large the
// exponent: past that it only has to be the right sign.
func power(f vaxfloat.Format, base, exponent vaxfloat.Value) (vaxfloat.Value, error) {
	switch {
	case base.Sign() < 0, base.IsZero() && exponent.Sign() <= 0:
		return vaxfloat.Value{}, errUndefinedPower
	case base.IsZero():
		return vaxfloat.Value{}, nil
	}

	w := workingPrecision(f)
	inner := w + scratchBits + exponentialOutOfRange

	product := newFloat(inner).Mul(exponent.Big(), logarithm(base.Big(), inner))

	return rounded(f, exponential(product, w))
}

// powerEntry makes a routine raising a base in one format to an exponent in another,
// with its result in format out: OTS$POWRD's F base, D exponent, and D result. A
// base in a shorter format than its result needs no conversion, a Value being exact.
func powerEntry(baseFormat, exponentFormat, out vaxfloat.Format) corevms.ShimFunc {
	baseWords := longwords(baseFormat)

	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		words := valueArguments(argv, baseWords+longwords(exponentFormat))

		base, err := loadRegisters(baseFormat, words, 0)
		if err != nil {
			return fail(env, err)
		}

		exponent, err := loadRegisters(exponentFormat, words, baseWords)
		if err != nil {
			return fail(env, err)
		}

		v, err := power(out, base, exponent)

		return returnPower(env, out, v, err)
	}
}

// integerPower is base ** n in format f, for an integer exponent. gate says what an
// underflow in a product means, which the caller's FU decides: 0.0, going on, or the
// error. If the products have come to 0.0 by then, 1.0 over them for a negative n
// is the divide by zero the hardware would take; no real page lists it, and the
// complex one does. Unconfirmed (the probe, subtask 10).
func integerPower(f vaxfloat.Format, base vaxfloat.Value, n int64, gate func(vaxfloat.Value, error) (vaxfloat.Value, error)) (vaxfloat.Value, error) {
	if base.IsZero() && n <= 0 {
		return vaxfloat.Value{}, errUndefinedPower
	}

	mul := func(a, b vaxfloat.Value) (vaxfloat.Value, error) { return gate(vaxfloat.Mul(f, a, b)) }
	div := func(a, b vaxfloat.Value) (vaxfloat.Value, error) {
		v, err := vaxfloat.Div(f, a, b)

		return v, asTrap(err, vaxfloat.ErrDivideByZero)
	}

	return vaxfloat.PowerBySquaring(base, vaxfloat.FromInt(1), n, mul, div)
}

// integerPowerEntry makes a routine raising a base in format f to an integer exponent
// of width w: a signed longword, OTS$POWRJ, or an unsigned one, OTS$POWRLU.
func integerPowerEntry(f vaxfloat.Format, w width) corevms.ShimFunc {
	baseWords := longwords(f)

	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		words := valueArguments(argv, baseWords+1)

		base, err := loadRegisters(f, words, 0)
		if err != nil {
			return fail(env, err)
		}

		gate := func(v vaxfloat.Value, err error) (vaxfloat.Value, error) {
			return gateUnderflow(env, v, err)
		}

		v, err := integerPower(f, base, w.fromValue(words[baseWords]), gate)

		return returnPower(env, f, v, err)
	}
}

// complexOperation computes a routine's complex result from two complex arguments,
// in format f. Its errors are a function's.
type complexOperation func(f vaxfloat.Format, a, b complexValue) (complexValue, error)

// complexInRegisters reads the complex number in format f at words[first:].
func complexInRegisters(f vaxfloat.Format, words []uint32, first int) (complexValue, error) {
	re, err := loadRegisters(f, words, first)
	if err != nil {
		return complexValue{}, err
	}

	im, err := loadRegisters(f, words, first+longwords(f))

	return complexValue{re, im}, err
}

// returnComplex ends a routine with a complex result in format f, or its failure.
func returnComplex(env *corevms.Environment, f vaxfloat.Format, z complexValue, err error) (uint32, error) {
	if err != nil {
		return failInRegisters(env, 2*longwords(f), err)
	}

	return returnWords(env, append(registerWords(f, z.re), registerWords(f, z.im)...)), nil
}

// complexOperationEntry makes a routine of two complex arguments and a complex
// result: OTS$POWCC, OTS$DIVC, OTS$MULCD_R3, and their kind.
func complexOperationEntry(f vaxfloat.Format, op complexOperation) corevms.ShimFunc {
	size := 2 * longwords(f)

	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		words := valueArguments(argv, 2*size)

		a, err := complexInRegisters(f, words, 0)
		if err != nil {
			return fail(env, err)
		}

		b, err := complexInRegisters(f, words, size)
		if err != nil {
			return fail(env, err)
		}

		z, err := op(f, a, b)

		return returnComplex(env, f, z, err)
	}
}

// complexProduct is OTS$MULCD_R3 and OTS$MULCG_R3. Overflow is the page's
// MTH$_FLOOVEMAT.
func complexProduct(f vaxfloat.Format, a, b complexValue) (complexValue, error) {
	re, im, err := vaxfloat.ComplexMul(f, a.re, a.im, b.re, b.im)

	return complexValue{re, im}, err
}

// complexQuotient is OTS$DIVC and its kind, whose page lists the hardware's
// exceptions, SS$_FLTDIV_F and SS$_FLTOVF_F.
func complexQuotient(f vaxfloat.Format, a, b complexValue) (complexValue, error) {
	re, im, err := vaxfloat.ComplexDiv(f, a.re, a.im, b.re, b.im)

	return complexValue{re, im}, asTrap(err, vaxfloat.ErrDivideByZero, vaxfloat.ErrOverflow)
}

// complexPower is OTS$POWCC and its kind, the page's exp(y * log(x)). A base of 0
// has no logarithm, so it's the page's MTH$_INVARGMAT whatever the exponent. The
// overflow and the parts too small are MTH$CEXP's, this being that function of a
// number that isn't rounded to the format first.
//
// The product's imaginary part is an angle, and needs a good absolute accuracy to
// find its sine, however large the exponent makes it, so the logarithm has the bits
// of the exponent's size besides the exponential's. A format with complex numbers has
// at most 1024 of them. That makes each part good beside the result's modulus, not
// beside itself: where the angle is a true multiple of pi/2, (-1, 0) ** (2, 0) for one,
// the part that should be 0 is the angle's error, some 2^-30 of a last bit of the other.
func complexPower(f vaxfloat.Format, x, y complexValue) (complexValue, error) {
	if x.re.IsZero() && x.im.IsZero() {
		return complexValue{}, errInvalidArg
	}

	size := max(y.re.Big().MantExp(nil), y.im.Big().MantExp(nil), 0)
	inner := workingPrecision(f) + scratchBits + exponentialOutOfRange + uint(size)

	logRe := logarithm(sumOfSquares(x), inner)
	logRe.SetMantExp(logRe, -1)
	logIm := arcTan2(x.im.Big(), x.re.Big(), inner)

	re := newFloat(inner).Sub(product(y.re.Big(), logRe, inner), product(y.im.Big(), logIm, inner))
	im := newFloat(inner).Add(product(y.re.Big(), logIm, inner), product(y.im.Big(), logRe, inner))

	return exponentialParts(f, re, im)
}

// complexIntegerPower is OTS$POWCJ and its kind, in format f. The page's "Not (0.,0.)
// <0" row says only "the product", but a power with a negative exponent has to be the
// reciprocal, in the order of the real pages. It lists the hardware's exceptions,
// and SS$_FLTDIV is what 1.0 over a product that has come to (0.0, 0.0) takes.
func complexIntegerPower(f vaxfloat.Format, base complexValue, n int64) (complexValue, error) {
	if base.re.IsZero() && base.im.IsZero() && n <= 0 {
		return complexValue{}, errUndefinedPower
	}

	mul := func(a, b complexValue) (complexValue, error) {
		z, err := complexProduct(f, a, b)

		return z, asTrap(err, vaxfloat.ErrOverflow)
	}
	div := func(a, b complexValue) (complexValue, error) { return complexQuotient(f, a, b) }

	return vaxfloat.PowerBySquaring(base, complexValue{re: vaxfloat.FromInt(1)}, n, mul, div)
}

// complexIntegerPowerEntry makes OTS$POWCJ and its kind.
func complexIntegerPowerEntry(f vaxfloat.Format) corevms.ShimFunc {
	size := 2 * longwords(f)

	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		words := valueArguments(argv, size+1)

		base, err := complexInRegisters(f, words, 0)
		if err != nil {
			return fail(env, err)
		}

		z, err := complexIntegerPower(f, base, long.fromValue(words[size]))

		return returnComplex(env, f, z, err)
	}
}

// integerPowerOfEntry makes OTS$POWII, OTS$POWJJ, and OTS$POWLULU.
//
// OTS$POWII's and OTS$POWJJ's pages have no table, and name floating traps and "a
// negative base" as undefined, which looks copied from a floating page: FORTRAN's
// (-2) ** 3 is -8. So only zero to zero or below signals MTH$_UNDEXP, and a negative
// exponent is 1 / base ** |n|, truncated as an integer division is. VMS's mask sets IV
// on these, so an overflow is SS$_INTOVF, with the low-order bits in R0. All
// unconfirmed (the probe, subtask 10). OTS$POWLULU's page says it can't overflow: its
// result is simply the low-order bits, and it has no negative numbers.
func integerPowerOfEntry(w width) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		words := valueArguments(argv, 2)
		base, n := w.fromValue(words[0]), w.fromValue(words[1])

		switch {
		case base == 0 && n <= 0:
			return signalFailure(env, errUndefinedPower)
		case n < 0:
			return w.lowBits(big.NewInt(negativePower(base, n))), nil
		}

		low, overflow := integerPowerBits(w, base, n)
		if overflow {
			return env.SignalFailure([]uint32{ssIntOvf}, low)
		}

		return low, nil
	}
}

// negativePower is base ** n, for n negative and base not 0: 1 / base ** |n|,
// truncated, which is 0 unless the base's size is 1.
func negativePower(base, n int64) int64 {
	switch base {
	case 1:
		return 1
	case -1:
		return 1 - 2*(n&1)
	}

	return 0
}

// integerPowerBits is base ** n, for n not negative, as width w's low-order bits, and
// whether the power doesn't fit. A base of size 2 or more leaves any width behind
// within 64 multiplications, so a larger n needs no exact value, which could have
// billions of bits, only the low-order ones.
func integerPowerBits(w width, base, n int64) (low uint32, overflow bool) {
	b, e := big.NewInt(base), big.NewInt(n)
	modulus := new(big.Int).Lsh(big.NewInt(1), w.bits)

	low = w.lowBits(new(big.Int).Exp(b, e, modulus))

	if w.unsigned || base >= -1 && base <= 1 {
		return low, false
	}

	return low, n >= 64 || !w.holds(new(big.Int).Exp(b, e, nil))
}

// asTrap makes err the hardware exception a floating instruction takes for it, if it
// is one of kinds, and otherwise leaves it as it is.
func asTrap(err error, kinds ...error) error {
	for _, kind := range kinds {
		if !errors.Is(err, kind) {
			continue
		}

		switch kind {
		case vaxfloat.ErrOverflow:
			return errFloatOverflow
		case vaxfloat.ErrDivideByZero:
			return errFloatDivide
		}
	}

	return err
}
