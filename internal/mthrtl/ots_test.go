package mthrtl

import (
	"errors"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/tucats/govax/internal/vaxfloat"
)

// logRange is about the size of the natural logarithm of each format's largest value.
func logRange(f vaxfloat.Format) float64 {
	switch f {
	case vaxfloat.G:
		return 709
	case vaxfloat.H:
		return 11356
	}

	return 88
}

// TestPowerAgainstReference holds the floating exponents' routines to
// referenceExp(y * referenceLog(x)), in every format: products all across the range
// of results and past both its ends, bases near 1 with large exponents, and tiny
// exponents, whose results are near 1.
func TestPowerAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 8))

	for _, f := range allFormats {
		low, high := formatExponents(f)

		for i := range 400 {
			var base, y vaxfloat.Value

			switch i % 4 {
			case 0, 1: // a base anywhere, and an exponent putting the product in reach
				base = randomValue(rng, f, low+1+rng.IntN(high-low-1), false)
				logBase, _ := referenceLog(base.Big()).Float64()
				target := (rng.Float64()*2.4 - 1.2) * logRange(f)

				if logBase == 0 {
					continue
				}

				y = value(t, f, big.NewFloat(target/logBase).Text('e', 7))

			case 2: // a base near 1, and an exponent to match
				nearby := 1 + rng.IntN(int(f.Precision())-1)
				one := big.NewFloat(1)
				d := randomValue(rng, f, -nearby, true).Big()
				base = value(t, f, newFloat(256).Add(one, d).Text('e', 60))
				y = randomValue(rng, f, 1+rng.IntN(nearby+1), true)

			case 3: // a tiny exponent
				base = randomValue(rng, f, 1+rng.IntN(high/2), false)
				y = randomValue(rng, f, -1-rng.IntN(int(f.Precision())+20), true)
			}

			product := newFloat(referenceBits).Mul(y.Big(), referenceLog(base.Big()))
			got, err := power(f, base, y)

			expectRounded(t, "power("+base.String()+")", f, y, got, err, referenceExp(product))
		}
	}
}

// TestPowerOfMixedFormats: OTS$POWRD's result has the exponent's bits, none lost to
// the base's format, and OTS$POWDR's the base's.
func TestPowerOfMixedFormats(t *testing.T) {
	nearOne := vaxfloat.AddExact(vaxfloat.FromInt(1), vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), -40)))

	got, err := power(vaxfloat.D, vaxfloat.FromInt(2), nearOne)
	if want := value(t, vaxfloat.D, "2.00000000000126082737653662442271986"); err != nil || got.Cmp(want) != 0 {
		t.Errorf("2 ** (1 + 2^-40) = %v, %v, want %v", got, err, want)
	}

	got, err = power(vaxfloat.D, nearOne, value(t, vaxfloat.F, "0.5"))
	if want := value(t, vaxfloat.D, "1.00000000000045474735088636072138095"); err != nil || got.Cmp(want) != 0 {
		t.Errorf("(1 + 2^-40) ** 0.5 = %v, %v, want %v", got, err, want)
	}
}

// exactPower is x ** n, for n positive, with every bit.
func exactPower(x *big.Float, n int) *big.Float {
	prec := x.MinPrec()*uint(n) + 64
	z := new(big.Float).SetPrec(prec).SetInt64(1)

	for range n {
		z.Mul(z, x)
	}

	return z
}

// TestPowerOfWholeExponents: with a whole exponent the true value is a product, a
// reference with no logarithm in it. Full-length random bases have no ties, which only
// a base of few bits can have (ots.go's comment).
func TestPowerOfWholeExponents(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 9))

	for _, f := range allFormats {
		for i := range 300 {
			n := 2 + i%40
			base := randomValue(rng, f, -2+rng.IntN(5), false)

			got, err := power(f, base, vaxfloat.FromInt(int64(n)))
			expectRounded(t, "power", f, base, got, err, exactPower(base.Big(), n))
		}
	}
}

