package mthrtl

import (
	"math/big"
)

// The trigonometric functions on arbitrary-precision numbers, in the engine's
// way (engine.go): each takes a precision, works above it, and leaves the one
// rounding to the routine that called it.
//
// A sine or cosine is two steps: reduce the angle to a small one and its
// quadrant, then sum the series on the small one. Everything that makes these
// functions hard is in the reduction. See reduceRadians and reduceDegrees.

// pi is the ratio of a circle's circumference to its diameter. An angle in
// H_floating can be as large as 2^16383, and reducing it needs as many bits of
// pi as that, so this constant can grow to some thousands of bits.
var pi = constant{compute: computePi}

// computePi returns pi to prec bits, from Machin's formula
//
//	pi/4 = 4 * atan(1/5) - atan(1/239)
//
// in fixed point: each number is a big.Int standing for that number times 2^w.
// A term of the series is then the last one divided by a small integer, which
// takes time in proportion to the number's length, where a big.Float's series
// (oddPowerSeries) multiplies two long numbers for each term. At H_floating's
// thousands of bits that's most of an order of magnitude.
func computePi(prec uint) *big.Float {
	// Each of the series' few thousand terms is off by less than 1 in the last
	// place, and 32 bits are more than their errors can reach.
	w := prec + 32

	sum := fixedAtanOfReciprocal(5, w)
	sum.Mul(sum, big.NewInt(4))
	sum.Sub(sum, fixedAtanOfReciprocal(239, w))
	sum.Lsh(sum, 2)

	// SetInt rounds to the precision the number has; SetMantExp then divides
	// by 2^w, exactly.
	result := newFloat(prec).SetInt(sum)

	return result.SetMantExp(result, -int(w))
}

// fixedAtanOfReciprocal returns atan(1/n) times 2^w, as an integer: the series
// 1/n - 1/(3 n^3) + 1/(5 n^5) - ..., which ends when the power of 1/n has no
// bits left.
func fixedAtanOfReciprocal(n int64, w uint) *big.Int {
	power := new(big.Int).Lsh(big.NewInt(1), w) // 2^w / n^(2k+1), as k goes up
	power.Quo(power, big.NewInt(n))

	sum := new(big.Int).Set(power)
	square := big.NewInt(n * n)
	term := new(big.Int)

	for k := int64(1); ; k++ {
		power.Quo(power, square)
		if power.Sign() == 0 {
			return sum
		}

		term.Quo(power, big.NewInt(2*k+1))

		if k%2 == 1 {
			sum.Sub(sum, term)
		} else {
			sum.Add(sum, term)
		}
	}
}

// halfPi returns pi/2 to prec bits. It rounds pi to that length, since pi may
// be held to thousands of bits and a product takes time in its operands'
// lengths, not its result's.
func halfPi(prec uint) *big.Float {
	half := newFloat(prec).Set(pi.at(prec))

	return half.SetMantExp(half, -1)
}

// reductionSlack is how many bits of cancellation reduceRadians first allows
// for. Few arguments cancel so many, and it tries again for those that do.
const reductionSlack = 32

