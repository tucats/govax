package mthrtl

import (
	"errors"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/tucats/govax/internal/vaxfloat"
)

// justAboveSmallest is the value next above format f's smallest: its last fraction
// bit set too.
func justAboveSmallest(f vaxfloat.Format) vaxfloat.Value {
	low, _ := formatExponents(f)
	x := new(big.Float).SetMantExp(big.NewFloat(1), low-1)
	step := new(big.Float).SetMantExp(big.NewFloat(1), low-int(f.Precision()))

	return vaxfloat.FromBig(new(big.Float).SetPrec(f.Precision()).Add(x, step))
}

// largestOf is format f's largest value: all fraction bits set, at the top
// exponent.
func largestOf(f vaxfloat.Format) vaxfloat.Value {
	_, high := formatExponents(f)
	one := new(big.Float).SetPrec(f.Precision() + 1).SetInt64(1)
	x := new(big.Float).SetPrec(f.Precision()+1).SetMantExp(one, high)
	step := new(big.Float).SetPrec(f.Precision()+1).SetMantExp(one, high-int(f.Precision()))

	return vaxfloat.FromBig(x.Sub(x, step))
}

// wholeReference is x's whole numbers by rationals: its quotient truncated, then
// adjusted, with no use of the code's Int or Split.
func wholeReference(x vaxfloat.Value) (truncate, near, floor *big.Int) {
	r := x.Rat()

	truncate = new(big.Int).Quo(r.Num(), r.Denom())
	floor = new(big.Int).Div(r.Num(), r.Denom()) // Euclidean: the denominator is positive

	half := big.NewRat(1, 2)
	if r.Sign() < 0 {
		half.Neg(half)
	}

	shifted := new(big.Rat).Add(r, half)
	near = new(big.Int).Quo(shifted.Num(), shifted.Denom())

	return truncate, near, floor
}

// wholeCases are values of format f to try: the small ones with fractions, near
// halves, at the top where everything's whole, and random across the range.
func wholeCases(rng *rand.Rand, f vaxfloat.Format, i int) vaxfloat.Value {
	low, high := formatExponents(f)
	exponent := []int{
		rng.IntN(8) - 2, rng.IntN(int(f.Precision())) + 1, int(f.Precision()) - rng.IntN(4),
		rng.IntN(high-low) + low,
	}[i%4]

	return randomValue(rng, f, exponent, true)
}

// TestWholeNumbers holds the routines' three ways of making a whole number to the
// rationals, in every format, with floating results and integer ones.
func TestWholeNumbers(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 11))

	for _, f := range allFormats {
		for i := range 4000 {
			x := wholeCases(rng, f, i)
			if _, err := vaxfloat.Pack(f, x); err != nil {
				continue // past the top of the format
			}

			wantTruncate, wantNear, wantFloor := wholeReference(x)

			for _, c := range []struct {
				name string
				fn   func(vaxfloat.Value) *big.Int
				want *big.Int
			}{{"truncated", truncated, wantTruncate}, {"nearest", nearest, wantNear}, {"floorOf", floorOf, wantFloor}} {
				if got := c.fn(x); got.Cmp(c.want) != 0 {
					t.Fatalf("%v %s(%s) = %v, want %v", f, c.name, x, got, c.want)
				}

				got, err := wholeNumber(c.fn)(f, x)
				if err != nil {
					t.Fatalf("%v %s(%s): %v", f, c.name, x, err)
				}

				if got.Rat().Cmp(new(big.Rat).SetInt(c.want)) != 0 {
					t.Fatalf("%v %s(%s) = %s as a float, want %v", f, c.name, x, got, c.want)
				}
			}
		}
	}
}

