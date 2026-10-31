package mthrtl

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// A function here is what a family of routines shares: MTH$SQRT, MTH$DSQRT,
// MTH$GSQRT, MTH$HSQRT, and their JSB entries differ only in the format they
// work in and where they find their argument and put their result. The
// routines themselves are made from a function by callEntry and jsbEntry,
// each one line of the tables in routines.go.

// function computes one of the library's functions of one floating argument,
// rounded to format f. Its error says why it has no result:
// vaxfloat.ErrOverflow or vaxfloat.ErrUnderflow, or a domainError.
type function func(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error)

// binaryFunction is a function of two floating arguments: MTH$ATAN2's sine and
// cosine. Its errors are a function's.
type binaryFunction func(f vaxfloat.Format, a, b vaxfloat.Value) (vaxfloat.Value, error)

// domainError is an argument a function isn't defined on, as the condition
// value its routine signals.
type domainError uint32

func (d domainError) Error() string { return "argument out of the function's domain" }

var (
	errLogZeroNeg = domainError(vmsdef.LibrarySymbols["MTH$_LOGZERNEG"])
	errSqrtNeg    = domainError(vmsdef.LibrarySymbols["MTH$_SQUROONEG"])
	errInvalidArg = domainError(vmsdef.LibrarySymbols["MTH$_INVARGMAT"])

	mthFloOveMat = vmsdef.LibrarySymbols["MTH$_FLOOVEMAT"]
	mthFloUndMat = vmsdef.LibrarySymbols["MTH$_FLOUNDMAT"]
)

// errNoAngle is MTH$_INVARGMAT for ATAN2 of (0, 0), which is the one failure of an H
// routine that leaves the result through its first argument untouched, on VMS 7.1
// (testdata/mth77): the others store the reserved operand there first.
var errNoAngle = fmt.Errorf("%w: no angle at the origin", errInvalidArg)

// gateUnderflow is what a function's result and error mean to the routine's
// caller. An underflow is an error only for a caller that set PSW<FU> (floating
// underflow enable, with BISPSW or an entry mask): for any other, the manual's
// result is 0.0 and nothing is signaled.
func gateUnderflow(env *corevms.Environment, v vaxfloat.Value, err error) (vaxfloat.Value, error) {
	if errors.Is(err, vaxfloat.ErrUnderflow) {
		enabled, ferr := env.CallerFU()
		if ferr != nil {
			return vaxfloat.Value{}, ferr
		}

		if !enabled {
			return vaxfloat.Value{}, nil
		}
	}

	return v, err
}

// signalFailure signals the condition a function's error stands for, and
// returns what the routine returns. The result a handler continues with is
// the reserved operand, but for an underflow, where the manual's is 0.0.
func signalFailure(env *corevms.Environment, err error) (uint32, error) {
	var (
		domain domainError
		trap   trapError
	)

	switch {
	case errors.As(err, &trap):
		return signalTrap(env, trap)
	case errors.Is(err, vaxfloat.ErrOverflow):
		return signalCondition(env, mthFloOveMat)
	case errors.Is(err, vaxfloat.ErrUnderflow):
		return signalZero(env, mthFloUndMat)
	case errors.As(err, &domain):
		return signalCondition(env, uint32(domain))
	}

	return fail(env, err)
}

// callEntry makes the routine reached by CALLS or CALLG for fn in format f.
func callEntry(f vaxfloat.Format, fn function) corevms.ShimFunc {
	return conversionEntry(floatArgs(f), f, fn)
}

// conversionEntry is callEntry for a routine whose argument load reads, and whose
// result fn gives in format out: MTH$DBLE's F_floating argument and D_floating
// result, or MTH$FLOATJ's longword and F_floating.
func conversionEntry(load loader, out vaxfloat.Format, fn function) corevms.ShimFunc {
	return callWithArguments(load, out, 1, func(args []vaxfloat.Value) (vaxfloat.Value, error) {
		return fn(out, args[0])
	})
}

// callBinaryEntry is callEntry for a function of two arguments.
func callBinaryEntry(f vaxfloat.Format, fn binaryFunction) corevms.ShimFunc {
	return callWithArguments(floatArgs(f), f, 2, func(args []vaxfloat.Value) (vaxfloat.Value, error) {
		return fn(f, args[0], args[1])
	})
}

// loader reads one argument of a routine, by reference, as a Value, whatever the
// argument's type. Its error is one that fail signals.
type loader func(env *corevms.Environment, addr uint32) (vaxfloat.Value, error)

// floatArgs is the loader of format f's arguments.
func floatArgs(f vaxfloat.Format) loader {
	return func(env *corevms.Environment, addr uint32) (vaxfloat.Value, error) {
		return loadArg(env, f, addr)
	}
}