// TestPowerExact: exact answers come out exact, by the rounding.
func TestPowerExact(t *testing.T) {
	for _, f := range allFormats {
		for _, c := range []struct{ base, y, want string }{
			{"8.0", "2.0", "64.0"},
			{"4.0", "0.5", "2.0"},
			{"16.0", "-0.25", "0.5"},
			{"2.0", "10.0", "1024.0"},
			{"3.0", "0.0", "1.0"},
			{"1.0", "12345.0", "1.0"},
			{"0.0", "3.0", "0.0"},
			{"10.0", "30.0", "1.0E30"},
		} {
			got, err := power(f, value(t, f, c.base), value(t, f, c.y))
			if want := value(t, f, c.want); err != nil || got.Cmp(want) != 0 {
				t.Errorf("%v: %s ** %s = %v, %v, want %v", f, c.base, c.y, got, err, want)
			}
		}
	}
}

// TestManualsPowers are the examples the OTS$ manual's pages print, to as many digits
// as they do.
func TestManualsPowers(t *testing.T) {
	for _, c := range []struct {
		name          string
		f             vaxfloat.Format
		base, y, want string
		bits          int
	}{
		{"OTS$POWRD", vaxfloat.D, "9768.0", "9.0", "8.0956338648832908E+35", 53},
		{"OTS$POWRD", vaxfloat.D, "7689.0", "-0.587436654545", "5.2155199252836588E-03", 53},
		{"OTS$POWGG", vaxfloat.G, "8.0", "2.0", "64.0000000000000", 53},
		{"OTS$POWGG", vaxfloat.G, "9.0", "-0.5", "0.333333333333333", 45},
		{"OTS$POWHH_R3", vaxfloat.H, "9877356535.0", "-0.5837653", "1.463779145994628357482343598205427E-0006", 108},
	} {
		got, err := power(c.f, value(t, c.f, c.base), value(t, c.f, c.y))
		if err != nil || !closeTo(got.Big(), decimal(t, c.want), c.bits) {
			t.Errorf("%s: %s ** %s = %v, %v, want %s", c.name, c.base, c.y, got, err, c.want)
		}
	}
}

// TestPowerTable is the floating exponents' table.
func TestPowerTable(t *testing.T) {
	for _, f := range allFormats {
		for _, c := range []struct {
			base, y string
			err     error
		}{
			{"-2.0", "2.0", errUndefinedPower},
			{"-2.0", "0.0", errUndefinedPower},
			{"-2.0", "-1.0", errUndefinedPower},
			{"0.0", "0.0", errUndefinedPower},
			{"0.0", "-1.0", errUndefinedPower},
			{"0.0", "1.0", nil},
			{"10.0", "6000.0", vaxfloat.ErrOverflow},
			{"10.0", "-6000.0", vaxfloat.ErrUnderflow},
			{"0.5", "1.0E30", vaxfloat.ErrUnderflow},
			{"2.0", "1.0E30", vaxfloat.ErrOverflow},
			{"2.0", "-1.0E30", vaxfloat.ErrUnderflow},
		} {
			if _, err := power(f, value(t, f, c.base), value(t, f, c.y)); !errors.Is(err, c.err) {
				t.Errorf("%v: %s ** %s: %v, want %v", f, c.base, c.y, err, c.err)
			}
		}
	}
}

// keepUnderflow is the gate of a caller with FU set.
func keepUnderflow(v vaxfloat.Value, err error) (vaxfloat.Value, error) { return v, err }

// TestIntegerPowerSteps: each format's products, each rounded, spelled out.
func TestIntegerPowerSteps(t *testing.T) {
	for _, f := range allFormats {
		x := value(t, f, "1.1")
		times := func(a, b vaxfloat.Value) vaxfloat.Value {
			v, err := vaxfloat.Mul(f, a, b)
			if err != nil {
				t.Fatal(err)
			}

			return v
		}

		x2 := times(x, x)
		x4 := times(x2, x2)
		x8 := times(x4, x4)
		x13 := times(times(x, x4), x8)

		got, err := integerPower(f, x, 13, noUnderflow)
		if err != nil || got.Cmp(x13) != 0 {
			t.Errorf("%v: 1.1 ** 13 = %v, %v, want %v", f, got, err, x13)
		}

		inverse, err := vaxfloat.Div(f, vaxfloat.FromInt(1), x13)
		if err != nil {
			t.Fatal(err)
		}

		got, err = integerPower(f, x, -13, noUnderflow)
		if err != nil || got.Cmp(inverse) != 0 {
			t.Errorf("%v: 1.1 ** -13 = %v, %v, want %v", f, got, err, inverse)
		}
	}
}

