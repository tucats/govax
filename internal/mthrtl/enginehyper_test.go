package mthrtl

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/tucats/govax/internal/vaxfloat"
)

// Values to 40 digits or more, published constants, none of them from this
// package's arithmetic.
const (
	digitsSinh1     = "1.1752011936438014568823818505956008151557179813340958702295654130"
	digitsCosh1     = "1.5430806348152437784779056207570616826015291123658637047374022147"
	digitsTanh1     = "0.7615941559557648881194582826047935904127685972579365515968105001"
	digitsAtanhHalf = "0.5493061443340548456976226184612628523237452789113747258673471668"
)

// The reference: the definitions, with referenceExp at far more bits than a
// format's result needs. A difference of exponentials cancels about as many bits
// as x is below 1, so for a tiny x the reference is the first terms of the power
// series instead, each next term too small to reach a format's rounding.
const referenceTinyExponent = -64

// referencePolynomial is x + a * x^3 + b * x^5.
func referencePolynomial(x *big.Float, a, b *big.Rat) *big.Float {
	cube := newFloat(referenceBits).Mul(x, newFloat(referenceBits).Mul(x, x))
	fifth := newFloat(referenceBits).Mul(cube, newFloat(referenceBits).Mul(x, x))

	sum := newFloat(referenceBits).Set(x)
	sum.Add(sum, newFloat(referenceBits).Mul(cube, newFloat(referenceBits).SetRat(a)))

	return sum.Add(sum, newFloat(referenceBits).Mul(fifth, newFloat(referenceBits).SetRat(b)))
}

func referenceSinh(x *big.Float) *big.Float {
	if x.Sign() != 0 && x.MantExp(nil) < referenceTinyExponent {
		return referencePolynomial(x, big.NewRat(1, 6), big.NewRat(1, 120))
	}

	e := referenceExp(x)
	difference := newFloat(referenceBits).Sub(e, newFloat(referenceBits).Quo(big.NewFloat(1), e))

	return difference.SetMantExp(difference, -1)
}

func referenceCosh(x *big.Float) *big.Float {
	e := referenceExp(x)
	sum := newFloat(referenceBits).Add(e, newFloat(referenceBits).Quo(big.NewFloat(1), e))

	return sum.SetMantExp(sum, -1)
}

func referenceTanh(x *big.Float) *big.Float {
	if x.Sign() != 0 && x.MantExp(nil) < referenceTinyExponent {
		return referencePolynomial(x, big.NewRat(-1, 3), big.NewRat(2, 15))
	}

	// (e^2x - 1) / (e^2x + 1), which isn't how the code does it. It keeps the
	// bits of 1 - tanh x as x grows, which sinh / cosh wouldn't have at this
	// length for the largest x.
	e := referenceExp(newFloat(referenceBits).Mul(x, big.NewFloat(2)))
	one := big.NewFloat(1)

	return newFloat(referenceBits).Quo(newFloat(referenceBits).Sub(e, one), newFloat(referenceBits).Add(e, one))
}

// referenceAtanh is the logarithm at every size above the tiny, where the code
// sums a series below 1/8.
func referenceAtanh(x *big.Float) *big.Float {
	if x.Sign() != 0 && x.MantExp(nil) < referenceTinyExponent {
		return referencePolynomial(x, big.NewRat(1, 3), big.NewRat(1, 5))
	}

	one := big.NewFloat(1)
	ratio := newFloat(referenceBits).Add(one, x)
	ratio.Quo(ratio, newFloat(referenceBits).Sub(one, x))

	result := referenceLog(ratio)

	return result.SetMantExp(result, -1)
}

// referenceAtan2 adds pi to the arc tangent of the quotient, at the reference's
// length, and gives the cases the code doesn't divide for their answers. The
// test's known values check the rule itself.
func referenceAtan2(y, x *big.Float) *big.Float {
	w := uint(trigReferenceBits)
	half := newFloat(w).Set(referencePi())
	half.SetMantExp(half, -1)

	if x.Sign() == 0 {
		if y.Signbit() {
			half.Neg(half)
		}

		return half
	}

	angle := referenceAtan(newFloat(w).Quo(y, x))

	if x.Signbit() {
		whole := half.SetMantExp(half, 1)
		if y.Signbit() {
			whole.Neg(whole)
		}

		angle.Add(angle, whole)
	}

	return angle
}