// callWithArguments makes a routine reached by CALLS or CALLG that computes its
// result, in format out, from count arguments, by reference, as the MTH$
// routines' are. An F, D, or G result comes back in R0 or R0:R1, and the
// arguments start with the first. An H result is too long for that, so it goes
// through the first argument, and the values follow it.
//
// A handler that continues from the routine's condition can correct only R0 and
// R1, not a result in memory, so what's there is what the condition leaves: the
// reserved operand, or 0.0 for an underflow, as on VMS 7.1 (testdata/mth77), which goes
// on to store a corrected value there too, as a Go routine that has ended can't.
func callWithArguments(load loader, out vaxfloat.Format, count int, compute func(args []vaxfloat.Value) (vaxfloat.Value, error)) corevms.ShimFunc {
	first := 0
	if out == vaxfloat.H {
		first = 1
	}

	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		args := make([]vaxfloat.Value, count)

		for i := range args {
			x, err := load(env, arg(argv, first+i))
			if err != nil {
				return fail(env, err)
			}

			args[i] = x
		}

		v, err := compute(args)
		v, err = gateUnderflow(env, v, err)

		if err != nil {
			if out == vaxfloat.H {
				if serr := storeFailed(env, out, arg(argv, 0), err); serr != nil {
					return fail(env, serr)
				}
			}

			return signalFailure(env, err)
		}

		if out != vaxfloat.H {
			return returnFloat(env, out, v), nil
		}

		if err := storeResult(env, out, arg(argv, 0), v); err != nil {
			return fail(env, err)
		}

		// The manual gives such a routine no return value, and R0 has already
		// lost the caller's value to the stub's own MOVL.
		return 0, nil
	}
}

// jsbEntry makes the routine reached by JSB for fn in format f: its argument
// is in R0, R0:R1, or R0:R3, and its result goes in the same registers. A
// condition is signaled as its caller's, the routine having no call frame.
//
// signalReserved and signalZero set what a handler can correct, R0 and R1.
// For an H result, R2 and R3 are this routine's to clear, directly in the
// CPU, because a signal's dispatch doesn't take the routine's regs back.
func jsbEntry(f vaxfloat.Format, fn function) corevms.JSBShimFunc {
	return func(env *corevms.Environment, regs *corevms.JSBRegisters) error {
		x, err := loadRegisters(f, regs[:], 0)
		if err != nil {
			_, err = fail(env, err)

			return err
		}

		v, err := fn(f, x)
		v, err = gateUnderflow(env, v, err)

		if err != nil {
			if f == vaxfloat.H {
				env.CPU().SetGPR(registers.R2, 0)
				env.CPU().SetGPR(registers.R3, 0)
			}

			_, err = signalFailure(env, err)

			return err
		}

		storeRegisters(f, regs, 0, v)

		return nil
	}
}

// jsbFailure signals err for a JSB entry, its result of count longwords in R0 up.
// R2 and R3 are this routine's to clear, as in jsbEntry.
func jsbFailure(env *corevms.Environment, count int, err error) error {
	if count > 2 {
		env.CPU().SetGPR(registers.R2, 0)
		env.CPU().SetGPR(registers.R3, 0)
	}

	_, err = signalFailure(env, err)

	return err
}

// jsbBinaryEntry is jsbEntry for a function of two arguments, the second in the
// registers after the first: MTH$ATAN2_R4's sine in R0 and cosine in R1. No manual
// gives these entries; VMS 7.1 does (testdata/mth77).
func jsbBinaryEntry(f vaxfloat.Format, fn binaryFunction) corevms.JSBShimFunc {
	return func(env *corevms.Environment, regs *corevms.JSBRegisters) error {
		x, err := loadRegisters(f, regs[:], 0)
		if err != nil {
			_, err = fail(env, err)

			return err
		}

		y, err := loadRegisters(f, regs[:], longwords(f))
		if err != nil {
			_, err = fail(env, err)

			return err
		}

		v, err := fn(f, x, y)
		v, err = gateUnderflow(env, v, err)

		if err != nil {
			return jsbFailure(env, longwords(f), err)
		}

		storeRegisters(f, regs, 0, v)

		return nil
	}
}