// TestIntegerPowerTable is the integer exponents' table, and what happens at the
// format's ends: a product too large is MTH$_FLOOVEMAT, even where its reciprocal
// would fit, and one too small is the caller's FU's to decide. The trap that follows
// is the one no real page lists.
func TestIntegerPowerTable(t *testing.T) {
	var trap trapError

	for _, f := range allFormats {
		_, high := formatExponents(f)

		edge := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), high-1))
		two := vaxfloat.FromInt(2)
		half := value(t, f, "0.5")

		for _, c := range []struct {
			name string
			base vaxfloat.Value
			n    int64
			want string
			err  error
		}{
			{"zero to zero", vaxfloat.Value{}, 0, "", errUndefinedPower},
			{"zero to negative", vaxfloat.Value{}, -1, "", errUndefinedPower},
			{"zero to positive", vaxfloat.Value{}, 3, "0.0", nil},
			{"negative to zero", value(t, f, "-2.0"), 0, "1.0", nil},
			{"negative to odd", value(t, f, "-2.0"), 3, "-8.0", nil},
			{"negative to even", value(t, f, "-2.0"), 4, "16.0", nil},
			{"negative to negative", value(t, f, "-2.0"), -3, "-0.125", nil},
			{"just in range", two, int64(high - 1), "", nil},
			{"its reciprocal", two, -int64(high - 1), "", nil},
			{"product too large", two, int64(high), "", vaxfloat.ErrOverflow},
			{"reciprocal would fit", two, -int64(high), "", vaxfloat.ErrOverflow},
			{"square too large", edge, 2, "", vaxfloat.ErrOverflow},
			{"too small, FU clear", half, 1 << 20, "0.0", nil},
		} {
			got, err := integerPower(f, c.base, c.n, noUnderflow)

			if !errors.Is(err, c.err) || errors.As(err, &trap) {
				t.Errorf("%v: %s: %v, want %v", f, c.name, err, c.err)
			} else if c.want != "" && err == nil && got.Cmp(value(t, f, c.want)) != 0 {
				t.Errorf("%v: %s = %v, want %s", f, c.name, got, c.want)
			}
		}

		if _, err := integerPower(f, half, 1<<20, keepUnderflow); !errors.Is(err, vaxfloat.ErrUnderflow) {
			t.Errorf("%v: too small, FU set: %v", f, err)
		}

		// With FU clear the products are 0.0 by the division.
		if _, err := integerPower(f, half, -(1 << 20), noUnderflow); !errors.As(err, &trap) || trap.condition != ssFltDiv {
			t.Errorf("%v: 1.0 over a product of 0.0: %v", f, err)
		}
	}
}

func TestIntegerPowerBits(t *testing.T) {
	for _, c := range []struct {
		w        width
		base, n  int64
		low      uint32
		overflow bool
	}{
		{word, 2, 14, 16384, false},
		{word, 2, 15, 0xFFFF8000, true},
		{word, -2, 15, 0xFFFF8000, false},
		{word, -2, 16, 0, true},
		{word, 181, 2, 32761, false},
		{word, 182, 2, 0xFFFF8164, true},
		{long, 2, 30, 1 << 30, false},
		{long, 2, 31, 0x80000000, true},
		{long, -2, 31, 0x80000000, false},
		{long, 3, 100, 3476558801, true},
		{long, 1, 1 << 31, 1, false},
		{long, -1, 1<<31 - 1, 0xFFFFFFFF, false},
		{long, 0, 1<<31 - 1, 0, false},
		{long, 2, 1<<31 - 1, 0, true},
		{long, 3, 1<<31 - 1, 2863311531, true}, // whose exact value would have 3.4 billion bits
		{long, 7, 0, 1, false},
		{unsignedLong, 2, 31, 0x80000000, false},
		{unsignedLong, 2, 32, 0, false},
		{unsignedLong, 3, 100, 3476558801, false},
		{unsignedLong, 0xFFFFFFFF, 2, 1, false},
	} {
		low, overflow := integerPowerBits(c.w, c.base, c.n)
		if low != c.low || overflow != c.overflow {
			t.Errorf("%+v ** : %08X, %v; want %08X, %v", c, low, overflow, c.low, c.overflow)
		}
	}
}

func TestNegativeIntegerPower(t *testing.T) {
	for _, c := range []struct{ base, n, want int64 }{
		{1, -5, 1}, {-1, -3, -1}, {-1, -4, 1}, {2, -1, 0}, {-2, -1, 0}, {100, -1 << 31, 0},
	} {
		if got := negativePower(c.base, c.n); got != c.want {
			t.Errorf("%d ** %d = %d, want %d", c.base, c.n, got, c.want)
		}
	}
}