// hyperbolicArgument is one of the kinds of argument that matter to a hyperbolic
// function: across the format's range of results and a little out of it, a
// moderate size either side of 1 where the code changes method, or tiny.
func hyperbolicArgument(rng *rand.Rand, f vaxfloat.Format, i int) vaxfloat.Value {
	_, high := formatExponents(f)
	limit := float64(high) * 0.7

	switch i % 4 {
	case 0:
		x, _ := vaxfloat.Round(f, vaxfloat.FromFloat64((rng.Float64()*2-1)*limit*1.02))

		return x
	case 1:
		return randomValue(rng, f, rng.IntN(6)-2, true)
	case 2:
		// Just either side of 1.
		d := randomValue(rng, f, -rng.IntN(int(f.Precision())), true)
		x, _ := vaxfloat.Add(f, vaxfloat.FromInt(1), d)

		return x
	default:
		low, _ := formatExponents(f)
		if i%8 == 3 {
			return randomValue(rng, f, low+rng.IntN(64), true)
		}

		return randomValue(rng, f, -rng.IntN(2*int(f.Precision())), true)
	}
}

func TestHyperbolicAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 11))

	for _, f := range allFormats {
		for i := range 400 {
			x := hyperbolicArgument(rng, f, i)

			got, err := hyperbolicSine(f, x)
			expectRounded(t, "SINH", f, x, got, err, referenceSinh(x.Big()))

			got, err = hyperbolicCosine(f, x)
			expectRounded(t, "COSH", f, x, got, err, referenceCosh(x.Big()))

			got, err = hyperbolicTangent(f, x)
			expectRounded(t, "TANH", f, x, got, err, referenceTanh(x.Big()))
		}
	}
}

// TestArcTanhAgainstReference: the whole domain, each side of 1/8 where the code
// changes method, and each end, where the argument's last bit matters most.
func TestArcTanhAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 12))

	for _, f := range allFormats {
		low, _ := formatExponents(f)
		p := int(f.Precision())

		for i := range 300 {
			var x vaxfloat.Value

			switch i % 4 {
			case 0:
				x = randomValue(rng, f, -rng.IntN(p+8), true)
			case 1:
				x = randomValue(rng, f, -rng.IntN(5), true)
			case 2:
				// Just inside an end. The smallest d is an ulp below 1.
				d := randomValue(rng, f, -rng.IntN(p-1), false)
				x, _ = vaxfloat.Sub(f, vaxfloat.FromInt(1), d)

				if rng.UintN(2) == 0 {
					x = x.Neg()
				}
			default:
				x = randomValue(rng, f, low+rng.IntN(p), true)
			}

			if x.Abs().Cmp(vaxfloat.FromInt(1)) >= 0 {
				continue
			}

			got, err := arcHyperbolicTangent(f, x)
			expectRounded(t, "ATANH", f, x, got, err, referenceAtanh(x.Big()))
		}
	}
}

// TestArcTan2AgainstReference: all four quadrants, with the two arguments close
// in size and as far apart as a format lets them be. A quotient below the format
// is 0.0 with no error (see arcTangent2Function).
func TestArcTan2AgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 13))

	for _, f := range allFormats {
		low, high := formatExponents(f)
		p := int(f.Precision())

		for i := range 200 {
			var y, x vaxfloat.Value

			switch i % 4 {
			case 0:
				y = randomValue(rng, f, low+rng.IntN(high-low+1), true)
				x = randomValue(rng, f, low+rng.IntN(high-low+1), true)
			case 1:
				e := rng.IntN(40) - 20
				y = randomValue(rng, f, e+rng.IntN(4), true)
				x = randomValue(rng, f, e, true)
			case 2:
				// A small angle, or one near pi/2, by about the format's length.
				e := low + p + 8 + rng.IntN(high-low-2*p-16)
				y = randomValue(rng, f, e, true)
				x = randomValue(rng, f, e+rng.IntN(2*p)-p, true)
			default:
				// An exact quotient, a power of 2.
				y = randomValue(rng, f, 0, true)
				x, _ = vaxfloat.Round(f, vaxfloat.FromBig(new(big.Float).SetMantExp(y.Big(), rng.IntN(20)-10)))
			}

			reference := referenceAtan2(y.Big(), x.Big())

			for _, c := range []struct {
				name      string
				fn        binaryFunction
				reference *big.Float
			}{
				{"ATAN2", arcTangent2, reference},
				{"ATAND2", arcTangent2Degrees, referenceToDegrees(reference)},
			} {
				want, werr := rounded(f, c.reference)
				if errors.Is(werr, vaxfloat.ErrUnderflow) {
					want = vaxfloat.Value{}
				} else if werr != nil {
					t.Fatalf("%s %v(%s, %s): reference: %v", c.name, f, y, x, werr)
				}

				if got, err := c.fn(f, y, x); err != nil || got.Cmp(want) != 0 {
					t.Errorf("%s %v(%s, %s) = %s, %v; want %s", c.name, f, y, x, got, err, want)
				}
			}
		}
	}
}

