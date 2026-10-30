package mthrtl

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/tucats/govax/internal/vaxfloat"
)

// Values to about 100 digits, published constants, none of them from this
// package's arithmetic.
const (
	digitsPi     = "3.14159265358979323846264338327950288419716939937510582097494459230781640628620899862803482534211707"
	digitsSin1   = "0.841470984807896506652502321630298999622563060798371065672751709991910404391239668948639743543052695"
	digitsCos1   = "0.540302305868139717400936607442976603732310420617922227670682"
	digitsTan1   = "1.55740772465490223050697480745836017308725077238152003838394660569886139715172728955509996520224298"
	digitsSinDeg = "0.0174524064372835128194189785163161924722527203071396426836124276"
	digitsCosDeg = "0.999847695156391239157011558813914852733301670644968575"
	digitsAtan2  = "1.10714871779409050301706546017853704007004764540419"
)

// The reference: the functions by other means, at far more bits than any
// format's result needs to be rounded correctly. Pi is Gauss and Legendre's
// iteration, not a series; an angle is reduced by 2*pi with no quadrants, and
// goes straight into the Taylor series however large what's left of it is; the
// arc tangent solves tan y = x by iteration, from a start from float64.

const (
	trigReferenceBits = 1024

	// piReferenceBits is enough for H_floating's largest angles and all the
	// reference's bits left over.
	piReferenceBits = 18000
)

// referencePi is pi by the Gauss-Legendre iteration, each step doubling its
// correct digits, made once since it's long.
var referencePi = sync.OnceValue(func() *big.Float {
	w := uint(piReferenceBits + 64)
	a := newFloat(w).SetInt64(1)
	b := newFloat(w).Quo(a, newFloat(w).Sqrt(newFloat(w).SetInt64(2)))
	t := newFloat(w).Quo(a, newFloat(w).SetInt64(4))
	p := newFloat(w).SetInt64(1)

	for range 16 {
		next := newFloat(w).Add(a, b)
		next.SetMantExp(next, -1)

		difference := newFloat(w).Sub(a, next)
		difference.Mul(difference, difference)
		t.Sub(t, difference.Mul(difference, p))

		b.Sqrt(newFloat(w).Mul(a, b))
		a = next
		p.SetMantExp(p, 1)
	}

	sum := newFloat(w).Add(a, b)
	sum.Mul(sum, sum)
	t.SetMantExp(t, 2)

	return sum.Quo(sum, t)
})

// referenceTaylor sums sin and cos of r directly, for any r of moderate size.
// A tiny r's sine is good relative to r: the sum starts from r itself, and
// stops relative to it too.
func referenceTaylor(r *big.Float) (sine, cosine *big.Float) {
	w := uint(trigReferenceBits)
	sine = newFloat(w).Set(r)
	cosine = newFloat(w).SetInt64(1)

	if r.Sign() == 0 {
		return sine, cosine
	}

	smallest := min(r.MantExp(nil), 0) - int(w) - 8
	term := newFloat(w).Set(r)

	for k := int64(2); ; k++ {
		term.Mul(term, r)
		term.Quo(term, newFloat(w).SetInt64(k))

		// Past k = r the terms only fall.
		if k > 16 && (term.Sign() == 0 || term.MantExp(nil) < smallest) {
			return sine, cosine
		}

		switch k % 4 {
		case 0:
			cosine.Add(cosine, term)
		case 1:
			sine.Add(sine, term)
		case 2:
			cosine.Sub(cosine, term)
		case 3:
			sine.Sub(sine, term)
		}
	}
}

// referenceSinCos reduces x by 2*pi at pi's whole length and sums what's left.
func referenceSinCos(x *big.Float) (sine, cosine *big.Float) {
	if new(big.Float).Abs(x).Cmp(big.NewFloat(6)) < 0 {
		return referenceTaylor(x)
	}

	w := uint(piReferenceBits)
	twoPi := newFloat(w).Set(referencePi())
	twoPi.SetMantExp(twoPi, 1)

	q := newFloat(w).Quo(x, twoPi)
	n, _ := q.Int(nil)

	r := newFloat(w).Mul(newFloat(w).SetInt(n), twoPi)
	r.Sub(x, r)

	return referenceTaylor(r)
}

