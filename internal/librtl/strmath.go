package librtl

import (
	"errors"
	"math"
	"math/big"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The STR$ arithmetic routines (docs/planning/PHASE-113, subtask 7): STR$ADD,
// STR$MUL, STR$DIVIDE, STR$RECIP, and STR$ROUND, written from the VMS 5.0 and
// OpenVMS 7.3 STR$ manuals. Each number is passed in three arguments, all by
// reference but the digits: a sign (a longword, 0 positive and 1 negative), an
// exponent (a signed longword, the power of 10 the digits are multiplied by),
// and the digits (a string of unsigned decimal digits, by descriptor). So the
// manual's example -1000 is sign 1, exponent 3, digits "1", and .0002 is sign 0,
// exponent -4, digits "2". A result comes back the same way, its digits written
// to the destination by its class as every STR$ routine writes (STR$_TRU when
// they don't fit).
//
// The arithmetic is exact, in math/big integers: a number is its digits as an
// integer and its exponent, so adding aligns the two on the smaller exponent and
// multiplying adds the exponents. Only DIVIDE, RECIP, and ROUND give up digits,
// each by its own rule (below). What the manuals don't settle is govax's own
// choice, unconfirmed until testdata/str113's probe runs on VMS (subtask 12):
//
//   - a result's digits have no leading zeros; zero is "0" with exponent 0 and
//     sign 0, whatever signs and exponents made it;
//   - digits that aren't all decimal digits, and a sign other than 0 or 1, are
//     LIB$_INVARG (the manuals list it, without saying for what); an empty digit
//     string is zero;
//   - a result whose exponent doesn't fit in a longword is LIB$_INVARG, and one
//     with more than 65,535 digits STR$_STRTOOLON, as the other STR$ routines'
//     results that long are;
//   - too few arguments is STR$_WRONUMARG, which the manuals list.

var (
	strDivByZer = vmsdef.LibrarySymbols["STR$_DIVBY_ZER"]

	// errBadDecimal is a sign or a digit string the routines can't read.
	errBadDecimal = errors.New("not a decimal number")

	// errExponentRange is a result whose exponent isn't a longword.
	errExponentRange = errors.New("exponent out of range")

	// errTooManyDigits is a result longer than any string.
	errTooManyDigits = errors.New("too many digits")

	ten = big.NewInt(10)
)

// decimal is one number: digits times 10 to the power exp, negative if neg.
type decimal struct {
	neg    bool
	exp    int64
	digits *big.Int // never negative
}

// zero reports whether d is zero.
func (d decimal) zero() bool { return d.digits.Sign() == 0 }

// normal is d as the routines return it: zero is "0", exponent 0, sign 0.
func (d decimal) normal() decimal {
	if d.zero() {
		return decimal{digits: new(big.Int)}
	}

	return d
}

// powerOfTen is 10 to the power n, n not negative.
func powerOfTen(n int64) *big.Int {
	return new(big.Int).Exp(ten, big.NewInt(n), nil)
}

// decimalLength is how many digits x has, 1 for zero.
func decimalLength(x *big.Int) int64 {
	if x.Sign() == 0 {
		return 1
	}

	return int64(len(x.String()))
}

// decimalDigits reads a digit string: decimal digits only, an empty string zero.
func decimalDigits(s string) (*big.Int, bool) {
	if strings.Trim(s, "0123456789") != "" {
		return nil, false
	}

	if s == "" {
		return new(big.Int), true
	}

	x, ok := new(big.Int).SetString(s, 10)

	return x, ok
}

// readDecimal reads one number from its three arguments, sign first, as the
// manuals list them.
func readDecimal(env *corevms.Environment, signAddr, expAddr, digitsAddr uint32) (decimal, error) {
	sign, err := env.Memory().LoadLongword(env.CPU(), signAddr)
	if err != nil {
		return decimal{}, err
	}

	exp, err := signedAt(env, expAddr)
	if err != nil {
		return decimal{}, err
	}

	s, _, err := strSource(env, digitsAddr)
	if err != nil {
		return decimal{}, err
	}

	digits, ok := decimalDigits(s)
	if !ok || sign > 1 {
		return decimal{}, errBadDecimal
	}

	return decimal{neg: sign == 1, exp: int64(exp), digits: digits}, nil
}

// arithmeticFailure is strFailure for the arithmetic routines' own errors too.
func arithmeticFailure(env *corevms.Environment, dst uint32, err error) (uint32, error) {
	switch {
	case errors.Is(err, errBadDecimal), errors.Is(err, errExponentRange):
		return env.SignalFailure([]uint32{libInvArg}, libInvArg)
	case errors.Is(err, errTooManyDigits):
		return strTooLong(env, dst)
	default:
		return strFailure(env, err)
	}
}

// storeDecimal writes a result to its three arguments: the digits first (so a
// destination of no string class changes nothing), then the exponent and the
// sign. The status is the digits' store's, SS$_NORMAL or STR$_TRU.
func storeDecimal(env *corevms.Environment, signAddr, expAddr, digitsAddr uint32, d decimal) (uint32, error) {
	d = d.normal()

	if d.exp < math.MinInt32 || d.exp > math.MaxInt32 {
		return arithmeticFailure(env, digitsAddr, errExponentRange)
	}

	text := d.digits.String()
	if len(text) > maxStringLength {
		return arithmeticFailure(env, digitsAddr, errTooManyDigits)
	}

	status, err := strPut(env, digitsAddr, text)
	if err != nil {
		return status, err
	}

	sign := uint32(0)
	if d.neg {
		sign = 1
	}

	mem, cpu := env.Memory(), env.CPU()
	if err := mem.StoreLongword(cpu, expAddr, uint32(int32(d.exp))); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, signAddr, sign); err != nil {
		return memoryFault(env, writing(err))
	}

	return status, nil
}