// TestKnownHyperbolic: published digits, in every format, and each function's
// symmetry.
func TestKnownHyperbolic(t *testing.T) {
	for _, f := range allFormats {
		for _, c := range []struct {
			name   string
			fn     function
			arg    string
			digits string
			odd    bool
		}{
			{"SINH", hyperbolicSine, "1", digitsSinh1, true},
			{"COSH", hyperbolicCosine, "1", digitsCosh1, false},
			{"TANH", hyperbolicTangent, "1", digitsTanh1, true},
			{"ATANH", arcHyperbolicTangent, "0.5", digitsAtanhHalf, true},
		} {
			want := value(t, f, c.digits)
			arg := value(t, f, c.arg)

			if got, err := c.fn(f, arg); err != nil || got.Cmp(want) != 0 {
				t.Errorf("%s %v(%s) = %s, %v; want %s", c.name, f, c.arg, got, err, want)
			}

			if c.odd {
				want = want.Neg()
			}

			if got, err := c.fn(f, arg.Neg()); err != nil || got.Cmp(want) != 0 {
				t.Errorf("%s %v(-%s) = %s, %v; want %s", c.name, f, c.arg, got, err, want)
			}
		}
	}
}

// TestHyperbolicAtTheEnds: exact at 0; at the format's smallest value, the
// argument or 1, the manual's first case; a tangent's 1 far out.
func TestHyperbolicAtTheEnds(t *testing.T) {
	for _, f := range allFormats {
		tiny := smallest(f)
		one := vaxfloat.FromInt(1)

		for _, c := range []struct {
			name string
			fn   function
			arg  vaxfloat.Value
			want vaxfloat.Value
		}{
			{"SINH", hyperbolicSine, vaxfloat.Value{}, vaxfloat.Value{}},
			{"COSH", hyperbolicCosine, vaxfloat.Value{}, one},
			{"TANH", hyperbolicTangent, vaxfloat.Value{}, vaxfloat.Value{}},
			{"ATANH", arcHyperbolicTangent, vaxfloat.Value{}, vaxfloat.Value{}},

			{"SINH", hyperbolicSine, tiny, tiny},
			{"SINH", hyperbolicSine, tiny.Neg(), tiny.Neg()},
			{"COSH", hyperbolicCosine, tiny, one},
			{"COSH", hyperbolicCosine, tiny.Neg(), one},
			{"TANH", hyperbolicTangent, tiny, tiny},
			{"TANH", hyperbolicTangent, tiny.Neg(), tiny.Neg()},
			{"ATANH", arcHyperbolicTangent, tiny, tiny},
			{"ATANH", arcHyperbolicTangent, tiny.Neg(), tiny.Neg()},
		} {
			if got, err := c.fn(f, c.arg); err != nil || got.Cmp(c.want) != 0 {
				t.Errorf("%s %v(%s) = %s, %v; want %s", c.name, f, c.arg, got, err, c.want)
			}
		}

		_, high := formatExponents(f)
		largest := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), high-1))

		if got, err := hyperbolicTangent(f, largest); err != nil || got.Cmp(one) != 0 {
			t.Errorf("TANH %v(%s) = %s, %v", f, largest, got, err)
		}

		if got, err := hyperbolicTangent(f, largest.Neg()); err != nil || got.Cmp(one.Neg()) != 0 {
			t.Errorf("TANH %v(-%s) = %s, %v", f, largest, got, err)
		}
	}
}

// TestTanhManualsLimit: the manual's h, past which the hyperbolic tangent is 1
// with its sign.
func TestTanhManualsLimit(t *testing.T) {
	for _, c := range []struct {
		f vaxfloat.Format
		h int64
	}{{vaxfloat.F, 10}, {vaxfloat.D, 21}, {vaxfloat.G, 20}, {vaxfloat.H, 40}} {
		if got, err := hyperbolicTangent(c.f, vaxfloat.FromInt(c.h)); err != nil || got.Cmp(vaxfloat.FromInt(1)) != 0 {
			t.Errorf("TANH %v(%d) = %s, %v", c.f, c.h, got, err)
		}

		if got, err := hyperbolicTangent(c.f, vaxfloat.FromInt(-c.h)); err != nil || got.Cmp(vaxfloat.FromInt(-1)) != 0 {
			t.Errorf("TANH %v(%d) = %s, %v", c.f, -c.h, got, err)
		}
	}
}

