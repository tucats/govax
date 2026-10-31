package mthrtl

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/tucats/govax/internal/vaxfloat"
)

// complexFormats are the formats with complex numbers.
var complexFormats = []vaxfloat.Format{vaxfloat.F, vaxfloat.D, vaxfloat.G}

// complexBits is the length of this file's references.
const complexBits = 512

// referenceModulus is the manual's MAX * SQRT((MIN/MAX)^2 + 1), which the code
// doesn't use.
func referenceModulus(z complexValue) *big.Float {
	large, small := z.re.Abs(), z.im.Abs()
	if large.Cmp(small) < 0 {
		large, small = small, large
	}

	if large.IsZero() {
		return newFloat(complexBits)
	}

	ratio := newFloat(complexBits).Quo(small.Big(), large.Big())
	ratio.Mul(ratio, ratio)
	ratio.Add(ratio, big.NewFloat(1))

	root := newFloat(complexBits).Sqrt(ratio)

	return root.Mul(root, large.Big())
}

// referenceLogModulus is ln |z|, by referenceLog's iteration when |z|^2 is well away
// from 1, and otherwise by the series for ln(1 + d), with d the difference from 1 in
// rationals. The code uses neither.
func referenceLogModulus(z complexValue) *big.Float {
	re, im := z.re.Rat(), z.im.Rat()

	square := new(big.Rat).Add(new(big.Rat).Mul(re, re), new(big.Rat).Mul(im, im))
	difference := new(big.Rat).Sub(square, big.NewRat(1, 1))

	d := newFloat(complexBits).SetRat(difference)
	if d.Sign() != 0 && d.MantExp(nil) >= -8 {
		result := referenceLog(newFloat(complexBits).SetRat(square))

		return result.SetMantExp(result, -1)
	}

	sum := newFloat(complexBits)
	power := newFloat(complexBits).SetInt64(1)

	for k := int64(1); k < 80; k++ {
		power.Mul(power, d)

		term := newFloat(complexBits).Quo(power, newFloat(complexBits).SetInt64(k))
		if k%2 == 0 {
			term.Neg(term)
		}

		sum.Add(sum, term)
	}

	return sum.SetMantExp(sum, -1)
}

// referenceSquareRoot is the root as the two square roots
//
//	re = SQRT((|z| + r) / 2)     im = sign(i) * SQRT((|z| - r) / 2)
//
// which the page's Q replaces because they cancel in one part or the other: so it
// has the length to spare for that here, in the size of the parts' difference.
func referenceSquareRoot(z complexValue) (re, im *big.Float) {
	w := uint(complexBits)

	if !z.re.IsZero() && !z.im.IsZero() {
		gap := z.re.Big().MantExp(nil) - z.im.Big().MantExp(nil)
		w += uint(2*max(gap, -gap)) + 64
	}

	square := vaxfloat.AddExact(vaxfloat.MulExact(z.re, z.re), vaxfloat.MulExact(z.im, z.im))
	size := newFloat(w).Sqrt(square.Big())

	half := func(x *big.Float) *big.Float {
		x.SetMantExp(x, -1)

		return newFloat(w).Sqrt(x)
	}

	re = half(newFloat(w).Add(size, z.re.Big()))
	im = half(newFloat(w).Sub(size, z.re.Big()))

	if z.im.Sign() < 0 {
		im.Neg(im)
	}

	return re, im
}

// complexArgument is one of the kinds of number that matter: parts of about the same
// size, parts far apart, a part zero, the size near 1, or tiny imaginary parts.
func complexArgument(rng *rand.Rand, f vaxfloat.Format, i int) complexValue {
	low, high := formatExponents(f)
	exponent := func(from, to int) int { return from + rng.IntN(to-from+1) }

	switch i % 6 {
	case 0: // about the same size, anywhere
		e := exponent(low+2, high-2)

		return complexValue{randomValue(rng, f, e, true), randomValue(rng, f, e+rng.IntN(3)-1, true)}
	case 1: // sizes anywhere, apart as they fall
		return complexValue{randomValue(rng, f, exponent(low, high-1), true), randomValue(rng, f, exponent(low, high-1), true)}
	case 2: // moderate
		return complexValue{randomValue(rng, f, exponent(-6, 6), true), randomValue(rng, f, exponent(-6, 6), true)}
	case 3:
		return complexValue{randomValue(rng, f, exponent(low, high-1), true), vaxfloat.Value{}}
	case 4:
		return complexValue{vaxfloat.Value{}, randomValue(rng, f, exponent(low, high-1), true)}
	}

	// One part just under 1 in size and the other small: |z| is near 1, how near
	// depending on the small part.
	large := randomValue(rng, f, 0, true)
	if rng.UintN(2) == 0 {
		large = vaxfloat.FromInt(1)
	}

	return complexValue{large, randomValue(rng, f, exponent(low, -1), true)}
}

