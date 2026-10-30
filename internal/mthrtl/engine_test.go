package mthrtl

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/tucats/govax/internal/vaxfloat"
)

// The floating formats, for tests that run in all of them.
var allFormats = []vaxfloat.Format{vaxfloat.F, vaxfloat.D, vaxfloat.G, vaxfloat.H}

// Values to about 100 digits, published constants, none of them from this
// package's arithmetic.
const (
	digitsE     = "2.718281828459045235360287471352662497757247093699959574966967627724076630353547594571382178525166427"
	digitsLn2   = "0.693147180559945309417232121458176568075500134360255254120680009493393621969694715605863326996418687"
	digitsLn10  = "2.302585092994045684017991454684364207601101488628772976033327900967572609677352480235997205089598298"
	digitsLog2  = "0.301029995663981195213738894724493026768189881462108541310427461127108189274424509486927252118186172"
	digitsLb10  = "3.321928094887362347870319429489390175864831393024580612054756395815934776608625215850139743359370155"
	digitsSqrt2 = "1.414213562373095048801688724209698078569671875376948073176679737990732478462107038850387534327641573"
)

// decimal is a long decimal number, as a big.Float.
func decimal(t *testing.T, s string) *big.Float {
	t.Helper()

	x, _, err := big.ParseFloat(s, 10, 1024, big.ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}

	return x
}

// closeTo reports whether x and y differ by less than 2^-bits, relative to y.
func closeTo(x, y *big.Float, bits int) bool {
	difference := newFloat(2048).Sub(x, y)

	return difference.Sign() == 0 || difference.MantExp(nil) < y.MantExp(nil)-bits
}

// value is a decimal number rounded to f.
func value(t *testing.T, f vaxfloat.Format, s string) vaxfloat.Value {
	t.Helper()

	x, err := vaxfloat.Parse(s)
	if err != nil {
		t.Fatal(err)
	}

	v, err := vaxfloat.Round(f, x)
	if err != nil {
		t.Fatalf("%s in %v: %v", s, f, err)
	}

	return v
}

// TestConstants checks ln 2 and ln 10 against their published digits, at the
// precision they're first asked for and at a higher one asked for after it.
func TestConstants(t *testing.T) {
	for _, c := range []struct {
		name   string
		value  *constant
		digits string
	}{{"ln 2", &ln2, digitsLn2}, {"ln 10", &ln10, digitsLn10}} {
		if got := c.value.at(64); got.Prec() < constantFloor || !closeTo(got, decimal(t, c.digits), constantFloor-2) {
			t.Errorf("%s: %s", c.name, got.Text('g', 80))
		}

		if got := c.value.at(320); got.Prec() < 320 || !closeTo(got, decimal(t, c.digits), 320) {
			t.Errorf("%s at 320 bits: %s", c.name, got.Text('g', 110))
		}
	}
}

// TestConstantFromGoroutines asks for a constant from several goroutines at
// once, as machines running side by side may: for go test -race.
func TestConstantFromGoroutines(t *testing.T) {
	var (
		wg sync.WaitGroup
		c  = constant{compute: ln2.compute}
	)

	for i := range 8 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if got := c.at(uint(200 + 100*i)); got.Prec() < uint(200+100*i) {
				t.Errorf("precision %d", got.Prec())
			}
		}()
	}

	wg.Wait()
}

// TestWorkingPrecisionUnderConstantFloor: at H's working precision, a constant
// needs computing only once, whichever function asks first. The exponential
// adds the bits of its argument's integer part, 14 at most in range.
func TestWorkingPrecisionUnderConstantFloor(t *testing.T) {
	if most := workingPrecision(vaxfloat.H) + 2*scratchBits + exponentialHalvings + 14; most > constantFloor {
		t.Errorf("the engine can ask for %d bits, above the constants' floor", most)
	}
}

// The reference: the functions by other means, at far more bits than any
// format's result needs to be rounded correctly. The exponential is the Taylor
// series after halving the argument, with no ln 2 in it; the logarithm solves
// e^y = x by iteration (Halley's method), from a start the size of the answer.

const referenceBits = 512

func referenceExp(x *big.Float) *big.Float {
	const halvings = 24

	z := newFloat(referenceBits).Set(x)
	z.SetMantExp(z, -halvings)

	sum := newFloat(referenceBits).SetInt64(1)
	term := newFloat(referenceBits).SetInt64(1)

	for j := int64(1); j < 200; j++ {
		term.Mul(term, z)
		term.Quo(term, newFloat(referenceBits).SetInt64(j))
		sum.Add(sum, term)
	}

	for range halvings {
		sum.Mul(sum, sum)
	}

	return sum
}