// referenceComplexPower is exp(y * log(x)) by other means: the logarithm's imaginary
// part from referenceAtan and referencePi's half, in x's quadrant, and its real part
// from referenceLog on the exact sum of squares.
func referenceComplexPower(x, y complexValue) (re, im *big.Float) {
	w := uint(complexBits)

	sum := vaxfloat.AddExact(vaxfloat.MulExact(x.re, x.re), vaxfloat.MulExact(x.im, x.im)).Big()
	logRe := referenceLog(newFloat(w).Set(sum))
	logRe.SetMantExp(logRe, -1)

	var angle *big.Float

	switch {
	case x.re.IsZero():
		angle = halfReferencePi()
		if x.im.Sign() < 0 {
			angle.Neg(angle)
		}
	default:
		angle = referenceAtan(newFloat(w).Quo(x.im.Big(), x.re.Big()))

		if x.re.Sign() < 0 {
			pi := halfReferencePi()
			pi.SetMantExp(pi, 1)

			if x.im.Sign() < 0 {
				pi.Neg(pi)
			}

			angle = newFloat(w).Add(angle, pi)
		}
	}

	r := newFloat(w).Sub(newFloat(w).Mul(y.re.Big(), logRe), newFloat(w).Mul(y.im.Big(), angle))
	i := newFloat(w).Add(newFloat(w).Mul(y.re.Big(), angle), newFloat(w).Mul(y.im.Big(), logRe))

	e := referenceExp(r)
	s, c := referenceSinCos(i)

	return newFloat(w).Mul(e, c), newFloat(w).Mul(e, s)
}

// TestComplexPowerAgainstReference: bases in every quadrant and on the axes, and
// exponents of a size to keep the result in range.
func TestComplexPowerAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 10))

	for _, f := range complexFormats {
		for i := range 300 {
			x := complexValue{randomValue(rng, f, -8+rng.IntN(17), true), randomValue(rng, f, -8+rng.IntN(17), true)}

			switch i % 8 {
			case 0:
				x.im = vaxfloat.Value{}
			case 1:
				x.re = vaxfloat.Value{}
			}

			y := complexValue{randomValue(rng, f, -3+rng.IntN(5), true), randomValue(rng, f, -3+rng.IntN(5), true)}

			if i%8 == 2 {
				y.im = vaxfloat.Value{}
			}

			got, err := complexPower(f, x, y)
			re, im := referenceComplexPower(x, y)

			expectParts(t, "complexPower", f, x, got, err, re, im)
		}
	}
}