// addDecimals is a + b: both on the smaller exponent, then a signed sum.
func addDecimals(a, b decimal) (decimal, error) {
	switch {
	case a.zero():
		return b, nil
	case b.zero():
		return a, nil
	}

	exp := min(a.exp, b.exp)

	// The larger exponent's digits grow by the difference; past any string's
	// length, the sum can't be written (and isn't worth computing).
	shiftA, shiftB := a.exp-exp, b.exp-exp
	if shiftA+decimalLength(a.digits) > maxStringLength+1 || shiftB+decimalLength(b.digits) > maxStringLength+1 {
		return decimal{}, errTooManyDigits
	}

	x := new(big.Int).Mul(a.digits, powerOfTen(shiftA))
	y := new(big.Int).Mul(b.digits, powerOfTen(shiftB))

	if a.neg {
		x.Neg(x)
	}

	if b.neg {
		y.Neg(y)
	}

	sum := x.Add(x, y)
	neg := sum.Sign() < 0

	return decimal{neg: neg, exp: exp, digits: sum.Abs(sum)}, nil
}

// mulDecimals is a times b: the digits' product, the exponents' sum.
func mulDecimals(a, b decimal) decimal {
	return decimal{
		neg:    a.neg != b.neg,
		exp:    a.exp + b.exp,
		digits: new(big.Int).Mul(a.digits, b.digits),
	}
}

// divideDecimals is a over b to places digits after the decimal point (the
// result's exponent -places), rounded half away from zero if round, else
// truncated. b is not zero.
func divideDecimals(a, b decimal, places int64, round bool) (decimal, error) {
	exp := -places

	// The quotient is a.digits * 10^k / b.digits.
	k := a.exp - b.exp + places
	lenA, lenB := decimalLength(a.digits), decimalLength(b.digits)

	// The quotient is below 10^(lenA-lenB+1+k): past any string when that's
	// too many digits, and below .1, so 0 either way, when that's -1 or less.
	if lenA-lenB+1+k > maxStringLength+1 {
		return decimal{}, errTooManyDigits
	}

	if a.zero() || lenA-lenB+1+k <= -1 {
		return decimal{exp: exp, digits: new(big.Int)}, nil
	}

	num := new(big.Int).Set(a.digits)
	den := new(big.Int).Set(b.digits)

	if k >= 0 {
		num.Mul(num, powerOfTen(k))
	} else {
		den.Mul(den, powerOfTen(-k))
	}

	q, r := new(big.Int).QuoRem(num, den, new(big.Int))
	if round && r.Sign() != 0 && r.Lsh(r, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}

	return decimal{neg: a.neg != b.neg, exp: exp, digits: q}, nil
}

// roundDecimal keeps a's first places significant digits, rounding the rest
// away (half away from zero) or truncating them. A carry that makes one more
// digit (999 to 1000) drops the last zero into the exponent, so the result
// never has more than places digits: the manual's 999.9998 to 3 is 100 times
// 10, rounded, and 999, truncated.
func roundDecimal(a decimal, places int64, truncate bool) decimal {
	s := a.digits.String()
	if int64(len(s)) <= places || a.zero() {
		return a
	}

	drop := int64(len(s)) - places
	kept, _ := new(big.Int).SetString(s[:places], 10)
	exp := a.exp + drop

	if !truncate && s[places] >= '5' {
		kept.Add(kept, big.NewInt(1))

		if decimalLength(kept) > places {
			kept.Quo(kept, ten)
			exp++
		}
	}

	return decimal{neg: a.neg, exp: exp, digits: kept}
}

// trimZeros takes a result's trailing zeros into its exponent: RECIP's result,
// as the manual's example shows it (1 / 1000 to the fourth place is "1" times
// 10^-3, not "10" times 10^-4).
func trimZeros(d decimal) decimal {
	if d.zero() {
		return d
	}

	q, r := new(big.Int), new(big.Int)
	for {
		q.QuoRem(d.digits, ten, r)
		if r.Sign() != 0 {
			return d
		}

		d.digits = new(big.Int).Set(q)
		d.exp++
	}
}