func referenceLog(x *big.Float) *big.Float {
	// The start is x's exponent times ln 2, with a float64 for the fraction.
	m := new(big.Float)
	n := x.MantExp(m)
	fraction, _ := m.Float64()
	y := newFloat(referenceBits).SetFloat64(float64(n)*0.693147180559945 + 1.4*(fraction-0.75))

	// y += 2 * (x - e^y) / (x + e^y): the error cubes each time.
	for range 8 {
		e := referenceExp(y)
		difference := newFloat(referenceBits).Sub(x, e)
		sum := newFloat(referenceBits).Add(x, e)
		step := newFloat(referenceBits).Quo(difference, sum)
		y.Add(y, step.Mul(step, big.NewFloat(2)))
	}

	return y
}

// randomValue returns a value of format f with random fraction bits, in
// [2^(exponent-1), 2^exponent), of either sign if signed.
func randomValue(rng *rand.Rand, f vaxfloat.Format, exponent int, signed bool) vaxfloat.Value {
	mantissa := new(big.Int).SetBit(new(big.Int), int(f.Precision())-1, 1)

	for i := 0; i < int(f.Precision())-1; i++ {
		mantissa.SetBit(mantissa, i, rng.UintN(2))
	}

	x := new(big.Float).SetInt(mantissa)
	x.SetMantExp(x, exponent-int(f.Precision()))

	if signed && rng.UintN(2) == 0 {
		x.Neg(x)
	}

	return vaxfloat.FromBig(x)
}

// formatExponents are the range of a format's exponents, as a value's
// MantExp reports them: the values run from 2^(low-1) to just under 2^high.
func formatExponents(f vaxfloat.Format) (low, high int) {
	switch f {
	case vaxfloat.G:
		return -1023, 1023
	case vaxfloat.H:
		return -16383, 16383
	default:
		return -127, 127
	}
}

// expectRounded compares a function's result with the reference's, rounded
// to f once. They must be the same value: the engine's result differs only
// where the true value lies within 2^-40 of an ulp of a rounding boundary,
// which these seeded runs don't reach.
func expectRounded(t *testing.T, name string, f vaxfloat.Format, x, got vaxfloat.Value, err error, reference *big.Float) {
	t.Helper()

	want, werr := vaxfloat.Round(f, vaxfloat.FromBig(reference))

	if !errors.Is(err, werr) {
		t.Errorf("%s %v(%s): error %v, want %v", name, f, x, err, werr)
	} else if got.Cmp(want) != 0 {
		t.Errorf("%s %v(%s) = %s, want %s", name, f, x, got, want)
	}
}

// TestExpAgainstReference: all across each format's range of results, and
// out of it at both ends, and for arguments small enough that e^x is near 1.
func TestExpAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 3))

	for _, f := range allFormats {
		_, high := formatExponents(f)
		limit := float64(high) * 0.7

		for i := range 400 {
			var x vaxfloat.Value

			if i%4 == 0 {
				x = randomValue(rng, f, -rng.IntN(int(f.Precision())+8), true)
			} else {
				x, _ = vaxfloat.Round(f, vaxfloat.FromFloat64((rng.Float64()*2-1)*limit*1.02))
			}

			got, err := exp(f, x)
			expectRounded(t, "EXP", f, x, got, err, referenceExp(x.Big()))
		}
	}
}

// TestLogsAgainstReference: all across each format's range of arguments, and
// just either side of 1, where a logarithm's bits are hardest to keep.
func TestLogsAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 4))
	two, ten := referenceLog(big.NewFloat(2)), referenceLog(big.NewFloat(10))

	for _, f := range allFormats {
		low, high := formatExponents(f)

		for i := range 150 {
			var x vaxfloat.Value

			switch i % 3 {
			case 0:
				x = randomValue(rng, f, low+rng.IntN(high-low+1), false)
			case 1:
				// 1 plus or minus a small amount.
				d := randomValue(rng, f, -rng.IntN(int(f.Precision())), true)
				x, _ = vaxfloat.Add(f, vaxfloat.FromInt(1), d)
			default:
				x = randomValue(rng, f, rng.IntN(3)-1, false)
			}

			if x.Cmp(vaxfloat.FromInt(1)) == 0 {
				continue
			}

			reference := referenceLog(x.Big())

			got, err := naturalLog(f, x)
			expectRounded(t, "LOG", f, x, got, err, reference)

			got, err = base2Log(f, x)
			expectRounded(t, "LOG2", f, x, got, err, newFloat(referenceBits).Quo(reference, two))

			got, err = base10Log(f, x)
			expectRounded(t, "LOG10", f, x, got, err, newFloat(referenceBits).Quo(reference, ten))
		}
	}
}