// referenceDegrees reduces x by 360 in rational arithmetic, and gives a
// multiple of 90 its exact answers, as a rational can show it is.
func referenceDegrees(x *big.Float) (sine, cosine *big.Float) {
	rat, _ := x.Rat(nil)

	circle := new(big.Rat).Mul(new(big.Rat).SetFrac(rat.Denom(), big.NewInt(1)), big.NewRat(360, 1))
	remainder := new(big.Int).Rem(rat.Num(), circle.Num())
	reduced := new(big.Rat).SetFrac(remainder, rat.Denom())

	quarters := new(big.Rat).Quo(reduced, big.NewRat(90, 1))
	if quarters.IsInt() {
		quadrant := new(big.Int).Mod(quarters.Num(), big.NewInt(4)).Int64()
		s := []int64{0, 1, 0, -1}[quadrant]
		c := []int64{1, 0, -1, 0}[quadrant]

		return newFloat(64).SetInt64(s), newFloat(64).SetInt64(c)
	}

	w := uint(trigReferenceBits + 64)
	r := newFloat(w).SetRat(reduced)
	r.Mul(r, referencePi())

	return referenceTaylor(r.Quo(r, newFloat(w).SetInt64(180)))
}

// referenceAtan solves tan y = x: y gains the angle whose tangent is
// (x - tan y) / (1 + x tan y), which is the error, to within its cube.
func referenceAtan(x *big.Float) *big.Float {
	w := uint(trigReferenceBits)
	f, _ := x.Float64()

	y := newFloat(w).SetFloat64(math.Atan(f))

	for range 5 {
		s, c := referenceTaylor(y)
		number := newFloat(w).Sub(newFloat(w).Mul(x, c), s)
		denominator := newFloat(w).Add(c, newFloat(w).Mul(x, s))
		y.Add(y, number.Quo(number, denominator))
	}

	return y
}

// referenceAsin is the atan form at the reference's length, and pi/2 at the
// ends.
func referenceAsin(x *big.Float) *big.Float {
	w := uint(trigReferenceBits)
	one := big.NewFloat(1)

	product := newFloat(w).Mul(newFloat(w).Sub(one, x), newFloat(w).Add(one, x))
	if product.Sign() == 0 {
		half := newFloat(w).Set(referencePi())
		half.SetMantExp(half, -1)

		if x.Signbit() {
			half.Neg(half)
		}

		return half
	}

	return referenceAtan(newFloat(w).Quo(x, newFloat(w).Sqrt(product)))
}

// referenceAcos is pi/2 less the arc sine, which isn't how the code does it.
func referenceAcos(x *big.Float) *big.Float {
	w := uint(trigReferenceBits)
	half := newFloat(w).Set(referencePi())
	half.SetMantExp(half, -1)

	return half.Sub(half, referenceAsin(x))
}

// referenceToDegrees multiplies by 180/pi.
func referenceToDegrees(x *big.Float) *big.Float {
	w := uint(trigReferenceBits)
	product := newFloat(w).Mul(x, newFloat(w).SetInt64(180))

	return product.Quo(product, referencePi())
}

// TestReferencePi: the reference's pi has the published digits, and the
// constant, at each size it's asked for, the reference's bits.
func TestReferencePi(t *testing.T) {
	if !closeTo(referencePi(), decimal(t, digitsPi), 320) {
		t.Errorf("the reference's pi is %s", referencePi().Text('g', 110))
	}

	for _, bits := range []uint{64, 300, 1024, 5000, 17000} {
		if got := pi.at(bits); got.Prec() < bits || !closeTo(got, referencePi(), int(bits)-2) {
			t.Errorf("pi at %d bits is off", bits)
		}
	}
}

