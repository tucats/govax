package librtl

import (
	"errors"
	"math/big"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The STR$ arithmetic routines (docs/planning/PHASE-113, subtasks 7 and 13):
// STR$ADD, STR$MUL, STR$DIVIDE, STR$RECIP, and STR$ROUND, written from the VMS
// 5.0 and OpenVMS 7.3 STR$ manuals and made VMS 7.1's by testdata/str113's
// probe. Each number is passed in three arguments, all by reference but the
// digits: a sign (a longword, 0 positive and 1 negative), an exponent (a signed
// longword, the power of 10 the digits are multiplied by), and the digits (a
// string of unsigned decimal digits, by descriptor). So the manual's example
// -1000 is sign 1, exponent 3, digits "1", and .0002 is sign 0, exponent -4,
// digits "2". A result comes back the same way.
//
// The arithmetic is exact, in math/big integers: a number is its digits as an
// integer and its exponent, so adding aligns the two on the smaller exponent and
// multiplying adds the exponents. What VMS 7.1 showed (the probe's log):
//
//   - only bit 0 of a sign counts: 2 is positive, 3 and -1 negative;
//   - the digits are read up to the first character that isn't a digit, so
//     " 12" is 0, "12A" and "12 " are 12, and "1.2" is 1; nothing is refused;
//   - ADD's, MUL's, and RECIP's results lose their leading zeros, and their
//     trailing zeros go into the exponent (999 + 1 is 1 times 10^3); ADD's and
//     RECIP's zero is "0" with sign and exponent 0, MUL's keeps the product's
//     sign and the exponents' sum;
//   - an exponent wraps as a longword does, without a condition;
//   - DIVIDE's and ROUND's results are as their rules (below) leave them;
//   - a fixed-length destination shorter result is padded with zeros on the
//     left ("006"), a longer one truncated on the right with STR$_TRU;
//     a destination of no string class is LIB$_INVARG, nothing written;
//   - too few arguments is STR$_WRONUMARG, but DIVIDE's LIB$_INVARG.
//
// A result with more than 65,535 digits is STR$_STRTOOLON, as the other STR$
// routines' results that long are (unconfirmed).

var (
	strDivByZer = vmsdef.LibrarySymbols["STR$_DIVBY_ZER"]

	// errTooManyDigits is a result longer than any string.
	errTooManyDigits = errors.New("too many digits")

	// errNoPrecision is RECIP's precision limit of zero, which VMS 7.1's
	// STR$RECIP never returns from.
	errNoPrecision = errors.New("a precision limit of zero")

	ten = big.NewInt(10)
)

// decimal is one number: digits times 10 to the power exp, negative if neg.
// text is the digits as they were read or are to be written (ROUND keeps an
// operand's leading zeros); digits is their value.
type decimal struct {
	neg    bool
	exp    int64
	digits *big.Int // never negative
	text   string
}

// zero reports whether d is zero.
func (d decimal) zero() bool { return d.digits.Sign() == 0 }

// made is a result of value x, its text x's digits.
func made(neg bool, exp int64, x *big.Int) decimal {
	return decimal{neg: neg, exp: exp, digits: x, text: x.String()}
}

// normal is d as ADD, MUL, and RECIP return it: no leading zeros, its trailing
// zeros moved into the exponent. A zero is left as it is.
func (d decimal) normal() decimal {
	if d.zero() {
		return made(d.neg, d.exp, new(big.Int))
	}

	x, q, r := new(big.Int).Set(d.digits), new(big.Int), new(big.Int)
	exp := d.exp

	for {
		q.QuoRem(x, ten, r)
		if r.Sign() != 0 {
			return made(d.neg, exp, x)
		}

		x.Set(q)
		exp++
	}
}

// plainZero is the zero ADD and RECIP return: sign and exponent 0.
func plainZero() decimal { return made(false, 0, new(big.Int)) }

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

// decimalDigits reads a digit string as VMS's routines do: the digits before
// the first character that isn't one, an empty string or none zero. It returns
// the digits read and their value.
func decimalDigits(s string) (string, *big.Int) {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}

	s = s[:n]
	if s == "" {
		return s, new(big.Int)
	}

	x, _ := new(big.Int).SetString(s, 10)

	return s, x
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

	text, digits := decimalDigits(s)

	return decimal{neg: sign&1 != 0, exp: int64(exp), digits: digits, text: text}, nil
}

// arithmeticFailure is strFailure for the arithmetic routines' own errors too.
func arithmeticFailure(env *corevms.Environment, dst uint32, err error) (uint32, error) {
	switch {
	case errors.Is(err, errNoPrecision):
		return env.SignalFailure([]uint32{libInvArg}, libInvArg)
	case errors.Is(err, errTooManyDigits):
		return strTooLong(env, dst)
	default:
		return strFailure(env, err)
	}
}