// TestWholeNumberCases: values the random ones wouldn't land on, whose right
// answers are known by hand. The value just below 0.5 is the case where the FORTRAN
// manual's INT(a + .5) would round up.
func TestWholeNumberCases(t *testing.T) {
	justBelowHalf := func(f vaxfloat.Format) vaxfloat.Value {
		half := new(big.Float).SetPrec(f.Precision() + 1).SetFloat64(0.5)
		step := new(big.Float).SetMantExp(big.NewFloat(1), -1-int(f.Precision()))

		return vaxfloat.FromBig(half.Sub(half, step))
	}

	for _, f := range allFormats {
		for _, c := range []struct {
			text                  string
			truncate, near, floor int64
		}{
			{"0", 0, 0, 0},
			{"0.25", 0, 0, 0},
			{"0.5", 0, 1, 0},
			{"-0.5", 0, -1, -1},
			{"1.5", 1, 2, 1},
			{"-1.5", -1, -2, -2},
			{"2.5", 2, 3, 2},
			{"-2.5", -2, -3, -3},
			{"-0.25", 0, 0, -1},
			{"7", 7, 7, 7},
			{"-7", -7, -7, -7},
		} {
			x := value(t, f, c.text)

			if got := truncated(x); got.Int64() != c.truncate {
				t.Errorf("%v truncated(%s) = %v", f, c.text, got)
			}

			if got := nearest(x); got.Int64() != c.near {
				t.Errorf("%v nearest(%s) = %v", f, c.text, got)
			}

			if got := floorOf(x); got.Int64() != c.floor {
				t.Errorf("%v floorOf(%s) = %v", f, c.text, got)
			}
		}

		if got := nearest(justBelowHalf(f)); got.Sign() != 0 {
			t.Errorf("%v nearest(just below 0.5) = %v, want 0", f, got)
		}

		if got := nearest(justBelowHalf(f).Neg()); got.Sign() != 0 {
			t.Errorf("%v nearest(just above -0.5) = %v, want 0", f, got)
		}

		// The largest value is whole, and comes back as itself.
		top := largestOf(f)
		if _, err := vaxfloat.Pack(f, top); err != nil {
			t.Fatalf("%v's largest doesn't pack: %v", f, err)
		}

		for _, fn := range []func(vaxfloat.Value) *big.Int{truncated, nearest, floorOf} {
			if got, err := wholeNumber(fn)(f, top); err != nil || got.Cmp(top) != 0 {
				t.Errorf("%v: largest whole = %s, %v; want %s", f, got, err, top)
			}
		}
	}

	// A floor that carries: -(2^24 - 1/2) to -(2^24), still in F_floating.
	x := value(t, vaxfloat.F, "-16777215.5")
	if got, err := wholeNumber(floorOf)(vaxfloat.F, x); err != nil || got.Float64() != -16777216 {
		t.Errorf("a floor that carries = %s, %v", got, err)
	}
}

// remainderReference is a - INT(a/b) * b in rationals.
func remainderReference(a, b vaxfloat.Value) *big.Rat {
	q := new(big.Rat).Quo(a.Rat(), b.Rat())
	whole := new(big.Rat).SetInt(new(big.Int).Quo(q.Num(), q.Denom()))

	return new(big.Rat).Sub(a.Rat(), new(big.Rat).Mul(whole, b.Rat()))
}

// TestRemainder: the result is exact in every format, across quotients from below
// 1 to far beyond the format's precision, so nothing rounded it, it has the
// dividend's sign, and it's smaller than the divisor.
func TestRemainder(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 12))

	for _, f := range allFormats {
		low, high := formatExponents(f)
		tried := 0

		for i := range 4000 {
			exponent := rng.IntN((high-low)/2) + low/2

			a := randomValue(rng, f, exponent, true)
			b := randomValue(rng, f, exponent-[]int{0, 3, 40, 200, -20}[i%5], true)

			if _, err := vaxfloat.Div(f, a, b); err != nil {
				continue
			}

			tried++

			got, err := remainder(f, a, b)
			if err != nil {
				t.Fatalf("%v remainder(%s, %s): %v", f, a, b, err)
			}

			want := remainderReference(a, b)
			if got.Rat().Cmp(want) != 0 {
				t.Fatalf("%v remainder(%s, %s) = %s, want %s", f, a, b, got, want.FloatString(40))
			}

			if got.Sign() != 0 && got.Sign() != a.Sign() {
				t.Fatalf("%v remainder(%s, %s) = %s: the wrong sign", f, a, b, got)
			}

			if got.Abs().Cmp(b.Abs()) >= 0 {
				t.Fatalf("%v remainder(%s, %s) = %s: not below the divisor", f, a, b, got)
			}
		}

		if tried < 3000 {
			t.Errorf("%v: only %d of the cases were tried", f, tried)
		}
	}
}

