package mthrtl

import (
	"math/big"

	"github.com/tucats/govax/internal/vaxfloat"
)

// Square root, the exponential, and the logarithms. The limits the manual
// gives for them aren't written down here, because they follow from the
// formats: MTH$EXP's overflow "above about 88.029" is where e^x reaches 2^127,
// F_floating's limit, and rounded reports it.

// squareRoot is MTH$SQRT and its kind. A square root of a VAX value never lies
// exactly halfway between two values of its format, so rounding it needs no
// more than vaxfloat.Sqrt's bits beyond the format's precision.
func squareRoot(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
	if x.Sign() < 0 {
		return vaxfloat.Value{}, errSqrtNeg
	}

	return vaxfloat.Sqrt(f, x)
}

// exp is MTH$EXP and its kind. A result too large or too small for the
// format is rounded's report: MTH$_FLOOVEMAT, or MTH$_FLOUNDMAT for a caller
// that enabled underflow.
func exp(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
	return rounded(f, exponential(x.Big(), workingPrecision(f)))
}

// logFunction makes a logarithm from the engine's function for its base. A
// logarithm of zero or a negative number is MTH$_LOGZERNEG, and a logarithm
// of a value in range is always in range.
func logFunction(log func(*big.Float, uint) *big.Float) function {
	return func(f vaxfloat.Format, x vaxfloat.Value) (vaxfloat.Value, error) {
		if x.Sign() <= 0 {
			return vaxfloat.Value{}, errLogZeroNeg
		}

		return rounded(f, log(x.Big(), workingPrecision(f)))
	}
}

var (
	naturalLog = logFunction(logarithm)
	base2Log   = logFunction(logBase2)
	base10Log  = logFunction(logBase10)
)