// jsbComplexEntry is jsbEntry for a function of a complex number with a complex
// result, both in R0 up: R0 and R1 for F, R0:R3 for D and G. Section 1.5 says complex
// functions have no JSB entries, and VMS 7.1 has them (testdata/mth77).
func jsbComplexEntry(f vaxfloat.Format, fn complexFunction) corevms.JSBShimFunc {
	return func(env *corevms.Environment, regs *corevms.JSBRegisters) error {
		z, err := complexInRegisters(f, regs[:], 0)
		if err != nil {
			_, err = fail(env, err)

			return err
		}

		z, err = gatedComplex(env, f, fn, z)
		if err != nil {
			return jsbFailure(env, 2*longwords(f), err)
		}

		storeRegisters(f, regs, 0, z.re)
		storeRegisters(f, regs, longwords(f), z.im)

		return nil
	}
}

// jsbModulusEntry is MTH$CABS_R4 and its kind: a complex number in R0 up, and one
// value in R0 or R0:R1.
func jsbModulusEntry(f vaxfloat.Format) corevms.JSBShimFunc {
	return func(env *corevms.Environment, regs *corevms.JSBRegisters) error {
		z, err := complexInRegisters(f, regs[:], 0)
		if err != nil {
			_, err = fail(env, err)

			return err
		}

		v, err := complexAbsolute(f, z)
		if err != nil {
			return jsbFailure(env, longwords(f), err)
		}

		storeRegisters(f, regs, 0, v)

		return nil
	}
}

// jsbSignEntry is MTH$SGN_R1: an F_floating value in R0, and -1, 0, or 1 in R0.
func jsbSignEntry(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	x, err := loadRegisters(vaxfloat.F, regs[:], 0)
	if err != nil {
		_, err = fail(env, err)

		return err
	}

	regs[0] = uint32(int32(x.Sign()))

	return nil
}

// pairFunction computes two results of one floating argument, rounded to
// format f: MTH$SINCOS's sine and cosine. The only error is
// vaxfloat.ErrUnderflow, for a first result too small for the format, and the
// results are still good: that first result is 0.0, and the second is right.
type pairFunction func(f vaxfloat.Format, x vaxfloat.Value) (first, second vaxfloat.Value, err error)

// evaluatePair runs fn on x for the routine's caller. It returns the results
// and whether to signal the underflow: only for a caller that set PSW<FU>, as
// in gateUnderflow. Its error is the one from reading the caller's PSW.
func evaluatePair(env *corevms.Environment, fn pairFunction, f vaxfloat.Format, x vaxfloat.Value) (first, second vaxfloat.Value, signal bool, err error) {
	first, second, ferr := fn(f, x)
	if ferr == nil {
		return first, second, false, nil
	}

	signal, err = env.CallerFU()

	return first, second, signal, err
}

// callPairEntry makes the routine reached by CALLS or CALLG for fn in format
// f, in every format the same: the argument's address, then the addresses to
// write the two results to. Nothing comes back in registers.
//
// Each result is in memory before the condition is signaled, so a handler that
// continues leaves the caller with 0.0 and the good second result.
func callPairEntry(f vaxfloat.Format, fn pairFunction) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		x, err := loadArg(env, f, arg(argv, 0))
		if err != nil {
			return fail(env, err)
		}

		first, second, signal, err := evaluatePair(env, fn, f, x)
		if err != nil {
			return fail(env, err)
		}

		if err := storeResult(env, f, arg(argv, 1), first); err != nil {
			return fail(env, err)
		}

		if err := storeResult(env, f, arg(argv, 2), second); err != nil {
			return fail(env, err)
		}

		if signal {
			return signalZero(env, mthFloUndMat)
		}

		return 0, nil
	}
}

// jsbPairEntry makes the routine reached by JSB for fn in format f: its
// argument is in R0:Rm, m being 0 for F, 1 for D and G, and 3 for H, its first
// result goes in the same registers, and its second in R(m+1):R(2m+1). An H
// pair so takes all of R0:R7.
//
// A signal's dispatch doesn't take the routine's regs back (JSBShim), so before
// a condition the results go straight in the CPU's registers. For an F pair the
// second result is in R1, which the mechanism array holds and a handler could
// change.
func jsbPairEntry(f vaxfloat.Format, fn pairFunction) corevms.JSBShimFunc {
	return func(env *corevms.Environment, regs *corevms.JSBRegisters) error {
		x, err := loadRegisters(f, regs[:], 0)
		if err != nil {
			_, err = fail(env, err)

			return err
		}

		first, second, signal, err := evaluatePair(env, fn, f, x)
		if err != nil {
			_, err = fail(env, err)

			return err
		}

		storeRegisters(f, regs, 0, first)
		storeRegisters(f, regs, f.Size()/4, second)

		if signal {
			for i, r := range regs {
				env.CPU().SetGPR(registers.Reg(i), r)
			}

			_, err = signalKeepingR1(env, mthFloUndMat, regs[0])
		}

		return err
	}
}