// TestConstantGrows: asked for more than it has, a constant at least doubles
// it, so a series of larger requests doesn't recompute each time.
func TestConstantGrows(t *testing.T) {
	computed := 0
	c := constant{compute: func(prec uint) *big.Float {
		computed++

		return newFloat(prec).SetInt64(1)
	}}

	for bits := uint(constantFloor - 10); bits < 2000; bits += 10 {
		c.at(bits)
	}

	if computed > 5 {
		t.Errorf("computed %d times", computed)
	}
}

// TestUsualAnglesUnderConstantFloor: at H's working precision, an argument that
// isn't large and doesn't cancel needs no more of pi than its first computation.
func TestUsualAnglesUnderConstantFloor(t *testing.T) {
	if most := workingPrecision(vaxfloat.H) + 3*scratchBits + reductionSlack + 8; most > constantFloor {
		t.Errorf("the engine can ask for %d bits, above the constants' floor", most)
	}
}

// angleCases makes test values of format f, in six kinds, by i: moderate,
// tiny, all across the format's range, very near a multiple of pi/2 (where a
// reduction cancels), a smallish multiple plus nothing, and near pi/4, where
// reduceRadians's first test falls.
func angleCases(rng *rand.Rand, f vaxfloat.Format, i int) vaxfloat.Value {
	low, high := formatExponents(f)
	p := int(f.Precision())

	switch i % 6 {
	case 0:
		return randomValue(rng, f, rng.IntN(11)-2, true)
	case 1:
		return randomValue(rng, f, -rng.IntN(p+10), true)
	case 2:
		return randomValue(rng, f, low+rng.IntN(high-low+1), true)
	case 3:
		k := randomValue(rng, f, rng.IntN(60), false).Int(false)
		x, _ := rounded(f, newFloat(piReferenceBits).Mul(newFloat(piReferenceBits).SetInt(k), halfReferencePi()))

		return x
	case 4:
		x, _ := rounded(f, newFloat(piReferenceBits).Mul(newFloat(64).SetInt64(1+rng.Int64N(16)), halfReferencePi()))

		return x
	default:
		x, _ := rounded(f, newFloat(trigReferenceBits).SetFloat64(0.7853+rng.Float64()/1000))

		return x
	}
}

func halfReferencePi() *big.Float {
	half := newFloat(piReferenceBits).Set(referencePi())

	return half.SetMantExp(half, -1)
}

// TestSinCosAgainstReference: sine, cosine, tangent, and the pair together, in
// every format.
func TestSinCosAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 6))

	for _, f := range allFormats {
		for i := range 360 {
			x := angleCases(rng, f, i)
			refSine, refCosine := referenceSinCos(x.Big())

			got, err := sine(f, x)
			expectRounded(t, "SIN", f, x, got, err, refSine)

			got, err = cosine(f, x)
			expectRounded(t, "COS", f, x, got, err, refCosine)

			got, err = tangent(f, x)
			expectRounded(t, "TAN", f, x, got, err, newFloat(trigReferenceBits).Quo(refSine, refCosine))

			first, second, err := sineCosine(f, x)
			expectRounded(t, "SINCOS's sine", f, x, first, err, refSine)
			expectRounded(t, "SINCOS's cosine", f, x, second, nil, refCosine)
		}
	}
}

// degreeCases makes test values of format f, by i: moderate, tiny (down below
// where the sine underflows), across the range, an odd multiple of 45 a very
// little off, and a multiple of 30 or 45 exactly.
func degreeCases(rng *rand.Rand, f vaxfloat.Format, i int) vaxfloat.Value {
	low, high := formatExponents(f)
	p := int(f.Precision())

	switch i % 6 {
	case 0:
		return randomValue(rng, f, rng.IntN(14), true)
	case 1:
		return randomValue(rng, f, -rng.IntN(p+10), true)
	case 2:
		return randomValue(rng, f, low+rng.IntN(high-low+1), true)
	case 3:
		return randomValue(rng, f, low+rng.IntN(12), false)
	case 4:
		base := vaxfloat.FromInt(45 * (1 + 2*rng.Int64N(16)))
		d := randomValue(rng, f, 6-rng.IntN(p), true)
		x, _ := vaxfloat.Add(f, base, d)

		return x
	default:
		x, _ := vaxfloat.Round(f, vaxfloat.FromInt(rng.Int64N(40000)*15-300000))

		return x
	}
}

