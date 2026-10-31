package mthrtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The maximum and minimum routines, whose names say what types they compare and
// return: MTH$AJMAX0 is the F_floating maximum of longwords, and MTH$JMAX1 the
// longword maximum of F_floating values. These are the library's only routines of
// any number of arguments. Each compares its arguments as they are, in full, and
// converts only the one it picks: MTH$AJMAX0 of two longwords that round to one
// F_floating value still picks the larger. MTH$IMAX1's and MTH$JMAX1's result
// truncates, as FORTRAN's INT does.

// wantLargest and wantSmallest say which argument a routine wants: the result of
// the comparison that replaces the one found so far.
const (
	wantLargest  = 1
	wantSmallest = -1
)

// extreme finds the routine's answer among the arguments in values, in order, so a
// reserved operand anywhere faults. With none it's 0, and no condition, as MTH$AMAX1's
// is on VMS 7.1 (testdata/mth77), whose integer forms read whatever follows the
// argument list.
func extreme(env *corevms.Environment, load loader, values []uint32, want int) (vaxfloat.Value, error) {
	var best vaxfloat.Value

	for i, addr := range values {
		v, err := load(env, addr)
		if err != nil {
			return vaxfloat.Value{}, err
		}

		if i == 0 || v.Cmp(best) == want {
			best = v
		}
	}

	return best, nil
}

// extremeFloatEntry makes a routine with a floating result. An H result goes
// through the first argument, and the values follow it, as for any H routine.
func extremeFloatEntry(load loader, out vaxfloat.Format, want int) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		values := argv
		if out == vaxfloat.H && len(argv) > 0 {
			values = argv[1:]
		}

		v, err := extreme(env, load, values, want)
		if err != nil {
			return signalFailure(env, err)
		}

		// Rounding is only for MTH$AJMAX0's longword, which F_floating's range
		// holds.
		v, err = vaxfloat.Round(out, v)
		if err != nil {
			return signalFailure(env, err)
		}

		if out != vaxfloat.H {
			return returnFloat(env, out, v), nil
		}

		if err := storeResult(env, out, arg(argv, 0), v); err != nil {
			return fail(env, err)
		}

		return 0, nil
	}
}

// extremeIntegerEntry makes a routine with an integer result of width w.
func extremeIntegerEntry(load loader, w width, want int) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		v, err := extreme(env, load, argv, want)
		if err != nil {
			return signalFailure(env, err)
		}

		return returnInteger(env, w, truncated(v))
	}
}
