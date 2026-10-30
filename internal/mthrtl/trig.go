package mthrtl

import (
	"math/big"

	"github.com/tucats/govax/internal/vaxfloat"
)

// The trigonometric functions and their inverses, in radians and in degrees.
// A family's two kinds differ only in the engine's function they use, so each
// function here is made from that (as logFunction makes the logarithms).

// angleFunction is one of the engine's functions from an angle to its sine and
// cosine: sinCos, for radians, or sinCosDegrees.
type angleFunction func(x *big.Float, prec uint) (sine, cosine *big.Float)

// sineFunction makes MTH$SIN or MTH$SIND's kind. An F, D, G, or H value in
// radians can't have a sine too small for its format, its sine being about the
// angle itself. In degrees it can, the sine being about a sixtieth of the
// angle: rounded reports that, MTH$_FLOUNDMAT for a caller that enabled
// underflow.
func sineFunction(angle angleFunction) function {
	return func(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
		sine, _ := angle(x.Big(), workingPrecision(f))

		return rounded(f, sine)
	}
}

// cosineFunction makes MTH$COS or MTH$COSD's kind.
func cosineFunction(angle angleFunction) function {
	return func(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
		_, cosine := angle(x.Big(), workingPrecision(f))

		return rounded(f, cosine)
	}
}

// tangentFunction makes MTH$TAN or MTH$TAND's kind. A cosine of exactly 0, which
// only an angle of an odd multiple of 90 degrees can have, has no tangent: the
// manual's overflow, MTH$_FLOOVEMAT. No value in radians is exactly such an
// angle, and the nearest any comes to one is far from making a tangent too
// large for the format, so MTH$TAN's overflow is one the manual gives and
// rounded could report, but no argument reaches.
func tangentFunction(angle angleFunction) function {
	return func(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
		w := workingPrecision(f)

		sine, cosine := angle(x.Big(), w+scratchBits)
		if cosine.Sign() == 0 {
			return vaxfloat.Value{}, vaxfloat.ErrOverflow
		}

		return rounded(f, newFloat(w).Quo(sine, cosine))
	}
}

// sineCosineFunction makes MTH$SINCOS or MTH$SINCOSD's kind. A sine too small
// for the format comes back as 0.0, with vaxfloat.ErrUnderflow (pairFunction).
func sineCosineFunction(angle angleFunction) pairFunction {
	return func(f vaxfloat.Format, x vaxfloat.Value) (first, second vaxfloat.Value, err error) {
		sine, cosine := angle(x.Big(), workingPrecision(f))

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
)
