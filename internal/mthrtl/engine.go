package mthrtl

import (
	"math/big"
	"sync"

	"github.com/tucats/govax/internal/vaxfloat"
)

// The engine: the library's functions on arbitrary-precision numbers
// (math/big's big.Float), each written once and given a precision in bits.
// The phase doc's Decision 3 chose this over Go's math package, for all four
// floating formats, because math's results differ in the last bit from one
// host to another and a float64 can't hold every G_floating value.
//
// A routine works at its format's precision plus guardBits and rounds once,
// to the format, at the end (rounded, below). The result is the correctly
// rounded one unless the true value lies within 2^-guardBits of an ulp of a
// rounding boundary, about one argument in 2^40, and then it's off by one
// ulp (one unit in the format's last place). The routines stay within the
// plan's one ulp either way.
//
// A big.Float carries its own precision, and an operation such as
// z.Mul(x, y) rounds its exact result to z's precision. So a function here
// sets the precision of each number it makes. Its result's precision is the
// prec it was asked for, and each function works a little above that
// (scratchBits), since the errors of its many steps add up.

const (
	// guardBits is what a routine works at beyond its format's precision.
	guardBits = 40

	// scratchBits is what a function works at beyond the precision it was
	// asked for. A function takes some tens of steps, each off by at most
	// half a unit in the last place, so 16 bits are more than the steps'
	// errors can reach.
	scratchBits = 16

	// constantFloor is the least precision a constant is computed at, so
	// that the four formats' precisions, the largest being H's 113 bits and
	// the guard and scratch bits, need one computation.
	constantFloor = 256
)

// workingPrecision is the precision format f's routines ask the engine for.
func workingPrecision(f vaxfloat.Format) uint { return f.Precision() + guardBits }

// rounded rounds x to format f: the one rounding a result gets. It returns
// vaxfloat.ErrOverflow or vaxfloat.ErrUnderflow for a value out of f's range.
func rounded(f vaxfloat.Format, x *big.Float) (vaxfloat.Value, error) {
	return vaxfloat.Round(f, vaxfloat.FromBig(x))
}

// newFloat returns a new big.Float of precision prec, holding zero until
// it's given a value. A big.Float that's made with new and no precision
// takes its first operand's, which is rarely what's wanted here.
func newFloat(prec uint) *big.Float { return new(big.Float).SetPrec(prec) }

// constant is a mathematical constant, computed when first asked for and
// again only if it's asked for at more bits. Machines (and tests) can run on
// several goroutines, hence the mutex: sync.Mutex's zero value is an unlocked
// mutex, so a constant needs no constructor.
type constant struct {
	compute func(prec uint) *big.Float

	mu    sync.Mutex
	value *big.Float
}

// at returns the constant to at least prec bits. The caller must not change
// the result, which others share. An operation that reads it rounds to its
// own result's precision, so a value longer than asked for does no harm.
//
// Asked for more bits than it has, a constant at least doubles them, so that
// a program whose arguments keep growing (pi's bits follow an angle's size)
// has it recomputed a few times, not at each call.
func (c *constant) at(prec uint) *big.Float {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch {
	case c.value == nil:
		c.value = c.compute(max(prec, constantFloor))
	case c.value.Prec() < prec:
		c.value = c.compute(max(prec, 2*c.value.Prec()))
	}

	return c.value
}

// ln2 and ln10 are the natural logarithms of 2 and 10, from the series for
// the inverse hyperbolic tangent: ln(y) = 2 * atanh((y-1)/(y+1)), so
// ln 2 = 2 * atanh(1/3), and ln 10 = 3 ln 2 + ln(5/4) = 3 ln 2 + 2 * atanh(1/9).
var (
	ln2 = constant{compute: func(prec uint) *big.Float {
		return newFloat(prec).Mul(big.NewFloat(2), atanhOfReciprocal(3, prec+scratchBits))
	}}

	ln10 = constant{compute: func(prec uint) *big.Float {
		w := prec + scratchBits
		three := newFloat(w).Mul(big.NewFloat(3), ln2.at(w))
		rest := newFloat(w).Mul(big.NewFloat(2), atanhOfReciprocal(9, w))

		return newFloat(prec).Add(three, rest)
	}}
)