// storeSignExponent writes a result's sign and exponent, the exponent wrapped
// to a longword.
func storeSignExponent(env *corevms.Environment, signAddr, expAddr uint32, neg bool, exp int64) error {
	sign := uint32(0)
	if neg {
		sign = 1
	}

	mem, cpu := env.Memory(), env.CPU()
	if err := mem.StoreLongword(cpu, expAddr, uint32(exp)); err != nil {
		return writing(err)
	}

	if err := mem.StoreLongword(cpu, signAddr, sign); err != nil {
		return writing(err)
	}

	return nil
}

// storeDecimal writes a result to its three arguments: the digits first (so a
// destination of no string class changes nothing), then the exponent and the
// sign. The status is the digits' store's, SS$_NORMAL or STR$_TRU.
func storeDecimal(env *corevms.Environment, signAddr, expAddr, digitsAddr uint32, d decimal) (uint32, error) {
	desc, err := readDescriptor(env, digitsAddr)
	if errors.Is(err, errInvStrDes) {
		return env.SignalFailure([]uint32{libInvArg}, libInvArg)
	} else if err != nil {
		return strFailure(env, err)
	}

	text := d.text
	if len(text) > maxStringLength {
		return arithmeticFailure(env, digitsAddr, errTooManyDigits)
	}

	// A fixed-length destination takes a shorter result right-justified, with
	// zeros before it.
	if desc.class != dscClassD && desc.class != dscClassVS && uint32(len(text)) < desc.length {
		text = strings.Repeat("0", int(desc.length)-len(text)) + text
	}

	status, err := strPut(env, digitsAddr, text)
	if err != nil {
		return status, err
	}

	if err := storeSignExponent(env, signAddr, expAddr, d.neg, d.exp); err != nil {
		return memoryFault(env, err)
	}

	return status, nil
}