// expectParts compares a complex result with its reference's parts, each rounded to f
// once, a part too small being 0.0.
func expectParts(t *testing.T, name string, f vaxfloat.Format, z, got complexValue, err error, re, im *big.Float) {
	t.Helper()

	if err != nil {
		t.Errorf("%s %v(%s, %s): %v", name, f, z.re, z.im, err)

		return
	}

	wantRe, rerr := noUnderflow(rounded(f, re))
	wantIm, ierr := noUnderflow(rounded(f, im))

	if rerr != nil || ierr != nil {
		t.Errorf("%s %v(%s, %s): reference: %v, %v", name, f, z.re, z.im, rerr, ierr)
	} else if got.re.Cmp(wantRe) != 0 || got.im.Cmp(wantIm) != 0 {
		t.Errorf("%s %v(%s, %s) = (%s, %s), want (%s, %s)", name, f, z.re, z.im, got.re, got.im, wantRe, wantIm)
	}
}

func TestComplexAbsoluteAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 7))

	for _, f := range complexFormats {
		for i := range 3000 {
			z := complexArgument(rng, f, i)
			got, err := complexAbsolute(f, z)

			want, werr := rounded(f, referenceModulus(z))
			if !errors.Is(err, werr) {
				t.Errorf("%v(%s, %s): error %v, want %v", f, z.re, z.im, err, werr)
			} else if got.Cmp(want) != 0 {
				t.Errorf("%v(%s, %s) = %s, want %s", f, z.re, z.im, got, want)
			}
		}
	}
}

func TestComplexLogAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 8))

	for _, f := range complexFormats {
		for i := range 600 {
			z := complexArgument(rng, f, i)

			// Past the range the D and G forms signal, which TestComplexLimits has.
			if f != vaxfloat.F && overflows(f, referenceModulus(z)) {
				continue
			}

			got, err := complexLog(f, z)
			expectParts(t, "CLOG", f, z, got, err, referenceLogModulus(z), referenceAtan2(z.im.Big(), z.re.Big()))
		}
	}
}

func TestComplexSquareRootAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 9))

	for _, f := range complexFormats {
		for i := range 3000 {
			z := complexArgument(rng, f, i)

			if overflows(f, newFloat(complexBits).Add(z.re.Abs().Big(), referenceModulus(z))) {
				continue
			}

			got, err := complexSquareRoot(f, z)
			re, im := referenceSquareRoot(z)
			expectParts(t, "CSQRT", f, z, got, err, re, im)
		}
	}
}

// moderateComplex is an argument of an exponential or a trigonometric function: the
// part that grows within its limit, and the other any size.
func moderateComplex(rng *rand.Rand, f vaxfloat.Format, i int) (growing, other vaxfloat.Value) {
	low, high := formatExponents(f)

	limit := 6 // |x| below 64, inside every format's limit
	if f == vaxfloat.G {
		limit = 9
	}

	switch i % 4 {
	case 0:
		return randomValue(rng, f, 1+rng.IntN(limit), true), randomValue(rng, f, 1+rng.IntN(8), true)
	case 1:
		return randomValue(rng, f, -rng.IntN(40), true), randomValue(rng, f, -rng.IntN(40), true)
	case 2:
		return randomValue(rng, f, low+rng.IntN(20), true), randomValue(rng, f, low+rng.IntN(20), true)
	}

	// A large other part's reduction, and zero for the growing one.
	if rng.UintN(2) == 0 {
		return vaxfloat.Value{}, randomValue(rng, f, 20+rng.IntN(min(high-20, 40)), true)
	}

	return randomValue(rng, f, 1+rng.IntN(limit), true), vaxfloat.Value{}
}

func TestComplexExponentialAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 10))

	for _, f := range complexFormats {
		for i := range 400 {
			r, im := moderateComplex(rng, f, i)
			z := complexValue{r, im}

			got, err := complexExponential(f, z)

			e := referenceExp(r.Big())
			s, c := referenceSinCos(im.Big())

			expectParts(t, "CEXP", f, z, got, err,
				newFloat(complexBits).Mul(e, c), newFloat(complexBits).Mul(e, s))
		}
	}
}

func TestComplexSineCosineAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 11))

	for _, f := range complexFormats {
		for i := range 400 {
			im, r := moderateComplex(rng, f, i)
			z := complexValue{r, im}

			s, c := referenceSinCos(r.Big())
			sh, ch := referenceSinh(im.Big()), referenceCosh(im.Big())

			got, err := complexSine(f, z)
			expectParts(t, "CSIN", f, z, got, err,
				newFloat(complexBits).Mul(s, ch), newFloat(complexBits).Mul(c, sh))

			imaginary := newFloat(complexBits).Mul(s, sh)

			got, err = complexCosine(f, z)
			expectParts(t, "CCOS", f, z, got, err,
				newFloat(complexBits).Mul(c, ch), imaginary.Neg(imaginary))
		}
	}
}

// TestComplexExact: values with exact answers, the same in every format.
func TestComplexExact(t *testing.T) {
	number := func(r, i int64) complexValue {
		return complexValue{vaxfloat.FromInt(r), vaxfloat.FromInt(i)}
	}

	for _, f := range complexFormats {
		for _, c := range []struct {
			r, i, want int64
		}{{3, 4, 5}, {-3, 4, 5}, {3, -4, 5}, {-3, -4, 5}, {0, 0, 0}, {0, -2, 2}, {7, 0, 7}, {5, 12, 13}} {
			got, err := complexAbsolute(f, number(c.r, c.i))
			if err != nil || got.Cmp(vaxfloat.FromInt(c.want)) != 0 {
				t.Errorf("%v CABS(%d, %d) = %s, %v", f, c.r, c.i, got, err)
			}
		}

		for _, c := range []struct {
			name string
			fn   complexFunction
			z    complexValue
			want complexValue
		}{
			{"CSQRT", complexSquareRoot, number(0, 0), number(0, 0)},
			{"CSQRT", complexSquareRoot, number(3, 4), number(2, 1)},
			{"CSQRT", complexSquareRoot, number(3, -4), number(2, -1)},
			{"CSQRT", complexSquareRoot, number(-3, 4), number(1, 2)},
			{"CSQRT", complexSquareRoot, number(-3, -4), number(1, -2)},
			{"CSQRT", complexSquareRoot, number(0, 2), number(1, 1)},
			{"CSQRT", complexSquareRoot, number(0, -2), number(1, -1)},
			{"CSQRT", complexSquareRoot, number(4, 0), number(2, 0)},
			{"CSQRT", complexSquareRoot, number(-4, 0), number(0, 2)},
			{"CEXP", complexExponential, number(0, 0), number(1, 0)},
			{"CSIN", complexSine, number(0, 0), number(0, 0)},
			{"CCOS", complexCosine, number(0, 0), number(1, 0)},
			{"CLOG", complexLog, number(1, 0), number(0, 0)},
			{"CONJG", conjugate, number(3, 4), number(3, -4)},
			{"CONJG", conjugate, number(3, 0), number(3, 0)},
			{"CMPLX", asComplex, number(3, -4), number(3, -4)},
		} {
			got, err := c.fn(f, c.z)
			if err != nil || got.re.Cmp(c.want.re) != 0 || got.im.Cmp(c.want.im) != 0 {
				t.Errorf("%s %v(%s, %s) = (%s, %s), %v", c.name, f, c.z.re, c.z.im, got.re, got.im, err)
			}
		}

		// The logarithm's angle at the axes, each a multiple of pi/2.
		for _, c := range []struct {
			z        complexValue
			quarters int64
		}{{number(2, 0), 0}, {number(0, 2), 1}, {number(-2, 0), 2}, {number(0, -2), -1}} {
			got, err := complexLog(f, c.z)

			half := halfReferencePi()
			want := referenceRounded(t, f, half.Mul(half, big.NewFloat(float64(c.quarters))))

			if err != nil || got.im.Cmp(want) != 0 {
				t.Errorf("CLOG %v(%s, %s) = (%s, %s), %v", f, c.z.re, c.z.im, got.re, got.im, err)
			}
		}
	}
}