// atanhOfReciprocal returns atanh(1/n), to prec bits.
func atanhOfReciprocal(n int64, prec uint) *big.Float {
	return atanhSeries(newFloat(prec).Quo(big.NewFloat(1), newFloat(prec).SetInt64(n)), prec)
}

// atanhSeries returns atanh(z), to prec bits, from its power series
//
//	atanh(z) = z + z^3/3 + z^5/5 + z^7/7 + ...
//
// for |z| well below 1.
func atanhSeries(z *big.Float, prec uint) *big.Float {
	return oddPowerSeries(z, false, prec)
}

// atanSeries returns atan(z), to prec bits, from its power series
//
//	atan(z) = z - z^3/3 + z^5/5 - z^7/7 + ...
//
// for |z| well below 1.
func atanSeries(z *big.Float, prec uint) *big.Float {
	return oddPowerSeries(z, true, prec)
}

// oddPowerSeries sums z + s*z^3/3 + z^5/5 + s*z^7/7 + ..., to prec bits, s
// being -1 if alternating is set and 1 otherwise: multiplying by z^2, or by
// -z^2 to alternate, is the only difference between the two series. Each term
// gains the bits z^2 is below 1. It stops at the first term too small to change
// the sum. The sum's relative error is small however small z is, since the
// first term is z itself.
func oddPowerSeries(z *big.Float, alternating bool, prec uint) *big.Float {
	w := prec + scratchBits
	sum := newFloat(w).Set(z)

	if z.Sign() == 0 {
		return sum
	}

	square := newFloat(w).Mul(z, z)
	if alternating {
		square.Neg(square)
	}

	power := newFloat(w).Set(z) // z^(2k+1), with its sign, as k goes up
	term := newFloat(w)
	divisor := newFloat(w)

	for k := int64(1); ; k++ {
		power.Mul(power, square)
		term.Quo(power, divisor.SetInt64(2*k+1))

		// A number's MantExp (with a nil first argument, just the exponent
		// e of its value m * 2^e, 0.5 <= |m| < 1) says how large it is.
		if term.MantExp(nil) < sum.MantExp(nil)-int(w) {
			return sum
		}

		sum.Add(sum, term)
	}
}

// logarithm returns the natural logarithm of x, which must be positive, to
// prec bits.
//
// It writes x as m * 2^n with m between sqrt(1/2) and sqrt(2), so that
//
//	ln x = n * ln 2 + ln m,   ln m = 2 * atanh((m-1)/(m+1))
//
// and the series gains at least 5 bits a term. For x near 1, n is 0 and the
// result is the series alone, with the bits it needs however close x is to 1:
// m-1 is exact (a big.Float rounds a difference only after computing it
// exactly). Otherwise |n ln 2| is at least twice |ln m|, so their sum can't
// cancel.
func logarithm(x *big.Float, prec uint) *big.Float {
	w := prec + scratchBits

	m, n := nearOne(x)

	series := newFloat(w).Mul(big.NewFloat(2), logRatio(m, w))
	if n == 0 {
		return newFloat(prec).Set(series)
	}

	whole := newFloat(w).Mul(newFloat(w).SetInt64(int64(n)), ln2.at(w))

	return newFloat(prec).Add(whole, series)
}

// logBase2 returns the base 2 logarithm of x, which must be positive, to prec
// bits: n + ln(m) / ln 2 for nearOne's m and n. A power of 2 has m exactly 1,
// so its logarithm is exactly n.
func logBase2(x *big.Float, prec uint) *big.Float {
	w := prec + scratchBits

	m, n := nearOne(x)

	series := newFloat(w).Mul(big.NewFloat(2), logRatio(m, w))
	fraction := newFloat(w).Quo(series, ln2.at(w))

	return newFloat(prec).Add(newFloat(w).SetInt64(int64(n)), fraction)
}