// TestRemainderCases: the signs (FORTRAN's MOD has the dividend's), a dividend
// smaller than the divisor, and the failures.
func TestRemainderCases(t *testing.T) {
	for _, f := range allFormats {
		for _, c := range []struct{ a, b, want string }{
			{"5.5", "2", "1.5"},
			{"-5.5", "2", "-1.5"},
			{"5.5", "-2", "1.5"},
			{"-5.5", "-2", "-1.5"},
			{"1", "4", "1"},
			{"-1", "4", "-1"},
			{"6", "2", "0"},
			{"0", "2", "0"},
		} {
			got, err := remainder(f, value(t, f, c.a), value(t, f, c.b))
			if err != nil || got.Cmp(value(t, f, c.want)) != 0 {
				t.Errorf("%v remainder(%s, %s) = %s, %v; want %s", f, c.a, c.b, got, err, c.want)
			}
		}

		// A dividend far below the divisor is itself: no underflow of the quotient.
		low, high := formatExponents(f)
		a := smallest(f)
		b := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), high-1))

		if got, err := remainder(f, a, b); err != nil || got.Cmp(a) != 0 {
			t.Errorf("%v remainder(smallest, huge) = %s, %v", f, got, err)
		}

		_, err := remainder(f, value(t, f, "1"), vaxfloat.Value{})
		if !errors.Is(err, vaxfloat.ErrDivideByZero) {
			t.Errorf("%v remainder by zero: %v", f, err)
		}

		// A quotient beyond the format.
		_, err = remainder(f, b, a)
		if !errors.Is(err, vaxfloat.ErrOverflow) {
			t.Errorf("%v remainder(huge, smallest): %v", f, err)
		}

		// A true remainder below the format's range: 2 * smallest less a little
		// more than smallest. F's and D's footnote gives them underflow, and G and H
		// are 0.0 quietly.
		twice := vaxfloat.FromBig(new(big.Float).SetMantExp(big.NewFloat(1), low))

		got, err := remainder(f, twice, justAboveSmallest(f))
		if f == vaxfloat.F || f == vaxfloat.D {
			if !errors.Is(err, vaxfloat.ErrUnderflow) {
				t.Errorf("%v remainder below the range: %s, %v", f, got, err)
			}
		} else if err != nil || !got.IsZero() {
			t.Errorf("%v remainder below the range: %s, %v", f, got, err)
		}
	}
}

