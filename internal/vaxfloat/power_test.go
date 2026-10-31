package vaxfloat

import (
	"errors"
	"math/big"
	"testing"
)

func bigOf(x float64) *big.Float { return big.NewFloat(x) }

func powerOf(t *testing.T, f Format, base float64, n int64) (Value, error) {
	t.Helper()

	b, err := Round(f, FromBig(bigOf(base)))
	if err != nil {
		t.Fatal(err)
	}

	return PowerBySquaring(b, FromInt(1), n,
		func(x, y Value) (Value, error) { return Mul(f, x, y) },
		func(x, y Value) (Value, error) { return Div(f, x, y) })
}

// TestPowerSteps counts what PowerBySquaring asks of mul and div: a square only as
// far as a bit needs it, and for a negative exponent a division for each bit.
func TestPowerSteps(t *testing.T) {
	for _, c := range []struct {
		n        int64
		want     int64
		mul, div int
	}{
		{0, 1, 0, 0},
		{1, 3, 1, 0},
		{2, 9, 2, 0}, // 3 ** 2, then 1 * that
		{5, 243, 4, 0}, // two squares, two products
		{8, 6561, 4, 0},
		{-1, 3, 1, 1},
		{-5, 243, 4, 2},
	} {
		muls, divs := 0, 0

		got, err := PowerBySquaring(int64(3), 1, c.n,
			func(a, b int64) (int64, error) { muls++; return a * b, nil },
			func(a, b int64) (int64, error) {
				divs++

				if a != 1 {
					t.Errorf("n = %d: div of %d", c.n, a)
				}

				return b, nil // the divisor, to see the product
			})
		if err != nil {
			t.Errorf("n = %d: %v", c.n, err)
		}

		if got != c.want || muls != c.mul || divs != c.div {
			t.Errorf("n = %d: %d, %d multiplications, %d divisions; want %d, %d, %d", c.n, got, muls, divs, c.want, c.mul, c.div)
		}
	}
}

// TestPowerLargestExponents: |n| of 2**31 has no overflow of its own, and takes 31
// squares and one product.
func TestPowerLargestExponents(t *testing.T) {
	for _, n := range []int64{1 << 31, -(1 << 31), 1<<31 - 1, 1<<32 - 1} {
		if v, err := powerOf(t, F, 1, n); err != nil || v.Cmp(FromInt(1)) != 0 {
			t.Errorf("1 ** %d = %v, %v", n, v, err)
		}

		want := int64(1)
		if n&1 != 0 {
			want = -1
		}

		if v, err := powerOf(t, F, -1, n); err != nil || v.Cmp(FromInt(want)) != 0 {
			t.Errorf("-1 ** %d = %v, %v", n, v, err)
		}
	}
}

// TestPowerOrder: the product of reciprocals, not 1.0 over the product. In F_floating
// 2 ** -127 is in range, and 2 ** 127 isn't.
func TestPowerOrder(t *testing.T) {
	if _, err := powerOf(t, F, 2, 127); !errors.Is(err, ErrOverflow) {
		t.Errorf("2 ** 127: %v, want an overflow", err)
	}

	if v, err := powerOf(t, F, 2, -127); err != nil || v.Float64() != 1.0/(1<<63)/(1<<63)/2 {
		t.Errorf("2 ** -127 = %v, %v", v, err)
	}

	// 3 ** -5 differs in the last bit between the orders, if the case is any use. VMS
	// 7.1's bits are ^XD9063C86 (testdata/mth77).
	three := FromInt(3)
	inverse, _ := Div(F, FromInt(1), three)
	square, _ := Mul(F, three, three)
	fourth, _ := Mul(F, square, square)
	inverseFourth, _ := Div(F, FromInt(1), fourth)
	want, _ := Mul(F, inverse, inverseFourth)

	got, err := powerOf(t, F, 3, -5)
	if err != nil {
		t.Fatal(err)
	}

	if got.Cmp(want) != 0 {
		t.Errorf("3 ** -5 = %v, want %v", got, want)
	}

	if b, err := Pack(F, got); err != nil || uint32(b.Lo) != 0xD9063C86 {
		t.Errorf("3 ** -5's bits are %08X, %v", uint32(b.Lo), err)
	}

	if whole, _ := Div(F, FromInt(1), FromInt(243)); got.Cmp(whole) == 0 {
		t.Errorf("3 ** -5 can't tell the orders apart: both %v", got)
	}
}

func TestComplexArithmetic(t *testing.T) {
	v := func(i int64) Value { return FromInt(i) }

	re, im, err := ComplexMul(D, v(8), v(4), v(2), v(3))
	if err != nil || re.Cmp(v(4)) != 0 || im.Cmp(v(32)) != 0 {
		t.Errorf("(8, 4) * (2, 3) = (%v, %v), %v", re, im, err)
	}

	re, im, err = ComplexDiv(F, v(8), v(4), v(1), v(1))
	if err != nil || re.Cmp(v(6)) != 0 || im.Cmp(v(-2)) != 0 {
		t.Errorf("(8, 4) / (1, 1) = (%v, %v), %v", re, im, err)
	}

	if _, _, err = ComplexDiv(F, v(1), v(1), v(0), v(0)); !errors.Is(err, ErrDivideByZero) {
		t.Errorf("/ (0, 0): %v", err)
	}

	huge := FromBig(bigOf(1e38))

	if _, _, err = ComplexMul(F, huge, v(0), huge, v(0)); !errors.Is(err, ErrOverflow) {
		t.Errorf("huge * huge: %v", err)
	}

	tiny := FromBig(bigOf(1e-30))

	if re, im, err = ComplexMul(F, tiny, v(0), tiny, tiny); err != nil || !re.IsZero() || !im.IsZero() {
		t.Errorf("tiny * tiny = (%v, %v), %v, want parts of 0.0", re, im, err)
	}
}