// TestDegreesAgainstReference: the degree forms. An underflow's error is
// compared too, as is a tangent's overflow, where the reference's cosine is 0.
func TestDegreesAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 7))

	for _, f := range allFormats {
		for i := range 600 {
			x := degreeCases(rng, f, i)
			refSine, refCosine := referenceDegrees(x.Big())

			got, err := sineDegrees(f, x)
			expectRounded(t, "SIND", f, x, got, err, refSine)

			got, err = cosineDegrees(f, x)
			expectRounded(t, "COSD", f, x, got, err, refCosine)

			got, err = tangentDegrees(f, x)
			if refCosine.Sign() == 0 {
				if !errors.Is(err, vaxfloat.ErrOverflow) {
					t.Errorf("TAND %v(%s) = %s, %v, want an overflow", f, x, got, err)
				}
			} else {
				expectRounded(t, "TAND", f, x, got, err, newFloat(trigReferenceBits).Quo(refSine, refCosine))
			}

			first, second, err := sineCosineDegrees(f, x)
			expectRounded(t, "SINCOSD's sine", f, x, first, err, refSine)
			expectRounded(t, "SINCOSD's cosine", f, x, second, nil, refCosine)
		}
	}
}

// TestArcTanAgainstReference: all across each format's range, and around 1
// and the point where the code changes how many times it halves.
func TestArcTanAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 8))

	for _, f := range allFormats {
		low, high := formatExponents(f)
		p := int(f.Precision())

		for i := range 200 {
			var x vaxfloat.Value

			switch i % 4 {
			case 0:
				x = randomValue(rng, f, low+rng.IntN(high-low+1), true)
			case 1:
				x = randomValue(rng, f, rng.IntN(9)-4, true)
			case 2:
				d := randomValue(rng, f, -rng.IntN(p), true)
				x, _ = vaxfloat.Add(f, vaxfloat.FromInt(1), d)
			default:
				x = randomValue(rng, f, -rng.IntN(p+8), true)
			}

			reference := referenceAtan(x.Big())

			got, err := arcTangent(f, x)
			expectRounded(t, "ATAN", f, x, got, err, reference)

			got, err = arcTangentDegrees(f, x)
			expectRounded(t, "ATAND", f, x, got, err, referenceToDegrees(reference))
		}
	}
}

// TestArcSinCosAgainstReference: the whole domain, and each end of it, where
// the cancellation is that complementRoot is written for.
func TestArcSinCosAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 9))

	for _, f := range allFormats {
		low, _ := formatExponents(f)
		p := int(f.Precision())

		for i := range 200 {
			var x vaxfloat.Value

			switch i % 4 {
			case 0:
				x = randomValue(rng, f, -rng.IntN(p+8), true)
			case 1:
				x = randomValue(rng, f, -rng.IntN(4), true)
			case 2:
				// Just inside an end.
				d := randomValue(rng, f, -rng.IntN(p), false)
				x, _ = vaxfloat.Sub(f, vaxfloat.FromInt(1), d)
				if x.Sign() < 0 {
					x = x.Neg()
				}

				if rng.UintN(2) == 0 {
					x = x.Neg()
				}
			default:
				x = randomValue(rng, f, low+rng.IntN(p), true)
			}

			if x.Abs().Cmp(vaxfloat.FromInt(1)) > 0 {
				continue
			}

			asin, acos := referenceAsin(x.Big()), referenceAcos(x.Big())

			got, err := arcSine(f, x)
			expectRounded(t, "ASIN", f, x, got, err, asin)

			got, err = arcCosine(f, x)
			expectRounded(t, "ACOS", f, x, got, err, acos)

			got, err = arcSineDegrees(f, x)
			expectRounded(t, "ASIND", f, x, got, err, referenceToDegrees(asin))

			got, err = arcCosineDegrees(f, x)
			expectRounded(t, "ACOSD", f, x, got, err, referenceToDegrees(acos))
		}
	}
}