// TestHyperbolicLimits: MTH$_FLOOVEMAT as the manual gives it, approximately, on
// each side and with each sign, and far past it, where exponential doesn't
// compute.
func TestHyperbolicLimits(t *testing.T) {
	for _, c := range []struct {
		f          vaxfloat.Format
		fits, over string
	}{
		{vaxfloat.F, "88.7228", "88.7229"},
		{vaxfloat.D, "88.7228", "88.7229"},
		{vaxfloat.G, "709.782", "709.783"},
		{vaxfloat.H, "11356.523", "11356.524"},
	} {
		_, high := formatExponents(c.f)
		largest := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), high-1))

		for _, fn := range []struct {
			name string
			fn   function
		}{{"SINH", hyperbolicSine}, {"COSH", hyperbolicCosine}} {
			fits, over := value(t, c.f, c.fits), value(t, c.f, c.over)

			for _, x := range []vaxfloat.Value{fits, fits.Neg()} {
				if v, err := fn.fn(c.f, x); err != nil || v.IsZero() {
					t.Errorf("%s %v(%s) = %s, %v", fn.name, c.f, x, v, err)
				}
			}

			for _, x := range []vaxfloat.Value{over, over.Neg(), largest, largest.Neg()} {
				if _, err := fn.fn(c.f, x); !errors.Is(err, vaxfloat.ErrOverflow) {
					t.Errorf("%s %v(%s): %v", fn.name, c.f, x, err)
				}
			}
		}
	}
}

// TestArcTanhDomain: 1 and beyond have no answer, and the value just below 1
// does, about ln(2^p + ...) / 2.
func TestArcTanhDomain(t *testing.T) {
	for _, f := range allFormats {
		ulp := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), 1-int(f.Precision())))
		beyond, err := vaxfloat.Add(f, vaxfloat.FromInt(1), ulp)
		if err != nil {
			t.Fatal(err)
		}

		for _, x := range []vaxfloat.Value{vaxfloat.FromInt(1), vaxfloat.FromInt(-1), beyond, beyond.Neg()} {
			if _, err := arcHyperbolicTangent(f, x); !errors.Is(err, errInvalidArg) {
				t.Errorf("ATANH %v(%s): %v", f, x, err)
			}
		}

		below := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), -int(f.Precision())))

		x, err := vaxfloat.Sub(f, vaxfloat.FromInt(1), below)
		if err != nil {
			t.Fatal(err)
		}

		got, err := arcHyperbolicTangent(f, x)
		expectRounded(t, "ATANH", f, x, got, err, referenceAtanh(x.Big()))
	}
}

// TestArcTan2Known: every quadrant and axis, exact in degrees, in every format.
func TestArcTan2Known(t *testing.T) {
	pi := decimal(t, digitsPi)
	fraction := func(n, d int64) *big.Float {
		product := newFloat(1024).Mul(pi, newFloat(1024).SetInt64(n))

		return product.Quo(product, newFloat(1024).SetInt64(d))
	}

	for _, f := range allFormats {
		for _, c := range []struct {
			y, x    int64
			radians *big.Float
			degrees int64
		}{
			{1, 1, fraction(1, 4), 45},
			{1, -1, fraction(3, 4), 135},
			{-1, -1, fraction(-3, 4), -135},
			{-1, 1, fraction(-1, 4), -45},
			{3, 3, fraction(1, 4), 45},
			{1, 0, fraction(1, 2), 90},
			{-1, 0, fraction(-1, 2), -90},
			{0, -1, fraction(1, 1), 180},
			{0, 1, new(big.Float), 0},
			{0, -1000, fraction(1, 1), 180},
		} {
			y, x := vaxfloat.FromInt(c.y), vaxfloat.FromInt(c.x)
			want := referenceRounded(t, f, c.radians)

			if got, err := arcTangent2(f, y, x); err != nil || got.Cmp(want) != 0 {
				t.Errorf("ATAN2 %v(%d, %d) = %s, %v; want %s", f, c.y, c.x, got, err, want)
			}

			if got, err := arcTangent2Degrees(f, y, x); err != nil || got.Cmp(vaxfloat.FromInt(c.degrees)) != 0 {
				t.Errorf("ATAND2 %v(%d, %d) = %s, %v; want %d", f, c.y, c.x, got, err, c.degrees)
			}
		}

		// atan(2), and pi less it.
		two, minusOne := vaxfloat.FromInt(2), vaxfloat.FromInt(-1)
		atan2 := decimal(t, digitsAtan2)

		if got, err := arcTangent2(f, two, vaxfloat.FromInt(1)); err != nil || got.Cmp(referenceRounded(t, f, atan2)) != 0 {
			t.Errorf("ATAN2 %v(2, 1) = %s, %v", f, got, err)
		}

		rest := newFloat(1024).Sub(pi, atan2)
		if got, err := arcTangent2(f, two, minusOne); err != nil || got.Cmp(referenceRounded(t, f, rest)) != 0 {
			t.Errorf("ATAN2 %v(2, -1) = %s, %v", f, got, err)
		}
	}
}