// TestSqrtAgainstReference checks a square root the exact way: r is the
// correctly rounded root of x exactly when x lies between the squares of the
// midpoints on either side of r.
func TestSqrtAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 5))

	for _, f := range allFormats {
		low, high := formatExponents(f)

		for range 1000 {
			x := randomValue(rng, f, low+rng.IntN(high-low+1), false)

			r, err := squareRoot(f, x)
			if err != nil {
				t.Fatalf("SQRT %v(%s): %v", f, x, err)
			}

			// An ulp of r: its last bit's value.
			root := r.Big()
			halfULP := new(big.Float).SetMantExp(big.NewFloat(1), root.MantExp(nil)-int(f.Precision())-1)
			lower := newFloat(4096).Sub(root, halfULP)
			upper := newFloat(4096).Add(root, halfULP)

			if newFloat(8192).Mul(lower, lower).Cmp(x.Big()) > 0 || newFloat(8192).Mul(upper, upper).Cmp(x.Big()) < 0 {
				t.Errorf("SQRT %v(%s) = %s isn't the nearest value", f, x, r)
			}
		}
	}
}

// TestKnownValues: published digits, in every format.
func TestKnownValues(t *testing.T) {
	for _, f := range allFormats {
		for _, c := range []struct {
			name   string
			fn     function
			arg    string
			digits string
		}{
			{"EXP", exp, "1", digitsE},
			{"LOG", naturalLog, "2", digitsLn2},
			{"LOG", naturalLog, "10", digitsLn10},
			{"LOG10", base10Log, "2", digitsLog2},
			{"LOG2", base2Log, "10", digitsLb10},
			{"SQRT", squareRoot, "2", digitsSqrt2},
		} {
			got, err := c.fn(f, value(t, f, c.arg))
			if want := value(t, f, c.digits); err != nil || got.Cmp(want) != 0 {
				t.Errorf("%s %v(%s) = %s, %v; want %s", c.name, f, c.arg, got, err, want)
			}
		}
	}
}

// TestExactValues (the phase doc's Decision 4): what has an exact answer gets
// it, to the last bit.
func TestExactValues(t *testing.T) {
	powerOfTen := big.NewInt(1)

	for n := range 11 { // F holds 10^10 exactly, and no more
		for _, f := range allFormats {
			got, err := base10Log(f, vaxfloat.FromBigInt(powerOfTen))
			if err != nil || got.Cmp(vaxfloat.FromInt(int64(n))) != 0 {
				t.Errorf("LOG10 %v(10^%d) = %s, %v", f, n, got, err)
			}
		}

		powerOfTen.Mul(powerOfTen, big.NewInt(10))
	}

	for _, f := range allFormats {
		low, high := formatExponents(f)

		for _, n := range []int{low, -100, -1, 0, 1, 3, 100, high - 1} {
			x := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), n))

			got, err := base2Log(f, x)
			if err != nil || got.Cmp(vaxfloat.FromInt(int64(n))) != 0 {
				t.Errorf("LOG2 %v(2^%d) = %s, %v", f, n, got, err)
			}
		}

		for _, c := range []struct {
			name      string
			fn        function
			arg, want float64
		}{
			{"SQRT", squareRoot, 0, 0},
			{"SQRT", squareRoot, 4, 2},
			{"SQRT", squareRoot, 2.25, 1.5},
			{"SQRT", squareRoot, 1 << 40, 1 << 20},
			{"EXP", exp, 0, 1},
			{"LOG", naturalLog, 1, 0},
			{"LOG2", base2Log, 1, 0},
			{"LOG10", base10Log, 1, 0},
		} {
			got, err := c.fn(f, vaxfloat.FromFloat64(c.arg))
			if err != nil || got.Cmp(vaxfloat.FromFloat64(c.want)) != 0 {
				t.Errorf("%s %v(%v) = %s, %v", c.name, f, c.arg, got, err)
			}
		}
	}
}