func TestComplexPowerValues(t *testing.T) {
	for _, f := range complexFormats {
		v := func(s string) vaxfloat.Value { return value(t, f, s) }
		one := complexValue{v("1.0"), vaxfloat.Value{}}

		// Anything to (0, 0), and 1 to anything.
		for _, c := range []struct {
			name string
			x, y complexValue
		}{
			{"(2, 3) ** 0", complexValue{v("2.0"), v("3.0")}, complexValue{}},
			{"1 ** (5, 7)", one, complexValue{v("5.0"), v("7.0")}},
		} {
			if got, err := complexPower(f, c.x, c.y); err != nil || got.re.Cmp(one.re) != 0 || !got.im.IsZero() {
				t.Errorf("%v: %s = (%v, %v), %v", f, c.name, got.re, got.im, err)
			}
		}

		// The first power gives the base back.
		base := complexValue{v("2.0"), v("3.0")}
		if got, err := complexPower(f, base, one); err != nil || got.re.Cmp(base.re) != 0 || got.im.Cmp(base.im) != 0 {
			t.Errorf("%v: (2, 3) ** 1 = (%v, %v), %v", f, got.re, got.im, err)
		}

		if _, err := complexPower(f, complexValue{}, one); !errors.Is(err, errInvalidArg) {
			t.Errorf("%v: (0, 0) ** 1: %v", f, err)
		}

		if _, err := complexPower(f, complexValue{}, complexValue{}); !errors.Is(err, errInvalidArg) {
			t.Errorf("%v: (0, 0) ** 0: %v", f, err)
		}

		if _, err := complexPower(f, complexValue{v("10.0"), vaxfloat.Value{}}, complexValue{v("1000.0"), vaxfloat.Value{}}); !errors.Is(err, vaxfloat.ErrOverflow) {
			t.Errorf("%v: 10 ** 1000: %v", f, err)
		}

		if got, err := complexPower(f, complexValue{v("10.0"), vaxfloat.Value{}}, complexValue{v("-1000.0"), vaxfloat.Value{}}); err != nil || !got.re.IsZero() || !got.im.IsZero() {
			t.Errorf("%v: 10 ** -1000 = (%v, %v), %v, want parts of 0.0", f, got.re, got.im, err)
		}

		// A base of size 1 on an axis and a huge exponent, a multiple of 4, are a
		// whole number of turns: the true imaginary part is exactly 0, and all the
		// routine's is the angle's error, far below the format's last bit of 1.
		i := complexValue{vaxfloat.Value{}, one.re}
		exponent := complexValue{v("1.0E38"), vaxfloat.Value{}}

		got, err := complexPower(f, i, exponent)
		if err != nil || got.re.Cmp(one.re) != 0 || !got.im.IsZero() && got.im.Big().MantExp(nil) > -int(f.Precision())-30 {
			t.Errorf("%v: i ** 1E38 = (%v, %v), %v", f, got.re, got.im, err)
		}

		// A base just off 1 and an exponent large enough that the angle is many turns
		// and the modulus stays in range, which takes the exponent's bits in the
		// logarithm: y * e is the angle, y * e^2 / 2 the size.
		near := complexValue{one.re, vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), -20))}
		large := complexValue{randomValue(rand.New(rand.NewPCG(77, 12)), f, 40, false), vaxfloat.Value{}}

		got, err = complexPower(f, near, large)
		re, im := referenceComplexPower(near, large)

		expectParts(t, "many turns", f, near, got, err, re, im)
	}
}

// TestManualsComplexPower is OTS$POWCC's page, to its seven digits. (OTS$POWCGCG_R3's
// example prints -0.46395650 twice, which is a misprint.)
func TestManualsComplexPower(t *testing.T) {
	f := vaxfloat.F
	got, err := complexPower(f, complexValue{value(t, f, "2.0"), value(t, f, "3.0")}, complexValue{value(t, f, "1.0"), value(t, f, "2.0")})

	if err != nil || !closeTo(got.re.Big(), decimal(t, "-0.4639565"), 22) || !closeTo(got.im.Big(), decimal(t, "-0.1995301"), 21) {
		t.Errorf("(2, 3) ** (1, 2) = (%v, %v), %v", got.re, got.im, err)
	}
}

// TestComplexIntegerPower: the products by hand, the reciprocal, and the conditions.
func TestComplexIntegerPower(t *testing.T) {
	var trap trapError

	for _, f := range complexFormats {
		v := func(s string) vaxfloat.Value { return value(t, f, s) }
		z := complexValue{v("1.1"), v("-0.3")}

		times := func(a, b complexValue) complexValue {
			p, err := complexProduct(f, a, b)
			if err != nil {
				t.Fatal(err)
			}

			return p
		}

		z2 := times(z, z)
		z4 := times(z2, z2)
		z5 := times(z, z4)

		got, err := complexIntegerPower(f, z, 5)
		if err != nil || got.re.Cmp(z5.re) != 0 || got.im.Cmp(z5.im) != 0 {
			t.Errorf("%v: ** 5 = (%v, %v), %v", f, got.re, got.im, err)
		}

		inverse, err := complexQuotient(f, complexValue{re: vaxfloat.FromInt(1)}, z5)
		if err != nil {
			t.Fatal(err)
		}

		got, err = complexIntegerPower(f, z, -5)
		if err != nil || got.re.Cmp(inverse.re) != 0 || got.im.Cmp(inverse.im) != 0 {
			t.Errorf("%v: ** -5 = (%v, %v), %v", f, got.re, got.im, err)
		}

		for _, n := range []int64{0, -1} {
			if _, err := complexIntegerPower(f, complexValue{}, n); !errors.Is(err, errUndefinedPower) {
				t.Errorf("%v: (0, 0) ** %d: %v", f, n, err)
			}
		}

		if got, err := complexIntegerPower(f, complexValue{}, 3); err != nil || !got.re.IsZero() || !got.im.IsZero() {
			t.Errorf("%v: (0, 0) ** 3 = (%v, %v), %v", f, got.re, got.im, err)
		}

		if got, err := complexIntegerPower(f, z, 0); err != nil || got.re.Cmp(vaxfloat.FromInt(1)) != 0 || !got.im.IsZero() {
			t.Errorf("%v: ** 0 = (%v, %v), %v", f, got.re, got.im, err)
		}

		if _, err := complexIntegerPower(f, complexValue{v("10.0"), v("10.0")}, 1<<20); !errors.As(err, &trap) || trap.condition != ssFltOvf {
			t.Errorf("%v: overflow: %v", f, err)
		}

		if _, err := complexIntegerPower(f, complexValue{v("0.1"), v("0.1")}, -(1 << 20)); !errors.As(err, &trap) || trap.condition != ssFltDiv {
			t.Errorf("%v: 1.0 over (0.0, 0.0): %v", f, err)
		}
	}
}

