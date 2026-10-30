package mthrtl

import (
	"errors"
	"math/big"

	"github.com/tucats/govax/internal/vaxfloat"
)

// The trigonometric functions and their inverses, in radians and in degrees.
// A family's two kinds differ only in the engine's function they use, so each
// function here is made from that (as logFunction makes the logarithms).

// sinCosFunction is one of the engine's functions from a number to its sine and
// cosine: sinCos, of an angle in radians; sinCosDegrees; or sinhCosh, the
// hyperbolic pair, which makes its routines the same way (hyperbolic.go).
type sinCosFunction func(x *big.Float, prec uint) (sine, cosine *big.Float)

// sineFunction makes MTH$SIN or MTH$SIND's kind. An F, D, G, or H value in
// radians can't have a sine too small for its format, its sine being about the
// angle itself. In degrees it can, the sine being about a sixtieth of the
// angle: rounded reports that, MTH$_FLOUNDMAT for a caller that enabled
// underflow.
func sineFunction(pair sinCosFunction) function {
	return func(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
		sine, _ := pair(x.Big(), workingPrecision(f))

		return rounded(f, sine)
	}
}

// cosineFunction makes MTH$COS or MTH$COSD's kind.
func cosineFunction(pair sinCosFunction) function {
	return func(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
		_, cosine := pair(x.Big(), workingPrecision(f))

		return rounded(f, cosine)
	}
}

// tangentFunction makes MTH$TAN or MTH$TAND's kind. A cosine of exactly 0, which
// only an angle of an odd multiple of 90 degrees can have, has no tangent: the
// manual's overflow, MTH$_FLOOVEMAT. No value in radians is exactly such an
// angle, and the nearest any comes to one is far from making a tangent too
// large for the format, so MTH$TAN's overflow is one the manual gives and
// rounded could report, but no argument reaches.
func tangentFunction(pair sinCosFunction) function {
	return func(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
		w := workingPrecision(f)

		sine, cosine := pair(x.Big(), w+scratchBits)
		if cosine.Sign() == 0 {
			return vaxfloat.Value{}, vaxfloat.ErrOverflow
		}

		return rounded(f, newFloat(w).Quo(sine, cosine))
	}
}

// sineCosineFunction makes MTH$SINCOS or MTH$SINCOSD's kind. A sine too small
// for the format comes back as 0.0, with vaxfloat.ErrUnderflow (pairFunction).
func sineCosineFunction(pair sinCosFunction) pairFunction {
	return func(f vaxfloat.Format, x vaxfloat.Value) (first, second vaxfloat.Value, err error) {
		sine, cosine := pair(x.Big(), workingPrecision(f))

		first, err = rounded(f, sine)

		// A cosine that isn't exactly 0 is about the distance from the angle to
		// where it would be, and no angle in a format is near enough for that
		// to be below the format's range. So rounding it never fails.
		second, _ = rounded(f, cosine)

		return first, second, err
	}
}

// arcFunction makes an inverse function from the engine's function for it. The
// last step is the angle's conversion, to radians (asRadians, which does
// nothing) or to degrees (toDegrees).
//
// bounded says the argument is a sine or a cosine, for which an absolute value
// above 1 is MTH$_INVARGMAT.
func arcFunction(arc func(*big.Float, uint) *big.Float, bounded bool, convert func(*big.Float, uint) *big.Float) function {
	return func(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
		if bounded && x.Abs().Cmp(vaxfloat.FromInt(1)) > 0 {
			return vaxfloat.Value{}, errInvalidArg
		}

		w := workingPrecision(f)

		return rounded(f, convert(arc(x.Big(), w+scratchBits), w))
	}
}

// arcTangent2Function makes MTH$ATAN2 or MTH$ATAND2's kind, from the conversion,
// as arcFunction has it. Both arguments 0 is MTH$_INVARGMAT.
//
// A small sine over a large cosine can be too small for any format. MTH$_FLOUNDMAT
// isn't among these routines' conditions, and no description says otherwise, as
// MTH$SIND's and MTH$TAND's do, so the result is 0.0 and nothing is signaled,
// whatever the caller's FU. Unconfirmed (the probe, subtask 10).
func arcTangent2Function(convert func(*big.Float, uint) *big.Float) binaryFunction {
	return func(f vaxfloat.Format, sine, cosine vaxfloat.Value) (vaxfloat.Value, error) {
		if sine.IsZero() && cosine.IsZero() {
			return vaxfloat.Value{}, errInvalidArg
		}

		w := workingPrecision(f)

		v, err := rounded(f, convert(arcTan2(sine.Big(), cosine.Big(), w+scratchBits), w))
		if errors.Is(err, vaxfloat.ErrUnderflow) {
			return vaxfloat.Value{}, nil
		}

		return v, err
	}
}

// asRadians is arcFunction's conversion for an angle that's already in radians.
func asRadians(radians *big.Float, _ uint) *big.Float { return radians }

var (
	sine              = sineFunction(sinCos)
	cosine            = cosineFunction(sinCos)
	tangent           = tangentFunction(sinCos)
	sineCosine        = sineCosineFunction(sinCos)
	sineDegrees       = sineFunction(sinCosDegrees)
	cosineDegrees     = cosineFunction(sinCosDegrees)
	tangentDegrees    = tangentFunction(sinCosDegrees)
	sineCosineDegrees = sineCosineFunction(sinCosDegrees)

	arcSine           = arcFunction(arcSin, true, asRadians)
	arcCosine         = arcFunction(arcCos, true, asRadians)
	arcTangent        = arcFunction(arcTan, false, asRadians)
	arcSineDegrees    = arcFunction(arcSin, true, toDegrees)
	arcCosineDegrees  = arcFunction(arcCos, true, toDegrees)
	arcTangentDegrees = arcFunction(arcTan, false, toDegrees)

	arcTangent2        = arcTangent2Function(asRadians)
	arcTangent2Degrees = arcTangent2Function(toDegrees)
)