// TestKnownAngles: published digits, in every format.
func TestKnownAngles(t *testing.T) {
	for _, f := range allFormats {
		for _, c := range []struct {
			name   string
			fn     function
			arg    string
			digits string
		}{
			{"SIN", sine, "1", digitsSin1},
			{"COS", cosine, "1", digitsCos1},
			{"TAN", tangent, "1", digitsTan1},
			{"SIND", sineDegrees, "1", digitsSinDeg},
			{"COSD", cosineDegrees, "1", digitsCosDeg},
			{"ATAN", arcTangent, "2", digitsAtan2},
		} {
			got, err := c.fn(f, value(t, f, c.arg))
			if want := value(t, f, c.digits); err != nil || got.Cmp(want) != 0 {
				t.Errorf("%s %v(%s) = %s, %v; want %s", c.name, f, c.arg, got, err, want)
			}
		}
	}
}

// TestHardestArgument: the double-precision number nearest to a multiple of
// pi/2 (Kahan, Ng), whose cosine is published. It has 53 bits, so it's a
// G_floating value, and reducing it cancels about 61 bits, past the first try's
// slack.
func TestHardestArgument(t *testing.T) {
	x := vaxfloat.FromBig(new(big.Float).SetMantExp(new(big.Float).SetInt64(6381956970095103), 797))
	x, err := vaxfloat.Round(vaxfloat.G, x)
	if err != nil {
		t.Fatal(err)
	}

	got, err := cosine(vaxfloat.G, x)
	if want := value(t, vaxfloat.G, "-4.68716592425462761112E-19"); err != nil || got.Cmp(want) != 0 {
		t.Errorf("COS = %s, %v; want %s", got, err, want)
	}

	r, _ := reduceRadians(x.Big(), workingPrecision(vaxfloat.G))
	if e := r.MantExp(nil); e >= -reductionSlack {
		t.Errorf("reduced to 2^%d, which didn't need a second try", e)
	}
}

// TestNearestToHalfPi: each format's own nearest value to pi/2, whose cosine is
// all cancellation: a reduction loses every bit of the argument and goes on.
func TestNearestToHalfPi(t *testing.T) {
	for _, f := range allFormats {
		x, err := rounded(f, halfReferencePi())
		if err != nil {
			t.Fatal(err)
		}

		got, err := cosine(f, x)
		_, reference := referenceSinCos(x.Big())
		expectRounded(t, "COS", f, x, got, err, reference)

		got, err = tangent(f, x)
		if err != nil || got.Abs().Cmp(vaxfloat.FromInt(1000000)) < 0 {
			t.Errorf("TAN %v(%s) = %s, %v", f, x, got, err)
		}
	}
}

// TestLargestAngle: the largest angle in each format, the far end of reduction,
// with pi grown to whatever it takes.
func TestLargestAngle(t *testing.T) {
	for _, f := range allFormats {
		_, high := formatExponents(f)
		x := new(big.Float).SetMantExp(big.NewFloat(1), high)
		x.Sub(x, new(big.Float).SetMantExp(big.NewFloat(1), high-int(f.Precision())))
		v := vaxfloat.FromBig(x)

		refSine, refCosine := referenceSinCos(v.Big())

		got, err := sine(f, v)
		expectRounded(t, "SIN", f, v, got, err, refSine)

		got, err = cosine(f, v)
		expectRounded(t, "COS", f, v, got, err, refCosine)
	}
}