// TestComplexMultiplyDivide: each part rounded once, against the exact rationals, and
// the two routines' different conditions for one overflow.
func TestComplexMultiplyDivide(t *testing.T) {
	var trap trapError

	rng := rand.New(rand.NewPCG(77, 11))

	for _, f := range complexFormats {
		for range 300 {
			part := func() vaxfloat.Value { return randomValue(rng, f, -30+rng.IntN(60), true) }
			a, b, c, d := part(), part(), part(), part()
			x, y := complexValue{a, b}, complexValue{c, d}

			mul := func(p, q vaxfloat.Value) *big.Rat { return new(big.Rat).Mul(p.Rat(), q.Rat()) }

			got, err := complexProduct(f, x, y)
			re := new(big.Rat).Sub(mul(a, c), mul(b, d))
			im := new(big.Rat).Add(mul(a, d), mul(b, c))

			expectParts(t, "product", f, x, got, err, ratFloat(re), ratFloat(im))

			got, err = complexQuotient(f, x, y)
			den := new(big.Rat).Add(mul(c, c), mul(d, d))
			re = new(big.Rat).Quo(new(big.Rat).Add(mul(a, c), mul(b, d)), den)
			im = new(big.Rat).Quo(new(big.Rat).Sub(mul(b, c), mul(a, d)), den)

			expectParts(t, "quotient", f, x, got, err, ratFloat(re), ratFloat(im))
		}

		v := func(s string) vaxfloat.Value { return value(t, f, s) }
		large := "1.0E38"
		if f == vaxfloat.G {
			large = "1.0E306"
		}

		huge := complexValue{v(large), vaxfloat.Value{}}

		if _, err := complexProduct(f, huge, huge); !errors.Is(err, vaxfloat.ErrOverflow) || errors.As(err, &trap) {
			t.Errorf("%v: product's overflow: %v, want MTH$_FLOOVEMAT's", f, err)
		}

		if _, err := complexQuotient(f, huge, complexValue{v("0.01"), vaxfloat.Value{}}); !errors.As(err, &trap) || trap.condition != ssFltOvf {
			t.Errorf("%v: quotient's overflow: %v", f, err)
		}

		if _, err := complexQuotient(f, huge, complexValue{}); !errors.As(err, &trap) || trap.condition != ssFltDiv {
			t.Errorf("%v: divide by (0, 0): %v", f, err)
		}
	}
}

// ratFloat is r with bits to spare beyond any format's rounding.
func ratFloat(r *big.Rat) *big.Float { return newFloat(complexBits).SetRat(r) }

func BenchmarkOTS(b *testing.B) {
	for _, f := range allFormats {
		x, _ := vaxfloat.Round(f, vaxfloat.FromRat(big.NewRat(11, 10)))
		y, _ := vaxfloat.Round(f, vaxfloat.FromRat(big.NewRat(27, 10)))

		b.Run("power/"+f.String(), func(b *testing.B) {
			for b.Loop() {
				_, _ = power(f, x, y)
			}
		})
		b.Run("integerPower/"+f.String(), func(b *testing.B) {
			for b.Loop() {
				_, _ = integerPower(f, x, 13, noUnderflow)
			}
		})

		if f == vaxfloat.H {
			continue
		}

		z := complexValue{x, y}

		b.Run("complexPower/"+f.String(), func(b *testing.B) {
			for b.Loop() {
				_, _ = complexPower(f, z, z)
			}
		})
		b.Run("complexQuotient/"+f.String(), func(b *testing.B) {
			for b.Loop() {
				_, _ = complexQuotient(f, z, z)
			}
		})
	}
}
