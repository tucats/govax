package mthrtl

import (
	"github.com/tucats/govax/internal/vaxfloat"
)

// The hyperbolic functions. Their limits follow from the formats, as the
// exponential's do: the manual's MTH$_FLOOVEMAT for MTH$SINH and MTH$COSH "above
// about 88.722" is where e^x / 2 reaches 2^127, F_floating's limit, and rounded
// reports it.
//
// None of them can underflow: a hyperbolic sine is at least its argument in size,
// a cosine at least 1, and a tangent so near its argument where that's small that
// it rounds to a value of the format, the argument at the least.

var (
	hyperbolicSine    = sineFunction(sinhCosh)
	hyperbolicCosine  = cosineFunction(sinhCosh)
	hyperbolicTangent = tangentFunction(sinhCosh)
)

// arcHyperbolicTangent is MTH$ATANH and its kind. A hyperbolic tangent is below 1
// in size, so 1 or more has no answer: MTH$_INVARGMAT.
func arcHyperbolicTangent(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
	if x.Abs().Cmp(vaxfloat.FromInt(1)) >= 0 {
		return vaxfloat.Value{}, errInvalidArg
	}

	return rounded(f, arcTanh(x.Big(), workingPrecision(f)))
}