// addDecimals is a + b: both on the smaller exponent, then a signed sum. A zero
// sum is a plain zero.
func addDecimals(a, b decimal) (decimal, error) {
	switch {
	case a.zero() && b.zero():
		return plainZero(), nil
	case a.zero():
		return b.normal(), nil
	case b.zero():
		return a.normal(), nil
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
	if sum.Sign() == 0 {
		return plainZero(), nil
	}

	neg := sum.Sign() < 0

	return made(neg, exp, sum.Abs(sum)).normal(), nil
}

// mulDecimals is a times b: the digits' product, the exponents' sum. A zero
// product keeps both (VMS's -3 times 0 is "0" with sign 1).
func mulDecimals(a, b decimal) decimal {
	return made(a.neg != b.neg, a.exp+b.exp, new(big.Int).Mul(a.digits, b.digits)).normal()
}

// divideDecimals is a over b, b not zero, by VMS 7.1's STR$DIVIDE: the
// quotient truncated to places+indicator digits after the decimal point, then,
// for an indicator of 1, rounded (half up) to places; its exponent is -places
// whatever the indicator. So the indicator's 0 truncates and 1 rounds, as the
// manual says, and VMS's 2 and 3 keep 2 and 3 more digits, and -1 one fewer,
// at the same exponent (2/3 to 4 places with 2 is 666666 times 10^-4).
// Leading zeros go, but a zero dividend's quotient is places+indicator zeros
// (VMS's 0/5 to 2 places is "00"; unconfirmed beyond that case) with sign 0.
func divideDecimals(a, b decimal, places, indicator int64) (decimal, error) {
	exp := -places
	digits := places + indicator

	if a.zero() {
		return decimal{exp: exp, digits: new(big.Int), text: strings.Repeat("0", int(max(1, min(digits, maxStringLength))))}, nil
	}

	// The quotient is a.digits * 10^k / b.digits.
	k := a.exp - b.exp + digits
	lenA, lenB := decimalLength(a.digits), decimalLength(b.digits)

	// The quotient is below 10^(lenA-lenB+1+k): past any string when that's
	// too many digits, and 0 when that's 0 or less.
	if lenA-lenB+1+k > maxStringLength+2 {
		return decimal{}, errTooManyDigits
	}

	neg := a.neg != b.neg

	if lenA-lenB+1+k <= 0 {
		return made(neg, exp, new(big.Int)), nil
	}

	num := new(big.Int).Set(a.digits)
	den := new(big.Int).Set(b.digits)

	if k >= 0 {
		num.Mul(num, powerOfTen(k))
	} else {
		den.Mul(den, powerOfTen(-k))
	}

	q := new(big.Int).Quo(num, den)
	if indicator == 1 {
		q.Add(q, big.NewInt(5))
		q.Quo(q, ten)
	}

	return made(neg, exp, q), nil
}

// recipDecimal is 1 / a, a not zero, to b's precision, by VMS 7.1's STR$RECIP:
// the places p start at -b's exponent and grow until 10^-p is below b, so b's
// digits of 1 give one more place than b's exponent says and any other digits
// none (1e-4 gives 5 places, 9e-4 and 25e-4 give 4). The quotient is
// truncated there and returned as ADD's are. Digits of 0 never satisfy the
// rule: VMS loops for ever, govax gives LIB$_INVARG.
func recipDecimal(a, b decimal) (decimal, error) {
	if b.zero() {
		return decimal{}, errNoPrecision
	}

	p := -b.exp
	if b.digits.Cmp(big.NewInt(1)) == 0 {
		p++
	}

	q, err := divideDecimals(made(false, 0, big.NewInt(1)), a, p, 0)
	if err != nil {
		return decimal{}, err
	}

	if q.zero() {
		return plainZero(), nil
	}

	return q.normal(), nil
}

// roundDecimal keeps a's first places digits as written, leading zeros and all
// (VMS's 0099 to one place is "0" times 10^3), rounding the rest away (half up)
// or truncating them. A carry that makes one more digit (999 to 1000) drops the
// last zero into the exponent, so the result never has more than places digits:
// the manual's 999.9998 to 3 is 100 times 10, rounded, and 999, truncated. A
// number with no more digits than places is returned as it is, and places
// below 1 is a plain zero (VMS 7.1's for 0; its -1 is a string of 65,535
// characters read from beyond the digits, which govax doesn't copy).
func roundDecimal(a decimal, places int64, truncate bool) decimal {
	if places < 1 {
		return plainZero()
	}

	s := a.text
	if int64(len(s)) <= places {
		return a
	}

	kept := []byte(s[:places])
	exp := a.exp + int64(len(s)) - places

	if !truncate && s[places] >= '5' {
		i := len(kept) - 1
		for ; i >= 0 && kept[i] == '9'; i-- {
			kept[i] = '0'
		}

		if i >= 0 {
			kept[i]++
		} else {
			// Every digit carried: 99 becomes 100, kept as 10 with one more
			// in the exponent.
			kept = append([]byte{'1'}, kept[:len(kept)-1]...)
			exp++
		}
	}

	_, x := decimalDigits(string(kept))

	return decimal{neg: a.neg, exp: exp, digits: x, text: string(kept)}
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
// (0 truncates, 1 rounds; divideDecimals says what the others do), csign,
// cexp, cdigits. The quotient's exponent is -total-digits, its digits as many
// as that takes (the manual's -1000 / .0002 to 3 is 5000000000 times 10^-3).
// Too few arguments is LIB$_INVARG, and a zero divisor STR$_DIVBY_ZER after
// the result's sign and exponent are written (VMS 7.1's).
func strDivide(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 11 {
		return env.SignalFailure([]uint32{libInvArg}, libInvArg)
	}

	a, b, err := twoOperands(env, argv)
	if err != nil {
		return arithmeticFailure(env, argv[10], err)
	}

	places, err := signedAt(env, argv[6])
	if err != nil {
		return strFailure(env, err)
	}

	indicator, err := signedAt(env, argv[7])
	if err != nil {
		return strFailure(env, err)
	}

	if b.zero() {
		if err := storeSignExponent(env, argv[8], argv[9], a.neg != b.neg, -int64(places)); err != nil {
			return memoryFault(env, err)
		}

		return env.SignalFailure([]uint32{strDivByZer}, strDivByZer)
	}

	c, err := divideDecimals(a, b, int64(places), int64(indicator))
	if err != nil {
		return arithmeticFailure(env, argv[10], err)
	}

	return storeDecimal(env, argv[8], argv[9], argv[10], c)
}

// strRecip is STR$RECIP: asign, aexp, adigits, then b, the precision limit
// (recipDecimal), then c. A zero a is STR$_DIVBY_ZER.
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

	c, err := recipDecimal(a, b)
	if err != nil {
		return arithmeticFailure(env, argv[8], err)
	}

	return storeDecimal(env, argv[6], argv[7], argv[8], c)
}

// strRound is STR$ROUND: places (significant digits kept), flags (bit 0: 1
// truncates, 0 rounds), asign, aexp, adigits, csign, cexp, cdigits.
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

	return storeDecimal(env, argv[5], argv[6], argv[7], roundDecimal(a, int64(places), flags&1 != 0))
}