// TestFloatingRoutines: the small functions, their limits, and which condition each
// failure is.
func TestFloatingRoutines(t *testing.T) {
	asTrap := func(err error) uint32 {
		var trap trapError
		if errors.As(err, &trap) {
			return trap.condition
		}

		return 0
	}

	for _, f := range allFormats {
		for _, c := range []struct{ x, y, abs, dim, sign string }{
			{"1.5", "1", "1.5", "0.5", "1.5"},
			{"1", "1.5", "1", "0", "1"},
			{"-1.5", "1", "1.5", "0", "1.5"},
			{"1.5", "-1", "1.5", "2.5", "-1.5"},
			{"-1.5", "-1", "1.5", "0", "-1.5"},
			{"-1.5", "0", "1.5", "0", "1.5"},
			{"0", "-1", "0", "1", "0"},
			{"2", "2", "2", "0", "2"},
		} {
			x, y := value(t, f, c.x), value(t, f, c.y)

			if got, err := absolute(f, x); err != nil || got.Cmp(value(t, f, c.abs)) != 0 {
				t.Errorf("%v absolute(%s) = %s, %v", f, c.x, got, err)
			}

			if got, err := positiveDifference(f, x, y); err != nil || got.Cmp(value(t, f, c.dim)) != 0 {
				t.Errorf("%v positiveDifference(%s, %s) = %s, %v", f, c.x, c.y, got, err)
			}

			if got, err := transferSign(f, x, y); err != nil || got.Cmp(value(t, f, c.sign)) != 0 {
				t.Errorf("%v transferSign(%s, %s) = %s, %v", f, c.x, c.y, got, err)
			}
		}

		// Opposite signs can overflow, and near-equal values at the bottom of the
		// range underflow, which no footnote of the 7.3 manual lists.
		top := largestOf(f)
		if _, err := positiveDifference(f, top, top.Neg()); asTrap(err) != ssFltOvf {
			t.Errorf("%v DIM of the extremes: %v", f, err)
		}

		if got, err := positiveDifference(f, justAboveSmallest(f), smallest(f)); err != nil || !got.IsZero() {
			t.Errorf("%v DIM below the range: %s, %v", f, got, err)
		}
	}

	// MTH$SNGL rounds up past F_floating's top, and D_floating holds what's below.
	top := largestOf(vaxfloat.D)
	if _, err := convertTrapping(vaxfloat.F, top); asTrap(err) != ssFltOvf {
		t.Errorf("SNGL of D's largest: %v", err)
	}

	if got, err := convertTrapping(vaxfloat.F, smallest(vaxfloat.D)); err != nil || got.Cmp(smallest(vaxfloat.F)) != 0 {
		t.Errorf("SNGL of D's smallest: %s, %v", got, err)
	}

	// MTH$SNGLG leaves the range at both ends.
	if _, err := convertTrapping(vaxfloat.F, largestOf(vaxfloat.G)); asTrap(err) != ssFltOvf {
		t.Errorf("SNGLG of G's largest: %v", err)
	}

	_, err := convertTrapping(vaxfloat.F, smallest(vaxfloat.G))
	if asTrap(err) != ssFltUnd || !errors.Is(err, vaxfloat.ErrUnderflow) {
		t.Errorf("SNGLG of G's smallest: %v", err)
	}

	// Rounding: 2^24 + 1 in F_floating is a tie, away from zero. And MTH$CVT_G_D
	// leaves the range by the library's own conditions, not a hardware exception.
	if got, err := convert(vaxfloat.F, vaxfloat.FromInt(1<<24+1)); err != nil || got.Float64() != 1<<24+2 {
		t.Errorf("FLOATJ of a tie: %s, %v", got, err)
	}

	if _, err := convert(vaxfloat.D, largestOf(vaxfloat.G)); asTrap(err) != 0 || !errors.Is(err, vaxfloat.ErrOverflow) {
		t.Errorf("CVT_G_D of G's largest: %v", err)
	}

	// D_floating holds 56 bits and G_floating 53.
	wide := value(t, vaxfloat.D, "36028797018963969") // 2^55 + 1
	if got, err := convert(vaxfloat.G, wide); err != nil || got.Float64() != 1<<55 {
		t.Errorf("CVT_D_G of 2^55 + 1: %s, %v", got, err)
	}
}

// TestWidths: an integer type's low bits, which are how an overflow's result gets
// to R0, and its range.
func TestWidths(t *testing.T) {
	huge, _ := new(big.Int).SetString("-123456789012345678901234567890", 10)

	for _, c := range []struct {
		w     width
		i     *big.Int
		low   uint32
		holds bool
	}{
		{word, big.NewInt(32767), 32767, true},
		{word, big.NewInt(32768), 0xFFFF8000, false},
		{word, big.NewInt(-32768), 0xFFFF8000, true},
		{word, big.NewInt(-32769), 32767, false},
		{word, big.NewInt(65537), 1, false},
		{long, big.NewInt(1<<31 - 1), 1<<31 - 1, true},
		{long, big.NewInt(1 << 31), 0x80000000, false},
		{long, big.NewInt(-1 << 31), 0x80000000, true},
		{long, big.NewInt(-1<<31 - 1), 0x7FFFFFFF, false},
		{long, big.NewInt(-1), 0xFFFFFFFF, true},
		{long, huge, 0xB1C0F52E, false},
		{unsignedLong, big.NewInt(1<<32 - 1), 0xFFFFFFFF, true},
		{unsignedLong, big.NewInt(1 << 32), 0, false},
		{unsignedLong, big.NewInt(-1), 0xFFFFFFFF, false},
	} {
		if got := c.w.lowBits(c.i); got != c.low {
			t.Errorf("%+v.lowBits(%v) = %#x, want %#x", c.w, c.i, got, c.low)
		}

		if got := c.w.holds(c.i); got != c.holds {
			t.Errorf("%+v.holds(%v) = %v", c.w, c.i, got)
		}
	}
}