// twoOperands reads a and b from the first six arguments.
func twoOperands(env *corevms.Environment, argv []uint32) (decimal, decimal, error) {
	a, err := readDecimal(env, argv[0], argv[1], argv[2])
	if err != nil {
		return decimal{}, decimal{}, err
	}

	b, err := readDecimal(env, argv[3], argv[4], argv[5])

	return a, b, err
}

// binaryArithmetic is ADD's and MUL's shape: a, b, then c, nine arguments.
func binaryArithmetic(env *corevms.Environment, argv []uint32, name string, op func(a, b decimal) (decimal, error)) (uint32, error) {
	if len(argv) < 9 {
		return wrongArgCount(env, argv, name)
	}

	a, b, err := twoOperands(env, argv)
	if err != nil {
		return arithmeticFailure(env, argv[8], err)
	}

	c, err := op(a, b)
	if err != nil {
		return arithmeticFailure(env, argv[8], err)
	}

	return storeDecimal(env, argv[6], argv[7], argv[8], c)
}

// strAdd is STR$ADD: asign, aexp, adigits, bsign, bexp, bdigits, csign, cexp,
// cdigits.
func strAdd(env *corevms.Environment, argv []uint32) (uint32, error) {
	return binaryArithmetic(env, argv, "STR$ADD", addDecimals)
}

// strMul is STR$MUL: the same arguments as STR$ADD.
func strMul(env *corevms.Environment, argv []uint32) (uint32, error) {
	return binaryArithmetic(env, argv, "STR$MUL", func(a, b decimal) (decimal, error) {
		if decimalLength(a.digits)+decimalLength(b.digits) > maxStringLength+1 {
			return decimal{}, errTooManyDigits
		}

		return mulDecimals(a, b), nil
	})
}

// strDivide is STR$DIVIDE: asign, aexp, adigits, bsign, bexp, bdigits,
// total-digits (the digits after the decimal point), round-truncate-indicator
// (bit 0: 1 rounds, 0 truncates), csign, cexp, cdigits. The quotient's
// exponent is -total-digits, its digits as many as that takes (the manual's
// -1000 / .0002 to 3 is 5000000000 times 10^-3). A zero divisor is
// STR$_DIVBY_ZER.
func strDivide(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 11 {
		return wrongArgCount(env, argv, "STR$DIVIDE")
	}

	a, b, err := twoOperands(env, argv)
	if err != nil {
		return arithmeticFailure(env, argv[10], err)
	}

	places, err := signedAt(env, argv[6])
	if err != nil {
		return strFailure(env, err)
	}

	indicator, err := env.Memory().LoadLongword(env.CPU(), argv[7])
	if err != nil {
		return strFailure(env, err)
	}

	if b.zero() {
		return env.SignalFailure([]uint32{strDivByZer}, strDivByZer)
	}

	c, err := divideDecimals(a, b, int64(places), indicator&1 != 0)
	if err != nil {
		return arithmeticFailure(env, argv[10], err)
	}

	return storeDecimal(env, argv[8], argv[9], argv[10], c)
}

// strRecip is STR$RECIP: asign, aexp, adigits, then b, whose exponent is the
// precision (1 / a to the place b's last digit is in), then c. The quotient is
// truncated there and its trailing zeros taken into the exponent, so the
// manual's 1 / -1000 to .0002's place is -1 times 10^-3. Only b's exponent is
// used (unconfirmed, as is truncating). A zero a is STR$_DIVBY_ZER.
func strRecip(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 9 {
		return wrongArgCount(env, argv, "STR$RECIP")
	}

	a, b, err := twoOperands(env, argv)
	if err != nil {
		return arithmeticFailure(env, argv[8], err)
	}

	if a.zero() {
		return env.SignalFailure([]uint32{strDivByZer}, strDivByZer)
	}

	one := decimal{digits: big.NewInt(1)}

	c, err := divideDecimals(one, a, -b.exp, false)
	if err != nil {
		return arithmeticFailure(env, argv[8], err)
	}

	return storeDecimal(env, argv[6], argv[7], argv[8], trimZeros(c))
}

// strRound is STR$ROUND: places (significant digits kept), flags (bit 0: 1
// truncates, 0 rounds), asign, aexp, adigits, csign, cexp, cdigits. Places
// below 1 is LIB$_INVARG (unconfirmed).
func strRound(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 8 {
		return wrongArgCount(env, argv, "STR$ROUND")
	}

	places, err := signedAt(env, argv[0])
	if err != nil {
		return strFailure(env, err)
	}

	flags, err := env.Memory().LoadLongword(env.CPU(), argv[1])
	if err != nil {
		return strFailure(env, err)
	}

	a, err := readDecimal(env, argv[2], argv[3], argv[4])
	if err != nil {
		return arithmeticFailure(env, argv[7], err)
	}

	if places < 1 {
		return arithmeticFailure(env, argv[7], errBadDecimal)
	}

	return storeDecimal(env, argv[5], argv[6], argv[7], roundDecimal(a, int64(places), flags&1 != 0))
}