// logBase10 returns the common logarithm of x, which must be positive, to
// prec bits: ln x / ln 10. A power of 10 comes out within far less than half
// an ulp of its integer logarithm, which the rounding to a format makes exact.
func logBase10(x *big.Float, prec uint) *big.Float {
	w := prec + scratchBits

	return newFloat(prec).Quo(logarithm(x, w), ln10.at(w))
}

// nearOne returns m and n with x = m * 2^n and m between sqrt(1/2) and
// sqrt(2). (MantExp gives m from 1/2 up to 1. The test against sqrt(1/2) needn't
// be exact: it only chooses the smaller of two series.)
func nearOne(x *big.Float) (*big.Float, int) {
	m := new(big.Float)
	n := x.MantExp(m)

	if m.Cmp(big.NewFloat(0.7071)) < 0 {
		m.SetMantExp(m, 1)
		n--
	}

	return m, n
}

// logRatio returns atanh((m-1)/(m+1)), half of ln m, to prec bits.
func logRatio(m *big.Float, prec uint) *big.Float {
	w := prec + scratchBits
	one := big.NewFloat(1)
	ratio := newFloat(w).Sub(m, one)

	return atanhSeries(ratio.Quo(ratio, newFloat(w).Add(m, one)), prec)
}

// exponentialHalvings is how many times exponential halves its reduced
// argument before the series, and so how many times it squares the result.
// Each halving makes every term of the series one bit smaller than the term
// before it would otherwise be, and each squaring doubles the error, which the
// working precision allows for.
const exponentialHalvings = 8

// exponentialOutOfRange is the exponent of an argument too large for any
// format's exponential: H's limit is about 11,356, below 2^14. The bound is
// well above that and small enough that k, below, can't overflow.
const exponentialOutOfRange = 20

// exponential returns e to the x, to prec bits.
//
// It writes x as k * ln 2 + r, with k an integer and |r| at most (ln 2)/2, so
//
//	e^x = 2^k * e^r
//
// and e^r comes from its Taylor series, 1 + r + r^2/2! + r^3/3! + ..., applied
// to r/2^s and squared s times, which needs fewer terms than the series on r.
// Since e^r is near 1, r needs only a good absolute accuracy, so the working
// precision goes up by the bits of x's integer part: however closely k * ln 2
// cancels x, what's left of r is accurate enough.
//
// An x beyond every format's limit returns a value beyond every format's
// range, of the right kind (very large or very small), not the true value.
func exponential(x *big.Float, prec uint) *big.Float {
	if x.Sign() == 0 {
		return newFloat(prec).SetInt64(1)
	}

	size := x.MantExp(nil)
	if size > exponentialOutOfRange {
		k := 1 << exponentialOutOfRange
		if x.Sign() < 0 {
			k = -k
		}

		return newFloat(prec).SetMantExp(big.NewFloat(1), k)
	}

	w := prec + scratchBits + exponentialHalvings + uint(max(size, 0))

	// k is x / ln 2 to the nearest integer. Int truncates, so add or subtract
	// a half first. A k off by one would only make r a little larger, so
	// 64 bits are plenty.
	quotient := newFloat(64).Quo(x, ln2.at(64))
	half := big.NewFloat(0.5)

	if x.Sign() < 0 {
		half.Neg(half)
	}

	k, _ := quotient.Add(quotient, half).Int64()

	r := newFloat(w).Mul(newFloat(w).SetInt64(k), ln2.at(w))
	r.Sub(x, r)
	r.SetMantExp(r, -exponentialHalvings)

	sum := newFloat(w).SetInt64(1)
	sum.Add(sum, r)

	term := newFloat(w).Set(r) // r^j / j!, as j goes up
	divisor := newFloat(w)

	for j := int64(2); ; j++ {
		term.Mul(term, r)
		term.Quo(term, divisor.SetInt64(j))

		if term.Sign() == 0 || term.MantExp(nil) < -int(w) {
			break
		}

		sum.Add(sum, term)
	}

	for range exponentialHalvings {
		sum.Mul(sum, sum)
	}

	// SetMantExp(sum, k) is sum * 2^k, exactly.
	return newFloat(prec).Set(sum.SetMantExp(sum, int(k)))
}