// TestExactAngles (the phase doc's Decision 4): angles with exact answers get
// them, to the last bit, from the reduction's exactness or from rounding.
func TestExactAngles(t *testing.T) {
	for _, f := range allFormats {
		for _, c := range []struct {
			name      string
			fn        function
			arg, want float64
		}{
			{"SIND", sineDegrees, 0, 0},
			{"SIND", sineDegrees, 30, 0.5},
			{"SIND", sineDegrees, 90, 1},
			{"SIND", sineDegrees, 150, 0.5},
			{"SIND", sineDegrees, 180, 0},
			{"SIND", sineDegrees, 210, -0.5},
			{"SIND", sineDegrees, 270, -1},
			{"SIND", sineDegrees, 360, 0},
			{"SIND", sineDegrees, -30, -0.5},
			{"SIND", sineDegrees, -90, -1},
			{"SIND", sineDegrees, -180, 0},
			{"SIND", sineDegrees, 360000000, 0},
			{"SIND", sineDegrees, 90 + 360000000, 1},

			{"COSD", cosineDegrees, 0, 1},
			{"COSD", cosineDegrees, 60, 0.5},
			{"COSD", cosineDegrees, 90, 0},
			{"COSD", cosineDegrees, 120, -0.5},
			{"COSD", cosineDegrees, 180, -1},
			{"COSD", cosineDegrees, 270, 0},
			{"COSD", cosineDegrees, 360, 1},
			{"COSD", cosineDegrees, -60, 0.5},
			{"COSD", cosineDegrees, -180, -1},
			{"COSD", cosineDegrees, 360000000, 1},

			{"TAND", tangentDegrees, 0, 0},
			{"TAND", tangentDegrees, 45, 1},
			{"TAND", tangentDegrees, 135, -1},
			{"TAND", tangentDegrees, 180, 0},
			{"TAND", tangentDegrees, 225, 1},
			{"TAND", tangentDegrees, 315, -1},
			{"TAND", tangentDegrees, -45, -1},

			{"SIN", sine, 0, 0},
			{"COS", cosine, 0, 1},
			{"TAN", tangent, 0, 0},

			{"ASIN", arcSine, 0, 0},
			{"ACOS", arcCosine, 1, 0},
			{"ATAN", arcTangent, 0, 0},

			{"ASIND", arcSineDegrees, 0, 0},
			{"ASIND", arcSineDegrees, 1, 90},
			{"ASIND", arcSineDegrees, -1, -90},
			{"ASIND", arcSineDegrees, 0.5, 30},
			{"ASIND", arcSineDegrees, -0.5, -30},
			{"ACOSD", arcCosineDegrees, 1, 0},
			{"ACOSD", arcCosineDegrees, 0.5, 60},
			{"ACOSD", arcCosineDegrees, 0, 90},
			{"ACOSD", arcCosineDegrees, -0.5, 120},
			{"ACOSD", arcCosineDegrees, -1, 180},
			{"ATAND", arcTangentDegrees, 0, 0},
			{"ATAND", arcTangentDegrees, 1, 45},
			{"ATAND", arcTangentDegrees, -1, -45},
		} {
			got, err := c.fn(f, vaxfloat.FromFloat64(c.arg))
			if err != nil || got.Cmp(vaxfloat.FromFloat64(c.want)) != 0 {
				t.Errorf("%s %v(%v) = %s, %v; want %v", c.name, f, c.arg, got, err, c.want)
			}
		}

		// A multiple of 90 as large as the format holds.
		_, high := formatExponents(f)
		huge := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(45), high-6))

		// 45 * 2^k is a multiple of 90 for k >= 1, an even multiple for k >= 2.
		if got, err := sineDegrees(f, huge); err != nil || !got.IsZero() {
			t.Errorf("SIND %v(45 * 2^%d) = %s, %v", f, high-6, got, err)
		}

		if got, err := cosineDegrees(f, huge); err != nil || got.Cmp(vaxfloat.FromInt(1)) != 0 {
			t.Errorf("COSD %v(45 * 2^%d) = %s, %v", f, high-6, got, err)
		}
	}
}

// smallest is format f's smallest value: 2^-128, 2^-1024, or 2^-16384.
func smallest(f vaxfloat.Format) vaxfloat.Value {
	low, _ := formatExponents(f)

	return vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), low-1))
}