// TestComplexPublished: the manual's own examples, to the digits it printed.
func TestComplexPublished(t *testing.T) {
	const (
		r1 = "0.8535407185554504"
		i1 = "0.2043401598930359"
		r2 = "3.1425637846746565"
		i2 = "7.43678469887"
	)

	for _, c := range []struct {
		name         string
		fn           complexFunction
		f            vaxfloat.Format
		r, i, wr, wi string
		bits         int
	}{
		{"CLOG", complexLog, vaxfloat.F, "2.0", "1.0", "0.8047190", "0.4636476", 20},
		{"CCOS", complexCosine, vaxfloat.D, r1, i1, "0.6710899028500762", "-0.1550672019621661", 45},
		{"CEXP", complexExponential, vaxfloat.D, r1, i1, "2.29909677719458", "0.476447678044977", 42},
		{"CDLOG", complexLog, vaxfloat.D, r2, i2, "2.088587642177504", "1.170985519274141", 45},
		{"CSIN", complexSine, vaxfloat.D, r1, i1, "0.769400835484975", "0.135253340912255", 42},
		{"CSQRT", complexSquareRoot, vaxfloat.D, r1, i1, "0.9303763973040062", "0.1098158554350485", 45},
	} {
		got, err := c.fn(c.f, complexValue{value(t, c.f, c.r), value(t, c.f, c.i)})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)

			continue
		}

		if !closeTo(got.re.Big(), decimal(t, c.wr), c.bits) || !closeTo(got.im.Big(), decimal(t, c.wi), c.bits) {
			t.Errorf("%s(%s, %s) = (%s, %s), want (%s, %s)", c.name, c.r, c.i, got.re, got.im, c.wr, c.wi)
		}
	}

	abs, err := complexAbsolute(vaxfloat.D, complexValue{value(t, vaxfloat.D, "12.34567890123"), value(t, vaxfloat.D, "45.536376385345")})
	if err != nil || !closeTo(abs.Big(), decimal(t, "47.1802645376230"), 42) {
		t.Errorf("CDABS = %s, %v", abs, err)
	}
}

// TestComplexNearUnitCircle: where the logarithm is all in how far |z|^2 is from 1,
// which is less than a format's last bit, or a modulus rounded first would be 1.
func TestComplexNearUnitCircle(t *testing.T) {
	power := func(e int) vaxfloat.Value {
		x := big.NewFloat(1)

		return vaxfloat.FromBig(x.SetMantExp(x, e))
	}

	one := vaxfloat.FromInt(1)

	for _, c := range []struct {
		f     vaxfloat.Format
		small int // the imaginary part is 2^small
		want  int // the real part of the logarithm is about 2^want
	}{
		{vaxfloat.F, -20, -41},
		{vaxfloat.D, -60, -121},
		{vaxfloat.G, -500, -1001},
		{vaxfloat.G, -60, -121},
	} {
		z := complexValue{one, power(c.small)}

		got, err := complexLog(c.f, z)
		if err != nil {
			t.Fatal(err)
		}

		if want := power(c.want); !closeTo(got.re.Big(), want.Big(), 30) {
			t.Errorf("%v (1, 2^%d): %s, want about %s", c.f, c.small, got.re, want)
		}
	}

	// Past the format's range, the part is 0.0 and the angle's still there.
	for _, c := range []struct {
		f     vaxfloat.Format
		small int
	}{{vaxfloat.F, -70}, {vaxfloat.D, -70}, {vaxfloat.G, -600}} {
		got, err := complexLog(c.f, complexValue{one, power(c.small)})
		if err != nil || !got.re.IsZero() || got.im.Cmp(power(c.small)) != 0 {
			t.Errorf("%v (1, 2^%d) = (%s, %s), %v", c.f, c.small, got.re, got.im, err)
		}
	}
}

