package mthrtl

import (
	"errors"

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

// domainError is an argument a function isn't defined on, as the condition
// value its routine signals.
type domainError uint32

func (d domainError) Error() string { return "argument out of the function's domain" }

var (
	errLogZeroNeg = domainError(vmsdef.LibrarySymbols["MTH$_LOGZERNEG"])
	errSqrtNeg    = domainError(vmsdef.LibrarySymbols["MTH$_SQUROONEG"])

	mthFloOveMat = vmsdef.LibrarySymbols["MTH$_FLOOVEMAT"]
	mthFloUndMat = vmsdef.LibrarySymbols["MTH$_FLOUNDMAT"]
)

// evaluate runs fn on x for the routine's caller. An underflow is an error
// only for a caller that set PSW<FU> (floating underflow enable, with BISPSW
// or an entry mask): for any other, the manual's result is 0.0 and nothing
// is signaled.
func evaluate(env *corevms.Environment, fn function, f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
	v, err := fn(f, x)

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
	var domain domainError

	switch {
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
// Its arguments are by reference, as the MTH$ routines' are. An F, D, or G
// result comes back in R0 or R0:R1, and the argument is the first one; an H
// result is too long for that, so it goes through the first argument, and
// the value is the second.
func callEntry(f vaxfloat.Format, fn function) corevms.ShimFunc {
	if f == vaxfloat.H {
		return throughFirstArgument(fn)
	}

	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		x, err := loadArg(env, f, arg(argv, 0))
		if err != nil {
			return fail(env, err)
		}

		v, err := evaluate(env, fn, f, x)
		if err != nil {
			return signalFailure(env, err)
		}

		return returnFloat(env, f, v), nil
	}
}

// throughFirstArgument is callEntry for an H_floating result.
//
// A handler that continues from the routine's condition can correct only R0
// and R1, not this result, so what's in memory is what the condition leaves:
// the reserved operand, or 0.0 for an underflow. Unconfirmed (the probe,
// subtask 10).
func throughFirstArgument(fn function) corevms.ShimFunc {
	const f = vaxfloat.H

	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		x, err := loadArg(env, f, arg(argv, 1))
		if err != nil {
			return fail(env, err)
		}

		result := arg(argv, 0)

		v, err := evaluate(env, fn, f, x)
		if err != nil {
			if serr := storeFailed(env, f, result, err); serr != nil {
				return fail(env, serr)
			}

			return signalFailure(env, err)
		}

		if err := storeResult(env, f, result, v); err != nil {
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

		v, err := evaluate(env, fn, f, x)
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