// TestTinyAngles: what's the angle itself at the small end, where the argument
// is the format's smallest value, and what's about pi/2.
func TestTinyAngles(t *testing.T) {
	for _, f := range allFormats {
		x := smallest(f)

		if _, err := vaxfloat.Pack(f, x); err != nil {
			t.Fatalf("%v's smallest value doesn't pack: %v", f, err)
		}

		for _, c := range []struct {
			name string
			fn   function
		}{{"SIN", sine}, {"TAN", tangent}, {"ASIN", arcSine}, {"ATAN", arcTangent}} {
			got, err := c.fn(f, x)
			if err != nil || got.Cmp(x) != 0 {
				t.Errorf("%s %v(%s) = %s, %v", c.name, f, x, got, err)
			}

			got, err = c.fn(f, x.Neg())
			if err != nil || got.Cmp(x.Neg()) != 0 {
				t.Errorf("%s %v(-%s) = %s, %v", c.name, f, x, got, err)
			}
		}

		half, _ := rounded(f, halfReferencePi())
		ninety := vaxfloat.FromInt(90)

		for _, c := range []struct {
			name string
			fn   function
			arg  vaxfloat.Value
			want vaxfloat.Value
		}{
			{"COS", cosine, x, vaxfloat.FromInt(1)},
			{"COSD", cosineDegrees, x, vaxfloat.FromInt(1)},
			{"ACOS", arcCosine, x, half},
			{"ACOSD", arcCosineDegrees, x, ninety},
			{"ASIND", arcSineDegrees, x, referenceRounded(t, f, referenceToDegrees(x.Big()))},
			{"ATAND", arcTangentDegrees, x, referenceRounded(t, f, referenceToDegrees(x.Big()))},
		} {
			got, err := c.fn(f, c.arg)
			if err != nil || got.Cmp(c.want) != 0 {
				t.Errorf("%s %v(%s) = %s, %v; want %s", c.name, f, c.arg, got, err, c.want)
			}
		}
	}
}

func referenceRounded(t *testing.T, f vaxfloat.Format, x *big.Float) vaxfloat.Value {
	t.Helper()

	v, err := rounded(f, x)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

// TestLargeArcTangents: the far end, where an arc tangent is pi/2 or 90.
func TestLargeArcTangents(t *testing.T) {
	for _, f := range allFormats {
		_, high := formatExponents(f)
		half, _ := rounded(f, halfReferencePi())
		x := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), high-1))

		for _, c := range []struct {
			name string
			fn   function
			want vaxfloat.Value
		}{{"ATAN", arcTangent, half}, {"ATAND", arcTangentDegrees, vaxfloat.FromInt(90)}} {
			if got, err := c.fn(f, x); err != nil || got.Cmp(c.want) != 0 {
				t.Errorf("%s %v(2^%d) = %s, %v", c.name, f, high-1, got, err)
			}

			if got, err := c.fn(f, x.Neg()); err != nil || got.Cmp(c.want.Neg()) != 0 {
				t.Errorf("%s %v(-2^%d) = %s, %v", c.name, f, high-1, got, err)
			}
		}
	}
}