// reduceRadians writes x, which must be positive, as n * pi/2 + r with n an
// integer and |r| about pi/4 at most, and returns r and the low two bits of n,
// the quadrant: the sines and cosines of x and r differ only by the quadrant's
// signs and exchange (fromQuadrant). r is accurate to prec bits and some.
//
// r is a difference of two nearly equal numbers, so its leading bits cancel,
// and with pi only ever known approximately those bits have to be worked for:
//
//   - x's own size: n * pi/2 is as large as x, so to know r to its last bit,
//     pi needs the bits of x's integer part as well. This is why H_floating's
//     largest angles need 16,000 bits of pi.
//   - How near x is to a multiple of pi/2. MTH$COS of the VAX's nearest value
//     to pi/2 cancels all of that value's bits. No number here is exactly a
//     multiple (pi is irrational, and x isn't), so r is never truly 0, and
//     how far the cancellation can go is limited but hard to bound well. So
//     the code allows reductionSlack bits and looks at what it got: if r is
//     smaller than that allows, it doubles the slack and does it again.
func reduceRadians(x *big.Float, prec uint) (r *big.Float, quadrant uint) {
	// An angle below pi/4 needs no reduction, and 0.785 is below it.
	if x.Cmp(big.NewFloat(0.785)) < 0 {
		return x, 0
	}

	size := uint(max(x.MantExp(nil), 0))

	for slack := uint(reductionSlack); ; slack *= 2 {
		w := prec + scratchBits + size + slack
		half := halfPi(w)

		// n is x / (pi/2) to the nearest integer. It needn't be exactly the
		// nearest: r would only be a little larger than pi/4. Int truncates,
		// so add a half first.
		quotient := newFloat(size+16).Quo(x, half)
		n, _ := quotient.Add(quotient, big.NewFloat(0.5)).Int(nil)

		r = newFloat(w).Mul(newFloat(w).SetInt(n), half)
		r.Sub(x, r)

		if r.Sign() != 0 && r.MantExp(nil) >= -int(slack) {
			return r, uint(n.Bit(0)) | uint(n.Bit(1))<<1
		}
	}
}

// reduceDegrees is reduceRadians for an angle in degrees: x, which must be
// positive, is n * 90 + r with |r| at most 45. Here the arithmetic is exact, in
// integers (the phase doc's Decision 4), which radians can't be. So an angle
// that's a multiple of 90 reduces to exactly 0, and its sine and cosine are
// exactly 0 and 1, however large it is.
//
// x is a / 2^s for integers a and s, and n and r come from dividing a by
// 90 * 2^s. r is the remainder over 2^s, which a big.Float holds exactly.
func reduceDegrees(x *big.Float) (r *big.Float, quadrant uint) {
	if x.Cmp(big.NewFloat(45)) <= 0 {
		return x, 0
	}

	// MantExp gives a fraction and an exponent, and MinPrec is how many of the
	// fraction's bits are in use: shifted up by that many it's an integer.
	fraction := new(big.Float)
	exponent := x.MantExp(fraction)
	bits := int(fraction.MinPrec())

	a, _ := fraction.SetMantExp(fraction, bits).Int(nil)
	exponent -= bits

	s := uint(0)
	if exponent >= 0 {
		a.Lsh(a, uint(exponent))
	} else {
		s = uint(-exponent)
	}

	ninety := new(big.Int).Lsh(big.NewInt(90), s)
	n, remainder := new(big.Int).QuoRem(a, ninety, new(big.Int))

	// To the nearest multiple of 90: a remainder above 45 is below the next.
	if new(big.Int).Lsh(remainder, 1).Cmp(ninety) > 0 {
		n.Add(n, big.NewInt(1))
		remainder.Sub(remainder, ninety)
	}

	r = new(big.Float).SetInt(remainder) // a new big.Float takes the integer's length

	return r.SetMantExp(r, -int(s)), uint(n.Bit(0)) | uint(n.Bit(1))<<1
}

// sinCosSeries returns the sine and cosine of y, for |y| below 1, to prec
// bits, from their Taylor series:
//
//	sin y = y - y^3/3! + y^5/5! - ...
//	cos y = 1 - y^2/2! + y^4/4! - ...
func sinCosSeries(y *big.Float, prec uint) (sine, cosine *big.Float) {
	return factorialSeries(y, true, prec)
}