// TestComplexLimits: the pages' figures, each side of them.
func TestComplexLimits(t *testing.T) {
	for _, c := range []struct {
		f           vaxfloat.Format
		below       string
		above       string
		coshInRange string // past the limit, and still within COSH's own
	}{
		{vaxfloat.F, "88.02", "88.03", "88.5"},
		{vaxfloat.D, "88.02", "88.03", "88.5"},
		{vaxfloat.G, "709.08", "709.09", "709.5"},
	} {
		below, above, between := value(t, c.f, c.below), value(t, c.f, c.above), value(t, c.f, c.coshInRange)
		one := vaxfloat.FromInt(1)

		for _, fn := range []struct {
			name string
			fn   complexFunction
			at   func(x vaxfloat.Value) complexValue
		}{
			{"CEXP", complexExponential, func(x vaxfloat.Value) complexValue { return complexValue{x, one} }},
			{"CSIN", complexSine, func(x vaxfloat.Value) complexValue { return complexValue{one, x} }},
			{"CSIN-", complexSine, func(x vaxfloat.Value) complexValue { return complexValue{one, x.Neg()} }},
			{"CCOS", complexCosine, func(x vaxfloat.Value) complexValue { return complexValue{one, x} }},
			{"CCOS-", complexCosine, func(x vaxfloat.Value) complexValue { return complexValue{one, x.Neg()} }},
		} {
			name := fmt.Sprintf("%s %v", fn.name, c.f)

			if _, err := fn.fn(c.f, fn.at(below)); err != nil {
				t.Errorf("%s(%s): %v", name, c.below, err)
			}

			if _, err := fn.fn(c.f, fn.at(above)); !errors.Is(err, vaxfloat.ErrOverflow) {
				t.Errorf("%s(%s): %v", name, c.above, err)
			}

			if _, err := fn.fn(c.f, fn.at(between)); !errors.Is(err, vaxfloat.ErrOverflow) {
				t.Errorf("%s(%s): %v", name, c.coshInRange, err)
			}
		}

		// The real function itself goes further, by its own page.
		if _, err := hyperbolicCosine(c.f, between); err != nil {
			t.Errorf("COSH %v(%s): %v", c.f, c.coshInRange, err)
		}

		// e^r for a large negative r is too small to matter, not an overflow.
		for _, r := range []string{"-" + c.above, "-" + c.coshInRange, "-30000"} {
			got, err := complexExponential(c.f, complexValue{value(t, c.f, r), one})
			if r == "-30000" {
				if !errors.Is(err, vaxfloat.ErrUnderflow) || !got.re.IsZero() || !got.im.IsZero() {
					t.Errorf("CEXP %v(%s, 1) = (%s, %s), %v, want an underflow", c.f, r, got.re, got.im, err)
				}
			} else if err != nil {
				t.Errorf("CEXP %v(%s, 1): %v", c.f, r, err)
			}
		}
	}

	large := map[vaxfloat.Format][4]string{
		// fits, modulus past the range; sum fits; sum past the range and modulus not.
		vaxfloat.F: {"1.0E38", "1.5E38", "5.0E37", "1.0E38"},
		vaxfloat.D: {"1.0E38", "1.5E38", "5.0E37", "1.0E38"},
		vaxfloat.G: {"6.0E307", "8.0E307", "3.0E307", "6.0E307"},
	}

	for f, s := range large {
		fits := value(t, f, s[0])
		huge := value(t, f, s[1])
		half := value(t, f, s[2])
		single := value(t, f, s[3])

		if _, err := complexAbsolute(f, complexValue{fits, fits}); err != nil {
			t.Errorf("CABS %v(%s, %s): %v", f, s[0], s[0], err)
		}

		pastModulus := complexValue{huge, huge}

		if _, err := complexAbsolute(f, pastModulus); !errors.Is(err, vaxfloat.ErrOverflow) {
			t.Errorf("CABS %v(%s, %s): %v", f, s[1], s[1], err)
		}

		// The logarithm of that is MTH$CxABS's overflow, on VMS 7.1, in every form.
		var trap trapError
		if _, err := complexLog(f, pastModulus); !errors.Is(err, vaxfloat.ErrOverflow) || errors.As(err, &trap) {
			t.Errorf("CLOG %v(%s, %s): %v", f, s[1], s[1], err)
		}

		if _, err := complexSquareRoot(f, complexValue{half, half}); err != nil {
			t.Errorf("CSQRT %v(%s, %s): %v", f, s[2], s[2], err)
		}

		// The page's overflow has no argument, on VMS 7.1.
		for _, z := range []complexValue{pastModulus, {single, vaxfloat.Value{}}} {
			if _, err := complexSquareRoot(f, z); err != nil {
				t.Errorf("CSQRT %v(%s, %s): %v", f, z.re, z.im, err)
			}
		}
	}
}