// TestDegreeLimits: the manual's MTH$_FLOUNDMAT, an angle below 180/pi * 2^-m,
// m being 128, 1024, or 16384. 180/pi is about 57.3, so 58 * 2^-m has a sine in
// range and 57 * 2^-m doesn't. A sine and a tangent are alike there, and a
// cosine's 1. And MTH$_FLOOVEMAT for a tangent at an odd multiple of 90.
func TestDegreeLimits(t *testing.T) {
	for _, f := range allFormats {
		low, _ := formatExponents(f)

		fits := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(58), low-1))
		under := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(57), low-1))

		for _, c := range []struct {
			name string
			fn   function
		}{{"SIND", sineDegrees}, {"TAND", tangentDegrees}} {
			if got, err := c.fn(f, fits); err != nil || got.IsZero() {
				t.Errorf("%s %v(%s) = %s, %v", c.name, f, fits, got, err)
			}

			if _, err := c.fn(f, under); !errors.Is(err, vaxfloat.ErrUnderflow) {
				t.Errorf("%s %v(%s): %v", c.name, f, under, err)
			}

			if _, err := c.fn(f, under.Neg()); !errors.Is(err, vaxfloat.ErrUnderflow) {
				t.Errorf("%s %v(-%s): %v", c.name, f, under, err)
			}
		}

		// The pair reports it, with both results good.
		first, second, err := sineCosineDegrees(f, under)
		if !errors.Is(err, vaxfloat.ErrUnderflow) || !first.IsZero() || second.Cmp(vaxfloat.FromInt(1)) != 0 {
			t.Errorf("SINCOSD %v(%s) = %s, %s, %v", f, under, first, second, err)
		}

		if got, err := cosineDegrees(f, under); err != nil || got.Cmp(vaxfloat.FromInt(1)) != 0 {
			t.Errorf("COSD %v(%s) = %s, %v", f, under, got, err)
		}

		for _, angle := range []int64{90, -90, 270, -270, 450, 90 + 360*1000000} {
			if got, err := tangentDegrees(f, vaxfloat.FromInt(angle)); !errors.Is(err, vaxfloat.ErrOverflow) {
				t.Errorf("TAND %v(%d) = %s, %v", f, angle, got, err)
			}
		}
	}
}

// TestArcDomains: a sine or cosine just beyond 1 has no angle, and the
// tangent's domain is everything.
func TestArcDomains(t *testing.T) {
	for _, f := range allFormats {
		ulp := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), 1-int(f.Precision())))
		beyond, err := vaxfloat.Add(f, vaxfloat.FromInt(1), ulp)
		if err != nil {
			t.Fatal(err)
		}

		for _, c := range []struct {
			name string
			fn   function
		}{{"ASIN", arcSine}, {"ACOS", arcCosine}, {"ASIND", arcSineDegrees}, {"ACOSD", arcCosineDegrees}} {
			if _, err := c.fn(f, beyond); !errors.Is(err, errInvalidArg) {
				t.Errorf("%s %v(%s): %v", c.name, f, beyond, err)
			}

			if _, err := c.fn(f, beyond.Neg()); !errors.Is(err, errInvalidArg) {
				t.Errorf("%s %v(-%s): %v", c.name, f, beyond, err)
			}

			if _, err := c.fn(f, vaxfloat.FromInt(1)); err != nil {
				t.Errorf("%s %v(1): %v", c.name, f, err)
			}

			if _, err := c.fn(f, vaxfloat.FromInt(-1)); err != nil {
				t.Errorf("%s %v(-1): %v", c.name, f, err)
			}
		}
	}
}

// BenchmarkAngles measures a call in each format, against the survey's
// estimate of tens of VAX instructions' time.
func BenchmarkAngles(b *testing.B) {
	for _, f := range allFormats {
		x, _ := vaxfloat.Round(f, vaxfloat.FromFloat64(1.2345678))
		half, _ := vaxfloat.Round(f, vaxfloat.FromFloat64(0.5))

		for _, c := range []struct {
			name string
			fn   function
			arg  vaxfloat.Value
		}{
			{"SIN", sine, x}, {"COS", cosine, x}, {"TAN", tangent, x},
			{"SIND", sineDegrees, x}, {"ATAN", arcTangent, x},
			{"ASIN", arcSine, half}, {"ACOS", arcCosine, half}, {"ATAND", arcTangentDegrees, x},
		} {
			b.Run(fmt.Sprintf("%s/%v", c.name, f), func(b *testing.B) {
				for b.Loop() {
					if _, err := c.fn(f, c.arg); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkPi is what the largest H_floating angles cost, once.
func BenchmarkPi(b *testing.B) {
	for _, bits := range []uint{256, 1300, 16700} {
		b.Run(fmt.Sprint(bits), func(b *testing.B) {
			for b.Loop() {
				computePi(bits)
			}
		})
	}
}