// TestIntegerFunctions: each function, at the ends of its width.
func TestIntegerFunctions(t *testing.T) {
	const (
		maxWord = 1<<15 - 1
		minWord = -1 << 15
		maxLong = 1<<31 - 1
		minLong = -1 << 31
	)

	for _, c := range []struct {
		name string
		fn   integerFunction
		w    width
		args []int64
		want int64
	}{
		{"ABS", integerAbsolute, word, []int64{-5}, 5},
		{"ABS", integerAbsolute, word, []int64{5}, 5},
		{"ABS", integerAbsolute, word, []int64{minWord}, maxWord + 1},
		{"ABS", integerAbsolute, long, []int64{minLong}, maxLong + 1},
		{"DIM", integerDifference, long, []int64{5, 3}, 2},
		{"DIM", integerDifference, long, []int64{3, 5}, 0},
		{"DIM", integerDifference, long, []int64{3, 3}, 0},
		{"DIM", integerDifference, word, []int64{maxWord, -1}, maxWord + 1},
		{"DIM", integerDifference, long, []int64{maxLong, minLong}, maxLong - minLong},
		{"SIGN", integerSign, long, []int64{5, -1}, -5},
		{"SIGN", integerSign, long, []int64{-5, 1}, 5},
		{"SIGN", integerSign, long, []int64{-5, 0}, 5},
		{"SIGN", integerSign, word, []int64{-5, -1}, -5},
		{"SIGN", integerSign, word, []int64{minWord, 1}, minWord}, // no positive: it stays
		{"SIGN", integerSign, long, []int64{minLong, 1}, minLong},
		{"SIGN", integerSign, long, []int64{minLong, -1}, minLong},
		{"AND", integerAnd, word, []int64{-1, 0x0F0F}, 0x0F0F},
		{"OR", integerOr, word, []int64{-0x8000, 1}, -0x7FFF},
		{"EOR", integerExclusiveOr, long, []int64{-1, 1}, -2},
		{"NOT", integerNot, word, []int64{0}, -1},
		{"NOT", integerNot, long, []int64{-1}, 0},
		{"SHFT", integerShift, word, []int64{1, 15}, minWord},
		{"SHFT", integerShift, word, []int64{3, 15}, minWord}, // the bit beyond is gone
		{"SHFT", integerShift, word, []int64{1, 16}, 0},
		{"SHFT", integerShift, word, []int64{-1, -15}, 1}, // logical: zeros come in
		{"SHFT", integerShift, word, []int64{-1, -16}, 0},
		{"SHFT", integerShift, word, []int64{-1, 0}, -1},
		{"SHFT", integerShift, long, []int64{1, 31}, minLong},
		{"SHFT", integerShift, long, []int64{1, 32}, 0},
		{"SHFT", integerShift, long, []int64{-1, -31}, 1},
		{"SHFT", integerShift, long, []int64{-1, -32}, 0},
		{"SHFT", integerShift, long, []int64{6, -1}, 3},
		{"SHFT", integerShift, long, []int64{6, 1}, 12},
		{"SHFT", integerShift, long, []int64{1, 1 << 31}, 0}, // counts far past the width
		{"SHFT", integerShift, long, []int64{-1, -1 << 31}, 0},
		{"SHFT", integerShift, word, []int64{-1, 100}, 0},
	} {
		if got := c.fn(c.w, c.args); got != c.want {
			t.Errorf("%s%v (%+v) = %d, want %d", c.name, c.args, c.w, got, c.want)
		}
	}

	if got := signOf(value(t, vaxfloat.F, "-2.5")); got.Int64() != -1 {
		t.Errorf("signOf(-2.5) = %v", got)
	}

	if got := signOf(vaxfloat.Value{}); got.Sign() != 0 {
		t.Errorf("signOf(0) = %v", got)
	}
}