// TestArcTan2Extremes: both arguments 0 has no answer. A quotient too small for
// the format is 0.0 with no error, or pi with y's sign, and a quotient too large
// for any format is pi/2.
func TestArcTan2Extremes(t *testing.T) {
	for _, f := range allFormats {
		zero := vaxfloat.Value{}
		one := vaxfloat.FromInt(1)
		tiny := smallest(f)

		_, high := formatExponents(f)
		large := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), high-1))

		for _, fn := range []binaryFunction{arcTangent2, arcTangent2Degrees} {
			if _, err := fn(f, zero, zero); !errors.Is(err, errInvalidArg) {
				t.Errorf("%v(0, 0): %v", f, err)
			}
		}

		half, _ := rounded(f, halfReferencePi())
		whole, _ := rounded(f, referencePi())
		ninety, oneEighty := vaxfloat.FromInt(90), vaxfloat.FromInt(180)

		for _, c := range []struct {
			y, x             vaxfloat.Value
			radians, degrees vaxfloat.Value
		}{
			{tiny, large, zero, zero},
			{tiny.Neg(), large, zero, zero},
			{tiny, large.Neg(), whole, oneEighty},
			{tiny.Neg(), large.Neg(), whole.Neg(), oneEighty.Neg()},
			{large, tiny, half, ninety},
			{large, tiny.Neg(), half, ninety},
			{large.Neg(), tiny, half.Neg(), ninety.Neg()},
			{large.Neg(), tiny.Neg(), half.Neg(), ninety.Neg()},
			{tiny, zero, half, ninety},
			{large, zero, half, ninety},
			{zero, tiny.Neg(), whole, oneEighty},
			{zero, large, zero, zero},
		} {
			if got, err := arcTangent2(f, c.y, c.x); err != nil || got.Cmp(c.radians) != 0 {
				t.Errorf("ATAN2 %v(%s, %s) = %s, %v; want %s", f, c.y, c.x, got, err, c.radians)
			}

			if got, err := arcTangent2Degrees(f, c.y, c.x); err != nil || got.Cmp(c.degrees) != 0 {
				t.Errorf("ATAND2 %v(%s, %s) = %s, %v; want %s", f, c.y, c.x, got, err, c.degrees)
			}
		}

		// Small but in range: the quotient itself, in radians.
		if got, err := arcTangent2(f, tiny, one); err != nil || got.Cmp(tiny) != 0 {
			t.Errorf("ATAN2 %v(%s, 1) = %s, %v", f, tiny, got, err)
		}
	}
}

// BenchmarkHyperbolic measures a call in each format, either side of where the
// code changes method, against the survey's estimate of tens of VAX
// instructions' time.
func BenchmarkHyperbolic(b *testing.B) {
	for _, f := range allFormats {
		small, _ := vaxfloat.Round(f, vaxfloat.FromFloat64(0.5))
		large, _ := vaxfloat.Round(f, vaxfloat.FromFloat64(5.5))

		for _, c := range []struct {
			name string
			fn   function
			arg  vaxfloat.Value
		}{
			{"SINH0.5", hyperbolicSine, small},
			{"SINH5.5", hyperbolicSine, large},
			{"COSH5.5", hyperbolicCosine, large},
			{"TANH5.5", hyperbolicTangent, large},
			{"ATANH0.5", arcHyperbolicTangent, small},
		} {
			b.Run(fmt.Sprintf("%s/%v", c.name, f), func(b *testing.B) {
				for b.Loop() {
					if _, err := c.fn(f, c.arg); err != nil {
						b.Fatal(err)
					}
				}
			})
		}

		b.Run(fmt.Sprintf("ATAN2/%v", f), func(b *testing.B) {
			for b.Loop() {
				if _, err := arcTangent2(f, large, small.Neg()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