// factorialSeries sums the two series
//
//	odd  = y + s*y^3/3! + y^5/5! + s*y^7/7! + ...
//	even = 1 + s*y^2/2! + y^4/4! + s*y^6/6! + ...
//
// for |y| below 1, to prec bits, s being -1 if alternating is set and 1
// otherwise. Between them they're every power of y over its factorial, so one
// loop makes both for the price of either. Since the terms get smaller, it stops
// at the first that's too small to change the odd series, the smaller of the two
// for such a y: for a tiny y that's at once, and that sum is exactly y.
func factorialSeries(y *big.Float, alternating bool, prec uint) (odd, even *big.Float) {
	w := prec + scratchBits

	odd = newFloat(w).Set(y)
	even = newFloat(w).SetInt64(1)

	if y.Sign() == 0 {
		return odd, even
	}

	smallest := y.MantExp(nil) - int(w)
	term := newFloat(w).Set(y) // y^k / k!, as k goes up
	divisor := newFloat(w)

	for k := int64(2); ; k++ {
		term.Mul(term, y)
		term.Quo(term, divisor.SetInt64(k))

		if term.Sign() == 0 || term.MantExp(nil) < smallest {
			return odd, even
		}

		sum := even
		if k%2 == 1 {
			sum = odd
		}

		// To alternate, the signs go round every four terms: y^2 subtracted
		// from the even series, y^3 from the odd, y^4 added to the even, y^5 to
		// the odd.
		if alternating && k%4 >= 2 {
			sum.Sub(sum, term)
		} else {
			sum.Add(sum, term)
		}
	}
}

// fromQuadrant turns the sine and cosine of a reduced angle r into those of
// the angle it came from, x = |x| or -|x|, with |x| = n * pi/2 + r: adding
// pi/2 turns (sine, cosine) into (cosine, -sine). Each result is rounded to
// prec bits.
func fromQuadrant(sine, cosine *big.Float, quadrant uint, negative bool, prec uint) (*big.Float, *big.Float) {
	s, c := newFloat(prec), newFloat(prec)

	switch quadrant {
	case 0:
		s.Set(sine)
		c.Set(cosine)
	case 1:
		s.Set(cosine)
		c.Neg(sine)
	case 2:
		s.Neg(sine)
		c.Neg(cosine)
	default:
		s.Neg(cosine)
		c.Set(sine)
	}

	// A sine is odd and a cosine even: sin(-x) = -sin(x), cos(-x) = cos(x).
	if negative {
		s.Neg(s)
	}

	return s, c
}

// sinCos returns the sine and cosine of x radians, to prec bits.
func sinCos(x *big.Float, prec uint) (sine, cosine *big.Float) {
	magnitude := new(big.Float).Abs(x)
	r, quadrant := reduceRadians(magnitude, prec)
	s, c := sinCosSeries(r, prec)

	return fromQuadrant(s, c, quadrant, x.Signbit(), prec)
}

// sinCosDegrees returns the sine and cosine of x degrees, to prec bits. An
// angle's exact answers come out exact: after reduceDegrees the reduced angle
// is exactly 0 for a multiple of 90, and only the rest of the angles are
// converted to radians, approximately, to go in the series.
func sinCosDegrees(x *big.Float, prec uint) (sine, cosine *big.Float) {
	magnitude := new(big.Float).Abs(x)
	r, quadrant := reduceDegrees(magnitude)

	w := prec + scratchBits

	// r * pi / 180 = r * (pi/2) / 90.
	radians := newFloat(w).Mul(r, halfPi(w))
	radians.Quo(radians, big.NewFloat(90))

	s, c := sinCosSeries(radians, prec)

	return fromQuadrant(s, c, quadrant, x.Signbit(), prec)
}

// arcTan returns the angle in radians whose tangent is x, to prec bits, between
// -pi/2 and pi/2.
//
// The series on z converges only for |z| below 1, and slowly near 1, so first
// the argument is made small by two identities:
//
//	atan(x) = pi/2 - atan(1/x)                      for x > 0
//	atan(z) = 2 * atan(z / (1 + sqrt(1 + z^2)))
//
// The second is the half-angle formula for the tangent, and each use about
// halves z. Three uses at most bring z from 1 to below 1/8, where each term of
// the series gains at least 6 bits.
func arcTan(x *big.Float, prec uint) *big.Float {
	w := prec + scratchBits
	z := newFloat(w).Abs(x)

	if z.Sign() == 0 {
		return z
	}

	one := big.NewFloat(1)

	inverted := z.Cmp(one) > 0
	if inverted {
		z.Quo(one, z)
	}

	halvings := 0

	for z.MantExp(nil) > -3 { // that is, z >= 1/8
		root := newFloat(w).Mul(z, z)
		root.Sqrt(root.Add(root, one))
		z.Quo(z, root.Add(root, one))

		halvings++
	}

	angle := atanSeries(z, w)
	angle.SetMantExp(angle, halvings)

	if inverted {
		angle.Sub(halfPi(w), angle)
	}

	if x.Signbit() {
		angle.Neg(angle)
	}

	return newFloat(prec).Set(angle)
}