// TestExpRoundsOneUp: e^x for x just past half an ulp of 1 rounds up, and
// for x just short of it down, so the tiny term isn't lost on the way.
func TestExpRoundsOneUp(t *testing.T) {
	for _, f := range allFormats {
		p := int(f.Precision())
		ulp := new(big.Float).SetMantExp(big.NewFloat(1), 1-p)
		halfULP := new(big.Float).SetMantExp(big.NewFloat(1), -p)
		quarter := new(big.Float).SetMantExp(big.NewFloat(1), -p-1)

		got, err := exp(f, vaxfloat.FromBig(halfULP))
		if want := new(big.Float).SetPrec(uint(p)).Add(big.NewFloat(1), ulp); err != nil || got.Big().Cmp(want) != 0 {
			t.Errorf("EXP %v(2^%d) = %s, %v", f, -p, got, err)
		}

		got, err = exp(f, vaxfloat.FromBig(quarter))
		if err != nil || got.Cmp(vaxfloat.FromInt(1)) != 0 {
			t.Errorf("EXP %v(2^%d) = %s, %v", f, -p-1, got, err)
		}
	}
}

// TestManualsLimits: EXP's limits as the manual gives them, approximately, on
// each side. The G_floating underflow is at the format's limit, -709.783; the
// manual's -709.774 is a question for the VMS probe.
func TestManualsLimits(t *testing.T) {
	for _, c := range []struct {
		f                vaxfloat.Format
		fits, over       string
		fitsLow, under   string
	}{
		{vaxfloat.F, "88.029", "88.0297", "-88.7228", "-88.7229"},
		{vaxfloat.D, "88.029", "88.0297", "-88.7228", "-88.7229"},
		{vaxfloat.G, "709.089", "709.09", "-709.782", "-709.783"},
		{vaxfloat.H, "11355.830", "11355.831", "-11356.523", "-11356.524"},
	} {
		for _, in := range []string{c.fits, c.fitsLow} {
			if v, err := exp(c.f, value(t, c.f, in)); err != nil || v.IsZero() {
				t.Errorf("EXP %v(%s) = %s, %v", c.f, in, v, err)
			}
		}

		if _, err := exp(c.f, value(t, c.f, c.over)); !errors.Is(err, vaxfloat.ErrOverflow) {
			t.Errorf("EXP %v(%s): %v", c.f, c.over, err)
		}

		if _, err := exp(c.f, value(t, c.f, c.under)); !errors.Is(err, vaxfloat.ErrUnderflow) {
			t.Errorf("EXP %v(%s): %v", c.f, c.under, err)
		}

		// Far past the limits: the shortcut that doesn't compute.
		for _, huge := range []string{"1E30", "-1E30"} {
			want := vaxfloat.ErrOverflow
			if huge[0] == '-' {
				want = vaxfloat.ErrUnderflow
			}

			if _, err := exp(c.f, value(t, c.f, huge)); !errors.Is(err, want) {
				t.Errorf("EXP %v(%s): %v", c.f, huge, err)
			}
		}
	}
}

// TestDomains: the arguments with no answer say why.
func TestDomains(t *testing.T) {
	for _, f := range allFormats {
		for _, c := range []struct {
			name string
			fn   function
			arg  int64
			want domainError
		}{
			{"SQRT", squareRoot, -1, errSqrtNeg},
			{"LOG", naturalLog, 0, errLogZeroNeg},
			{"LOG", naturalLog, -1, errLogZeroNeg},
			{"LOG2", base2Log, 0, errLogZeroNeg},
			{"LOG2", base2Log, -1, errLogZeroNeg},
			{"LOG10", base10Log, 0, errLogZeroNeg},
			{"LOG10", base10Log, -1, errLogZeroNeg},
		} {
			if _, err := c.fn(f, vaxfloat.FromInt(c.arg)); !errors.Is(err, c.want) {
				t.Errorf("%s %v(%d): %v", c.name, f, c.arg, err)
			}
		}
	}
}

// BenchmarkFunctions measures a call in each format, against the survey's
// estimate of tens of VAX instructions' time.
func BenchmarkFunctions(b *testing.B) {
	for _, f := range allFormats {
		x, _ := vaxfloat.Round(f, vaxfloat.FromFloat64(1.2345678))

		for _, c := range []struct {
			name string
			fn   function
		}{{"SQRT", squareRoot}, {"EXP", exp}, {"LOG", naturalLog}, {"LOG10", base10Log}} {
			b.Run(fmt.Sprintf("%s/%v", c.name, f), func(b *testing.B) {
				for b.Loop() {
					if _, err := c.fn(f, x); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