// TestComplexLogOfZero: MTH$_INVARGMAT in F, and the logarithm's MTH$_LOGZERNEG in D
// and G, as on VMS 7.1.
func TestComplexLogOfZero(t *testing.T) {
	for _, f := range complexFormats {
		if _, err := complexLog(f, complexValue{}); !errors.Is(err, zeroComplexBase(f)) {
			t.Errorf("%v: %v", f, err)
		}
	}

	if !errors.Is(zeroComplexBase(vaxfloat.F), errInvalidArg) || !errors.Is(zeroComplexBase(vaxfloat.G), errLogZeroNeg) {
		t.Error("zeroComplexBase has the formats the wrong way round")
	}
}

// TestComplexPartsTooSmall: a part out of the format's range at the small end is
// 0.0, with no error, since no page lists MTH$_FLOUNDMAT.
func TestComplexPartsTooSmall(t *testing.T) {
	tiny := map[vaxfloat.Format]string{vaxfloat.F: "1.0E-38", vaxfloat.D: "1.0E-38", vaxfloat.G: "1.0E-307"}
	large := map[vaxfloat.Format]string{vaxfloat.F: "1.0E37", vaxfloat.D: "1.0E37", vaxfloat.G: "1.0E307"}

	for _, f := range complexFormats {
		small := value(t, f, tiny[f])

		// The root's second part is small / (2 * ROOT), with ROOT about sqrt(large).
		got, err := complexSquareRoot(f, complexValue{value(t, f, large[f]), small})
		if err != nil || !got.im.IsZero() || got.re.IsZero() {
			t.Errorf("CSQRT %v: (%s, %s), %v", f, got.re, got.im, err)
		}

		half := vaxfloat.FromBig(halfReferencePi())

		half, herr := vaxfloat.Round(f, half)
		if herr != nil {
			t.Fatal(herr)
		}

		// The cosine of the nearest value to pi/2 times a tiny hyperbolic sine.
		got, err = complexSine(f, complexValue{half, small})
		if err != nil || got.re.IsZero() || !got.im.IsZero() {
			t.Errorf("CSIN %v: (%s, %s), %v", f, got.re, got.im, err)
		}
	}
}

func BenchmarkComplex(b *testing.B) {
	for _, f := range complexFormats {
		z := complexValue{vaxfloat.FromFloat64(1.25), vaxfloat.FromFloat64(0.75)}

		for _, c := range []struct {
			name string
			fn   complexFunction
		}{
			{"CEXP", complexExponential}, {"CLOG", complexLog}, {"CSIN", complexSine},
			{"CCOS", complexCosine}, {"CSQRT", complexSquareRoot},
		} {
			b.Run(fmt.Sprintf("%s/%v", c.name, f), func(b *testing.B) {
				for b.Loop() {
					_, _ = c.fn(f, z)
				}
			})
		}

		b.Run(fmt.Sprintf("CABS/%v", f), func(b *testing.B) {
			for b.Loop() {
				_, _ = complexAbsolute(f, z)
			}
		})
	}
}