// complementRoot returns sqrt(1 - x^2), to prec bits, for |x| at most 1. It
// computes (1 - x) * (1 + x), not 1 - x * x: when x is near 1 the product x * x
// would be rounded before the subtraction took away its leading bits, while
// 1 - x is exact there.
func complementRoot(x *big.Float, prec uint) *big.Float {
	one := big.NewFloat(1)
	product := newFloat(prec).Mul(newFloat(prec).Sub(one, x), newFloat(prec).Add(one, x))

	return product.Sqrt(product)
}

// arcSin returns the angle in radians whose sine is x, to prec bits, between
// -pi/2 and pi/2, for |x| at most 1: the manual's atan(x / sqrt(1 - x^2)), or
// plus or minus pi/2 at the ends, where there's nothing to divide by.
func arcSin(x *big.Float, prec uint) *big.Float {
	w := prec + scratchBits
	root := complementRoot(x, w)

	if root.Sign() == 0 {
		angle := newFloat(prec).Set(halfPi(w))
		if x.Signbit() {
			angle.Neg(angle)
		}

		return angle
	}

	return arcTan(newFloat(w).Quo(x, root), prec)
}

// arcCos returns the angle in radians whose cosine is x, to prec bits, between
// 0 and pi, for |x| at most 1: the manual's atan(sqrt(1 - x^2) / x), plus pi
// when x is negative, since a tangent's arc is never beyond pi/2. Where x is 0
// the division can't be done and the answer is pi/2, and where x is 1 the answer
// is exactly 0.
func arcCos(x *big.Float, prec uint) *big.Float {
	w := prec + scratchBits

	if x.Sign() == 0 {
		return newFloat(prec).Set(halfPi(w))
	}

	root := complementRoot(x, w)
	angle := arcTan(newFloat(w).Quo(root, x), w)

	if x.Signbit() {
		half := halfPi(w)
		angle.Add(angle, half.SetMantExp(half, 1))
	}

	return newFloat(prec).Set(angle)
}

// arcTan2 returns the angle in radians, between -pi and pi, whose tangent is
// y/x, in the quadrant the signs of y and x say, to prec bits. y and x mustn't
// both be 0. The manual's rule:
//
//	x = 0    pi/2, with y's sign
//	x > 0    atan(y/x)
//	x < 0    pi, with y's sign, plus atan(y/x)
//
// where 0 counts as positive, a VAX value having no -0. So an angle of x < 0 and
// y = 0 is pi. When x < 0, atan(y/x) has the opposite sign to y and is at most
// pi/2 in size, so the sum never cancels more than a bit.
func arcTan2(y, x *big.Float, prec uint) *big.Float {
	w := prec + scratchBits

	if x.Sign() == 0 {
		angle := newFloat(prec).Set(halfPi(w))
		if y.Signbit() {
			angle.Neg(angle)
		}

		return angle
	}

	angle := arcTan(newFloat(w).Quo(y, x), w)

	if x.Signbit() {
		half := halfPi(w)
		half.SetMantExp(half, 1)

		if y.Signbit() {
			half.Neg(half)
		}

		angle.Add(angle, half)
	}

	return newFloat(prec).Set(angle)
}

// toDegrees returns an angle in radians in degrees, to prec bits: times 180,
// over pi.
func toDegrees(radians *big.Float, prec uint) *big.Float {
	w := prec + scratchBits
	half := halfPi(w)

	angle := newFloat(w).Mul(radians, big.NewFloat(90))

	return newFloat(prec).Quo(angle, half)
}
