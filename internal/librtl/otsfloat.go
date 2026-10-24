package librtl

import (
	"errors"
	"math/big"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The OTS$ floating conversions (Phase 72, subtask 6), from the VMS 5.0
// RTL General Purpose (OTS$) manual, OTS-3 and OTS-29 to OTS-35: the
// routines BASIC's and FORTRAN's formatted I/O, and other languages',
// call to turn a floating value into text and text into one. The values
// are exact rationals (vaxfloat's), so every rounding is done once, on
// the true value.
//
// To text, FORTRAN's E format:
//
//	OTS$CNVOUT   value.rd.r ,out-str.wt.ds ,digits-in-fraction.rlu.v
//	OTS$CNVOUT_G value.rg.r ,...
//	OTS$CNVOUT_H value.rh.r ,...
//
// The text is a minus sign for a negative value, "0.", digits-in-fraction
// digits (the value rounded to them, half away from zero), and the
// exponent, right-justified in the fixed-length string with blanks
// before it. The exponent is E, its sign, and two digits; one of three
// digits drops the E (FORTRAN's rule, 0.1234+100). A result too long for
// the string fills it with asterisks and returns OTS$_OUTCONERR. A
// reserved operand returns SS$_ROPRAND, the string unchanged.
//
// From text, by the input string's descriptor (any class):
//
//	OTS$CVT_T_D in-str.rt.dx ,value.wd.r [,digits-in-fraction.rlu.v]
//	            [,scale-factor.rl.v] [,flags.rlu.v] [,extension-bits.wb.r]
//
// and _F, _G (extension bits a word), _H (a word). The syntax is blanks,
// a sign, digits with an optional period, then an optional exponent: a
// letter (E, D, or Q, either case), blanks, a sign, and digits. Without
// a period, the last digits-in-fraction digits are the fraction. Without
// an exponent (or with flags bit 6), the value is divided by
// 10^scale-factor. The flags: bit 0 ignores blanks after the first
// character that isn't one (otherwise they're zeros), bit 1 allows only
// E as the exponent letter, bit 2 makes an underflow an error, bit 3
// truncates instead of rounding, bit 4 ignores tabs (otherwise
// invalid), bit 5 requires the exponent's letter (otherwise a sign may
// start an exponent: 1234567+23). With extension-bits, the value is
// truncated and the next bits of it are stored there, as EMOD's
// extension operand takes them: 8 for F and D, 11 for G and 15 for H,
// left-justified in the word. An invalid character or a value out of
// the format's range is OTS$_INPCONERR, with +0.0 stored.
//
// Unconfirmed (the manual doesn't say; testdata/ots72 asks VMS): the
// exponent of an H value past 999 (asterisks here: it overflows the
// two-digit field by more than one digit); a result that fits only
// without its leading zero (written without it, as FORTRAN's E editing
// allows); digits-in-fraction 0 ("0.E+01"); digits beyond the format's
// precision (the true value's, rounded); a string of blanks, a sign, a
// period, or a letter with no digits (all 0); blanks between the
// exponent's letter and its sign (passed over); what CNVOUT returns in
// R0 on success (SS$_NORMAL).

// The flags of the text-to-floating routines.
const (
	cvtFlagIgnoreBlanks = 1 << 0 // blanks ignored, not zeros
	cvtFlagOnlyE        = 1 << 1 // E or e the only exponent letter
	cvtFlagUnderflowErr = 1 << 2 // an underflow is an error
	cvtFlagTruncate     = 1 << 3 // truncate, not round
	cvtFlagIgnoreTabs   = 1 << 4 // tabs ignored, not invalid
	cvtFlagNeedLetter   = 1 << 5 // an exponent must start with its letter
	cvtFlagAlwaysScale  = 1 << 6 // the scale factor applied with an exponent too
)

// maxDecimalExponent bounds the exponents the text routines compute
// with: past it, a nonzero value is out of every format's range (H's is
// about 10^±4932).
const maxDecimalExponent = 20000

// extensionBits is how many bits after a truncated value each format's
// extension-bits argument gets, and the size of that argument in bytes.
var extensionBits = map[vaxfloat.Format]struct{ bits, size int }{
	vaxfloat.F: {8, 1}, vaxfloat.D: {8, 1}, vaxfloat.G: {11, 2}, vaxfloat.H: {15, 2},
}

func otsCnvout(env *corevms.Environment, argv []uint32) (uint32, error) {
	return cnvout(env, argv, vaxfloat.D)
}

func otsCnvoutG(env *corevms.Environment, argv []uint32) (uint32, error) {
	return cnvout(env, argv, vaxfloat.G)
}

func otsCnvoutH(env *corevms.Environment, argv []uint32) (uint32, error) {
	return cnvout(env, argv, vaxfloat.H)
}

// cnvout is the CNVOUT routines' work, on a value of format f.
func cnvout(env *corevms.Environment, argv []uint32, f vaxfloat.Format) (uint32, error) {
	v, err := loadFloat(env, f, arg(argv, 0))
	if errors.Is(err, vaxfloat.ErrReserved) {
		return ssRopRand, nil
	} else if err != nil {
		return memoryFault(env, err)
	}

	d, err := readDescriptor(env, arg(argv, 1))
	if err != nil {
		return otsStringFailure(env, err)
	}

	width := int(d.length)
	digits := arg(argv, 2)

	// "0." and the digits alone must fit.
	if uint64(digits)+1 > uint64(width) {
		return storeRightJustified(env, arg(argv, 1), strings.Repeat("*", width+1))
	}

	text, ok := exponentText(v.Rat(), int(digits))
	if !ok {
		text = strings.Repeat("*", width+1)
	} else if len(text) > width {
		// FORTRAN's E editing may leave out the zero before the point.
		text = strings.Replace(text, "0.", ".", 1)
	}

	return storeRightJustified(env, arg(argv, 1), text)
}

// exponentText is v in FORTRAN's E format with digits digits after the
// point: [-]0.ddd, then E+nn, or +nnn when the exponent needs three
// digits. ok is false for an exponent of four digits or more.
func exponentText(v *big.Rat, digits int) (text string, ok bool) {
	a := new(big.Rat).Abs(v)
	exp := 0

	if a.Sign() != 0 {
		exp = decimalExponent(a)
	}

	// The digits: a / 10^exp is in [0.1, 1), times 10^digits rounded.
	m := roundRat(new(big.Rat).Mul(a, pow10(digits-exp)))
	if m.Cmp(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)) >= 0 {
		m.Quo(m, big.NewInt(10))
		exp++
	}

	if a.Sign() == 0 {
		exp = 0
	}

	frac := ""
	if digits > 0 {
		frac = leftPad(m.String(), digits)
	}

	esign := "+"
	if exp < 0 {
		esign = "-"
	}

	e := leftPad(big.NewInt(int64(abs(exp))).String(), 2)

	switch {
	case len(e) == 2:
		e = "E" + esign + e
	case len(e) == 3:
		e = esign + e
	default:
		return "", false
	}

	sign := ""
	if v.Sign() < 0 {
		sign = "-"
	}

	return sign + "0." + frac + e, true
}

// decimalExponent is the e with 0.1 <= a / 10^e < 1, for a positive a:
// first a guess from a's binary exponent, then corrected.
func decimalExponent(a *big.Rat) int {
	bf := new(big.Float).SetPrec(64).SetRat(a)
	exp := int(float64(bf.MantExp(nil)) * 0.30102999566398) // log10(2)

	for a.Cmp(pow10(exp)) >= 0 {
		exp++
	}

	for a.Cmp(pow10(exp-1)) < 0 {
		exp--
	}

	return exp
}

func otsCvtTD(env *corevms.Environment, argv []uint32) (uint32, error) {
	return textToFloat(env, argv, vaxfloat.D)
}

func otsCvtTF(env *corevms.Environment, argv []uint32) (uint32, error) {
	return textToFloat(env, argv, vaxfloat.F)
}

func otsCvtTG(env *corevms.Environment, argv []uint32) (uint32, error) {
	return textToFloat(env, argv, vaxfloat.G)
}

func otsCvtTH(env *corevms.Environment, argv []uint32) (uint32, error) {
	return textToFloat(env, argv, vaxfloat.H)
}

// textToFloat is the CVT_T_z routines' work, into format f.
func textToFloat(env *corevms.Environment, argv []uint32, f vaxfloat.Format) (uint32, error) {
	text, err := readString(env, arg(argv, 0))
	if err != nil {
		return otsStringFailure(env, err)
	}

	flags := arg(argv, 4)
	extAddr := arg(argv, 5)

	value, ok := parseFloatText(text, int(arg(argv, 2)), int(int32(arg(argv, 3))), flags)
	if !ok {
		return storeFloatResult(env, f, arg(argv, 1), vaxfloat.Value{}, otsInpConErr)
	}

	var (
		v   vaxfloat.Value
		ext uint64
	)

	switch {
	case value == nil:
		err = vaxfloat.ErrOverflow
	case value.Sign() == 0:
	case extAddr != 0 || flags&cvtFlagTruncate != 0:
		v, ext, err = truncateToFormat(f, value, extensionBits[f].bits)
	default:
		v, err = vaxfloat.Round(f, vaxfloat.FromRat(value))
	}

	status := ssNormal

	switch {
	case errors.Is(err, vaxfloat.ErrOverflow):
		v, ext, status = vaxfloat.Value{}, 0, otsInpConErr
	case errors.Is(err, vaxfloat.ErrUnderflow):
		v, ext = vaxfloat.Value{}, 0
		if flags&cvtFlagUnderflowErr != 0 {
			status = otsInpConErr
		}
	}

	if extAddr != 0 {
		e := extensionBits[f]
		left := uint64(8*e.size - e.bits) // the bits are left-justified

		if err := storeSized(env, extAddr, new(big.Int).SetUint64(ext<<left), e.size); err != nil {
			return memoryFault(env, err)
		}
	}

	return storeFloatResult(env, f, arg(argv, 1), v, status)
}

// storeFloatResult stores v at addr and returns status.
func storeFloatResult(env *corevms.Environment, f vaxfloat.Format, addr uint32, v vaxfloat.Value, status uint32) (uint32, error) {
	if err := storeFloat(env, f, addr, v); err != nil {
		return memoryFault(env, err)
	}

	return status, nil
}

// parseFloatText reads text by the CVT_T_z rules (see this file's
// opening comment). ok is false for invalid text. The value is nil, with
// ok true, for one so large no format holds it; one so small is zero
// with a negative exponent kept for the caller's underflow test, so a
// tiny value comes back as a tiny rational.
func parseFloatText(text string, fraction, scale int, flags uint32) (value *big.Rat, ok bool) {
	p := floatScanner{text: text, flags: flags}

	p.skipLeading()

	negative := p.sign()

	digits, point, ok := p.mantissa()
	if !ok {
		return nil, false
	}

	exp, hasExp, ok := p.exponent()
	if !ok || !p.done() {
		return nil, false
	}

	n := new(big.Int)
	if digits != "" {
		n.SetString(digits, 10)
	}

	// The power of ten the digits are scaled by.
	power := 0
	if point < 0 {
		power -= fraction
	} else {
		power -= len(digits) - point
	}

	if hasExp {
		power += exp
	}

	if !hasExp || flags&cvtFlagAlwaysScale != 0 {
		power -= scale
	}

	if n.Sign() == 0 {
		return new(big.Rat), true
	}

	// An exponent far past any format's range: too big, or too small.
	if top := power + len(n.String()); top > maxDecimalExponent {
		return nil, true
	} else if top < -maxDecimalExponent {
		power = -maxDecimalExponent - len(n.String())
	}

	r := new(big.Rat).SetInt(n)
	r.Mul(r, pow10(power))

	if negative {
		r.Neg(r)
	}

	return r, true
}

// floatScanner walks a CVT_T_z input string.
type floatScanner struct {
	text  string
	i     int
	flags uint32
}

// skipLeading passes over the blanks (and tabs, when they're ignored)
// before the number: those are never zeros.
func (p *floatScanner) skipLeading() {
	for p.i < len(p.text) && (p.text[p.i] == ' ' || p.text[p.i] == '\t' && p.flags&cvtFlagIgnoreTabs != 0) {
		p.i++
	}
}

// filler reports what the character at p.i is when it's a blank or a
// tab: skip it, take it as a zero, or neither (it's another character,
// or a tab that's invalid).
func (p *floatScanner) filler() (skip, zero bool) {
	switch p.text[p.i] {
	case ' ':
		if p.flags&cvtFlagIgnoreBlanks != 0 {
			return true, false
		}

		return false, true
	case '\t':
		return p.flags&cvtFlagIgnoreTabs != 0, false
	}

	return false, false
}

// sign reads an optional sign, reporting whether it was a minus.
func (p *floatScanner) sign() bool {
	if p.i < len(p.text) && (p.text[p.i] == '+' || p.text[p.i] == '-') {
		p.i++

		return p.text[p.i-1] == '-'
	}

	return false
}

// mantissa reads the digits and period, blanks as zeros or skipped. It
// returns the digits and the number of them before the period (-1 for
// no period).
func (p *floatScanner) mantissa() (digits string, point int, ok bool) {
	var b strings.Builder

	point = -1

	for ; p.i < len(p.text); p.i++ {
		c := p.text[p.i]

		if skip, zero := p.filler(); skip {
			continue
		} else if zero {
			c = '0'
		}

		switch {
		case c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '.' && point < 0:
			point = b.Len()
		default:
			return b.String(), point, true
		}
	}

	return b.String(), point, true
}

// exponent reads an optional exponent: its letter (or, when the flags
// allow, only its sign), blanks, a sign, and digits. ok is false for a
// character the exponent can't start with.
func (p *floatScanner) exponent() (exp int, present, ok bool) {
	if p.done() {
		return 0, false, true
	}

	switch c := p.text[p.i]; {
	case c == 'E' || c == 'e':
		p.i++
	case (c == 'D' || c == 'd' || c == 'Q' || c == 'q') && p.flags&cvtFlagOnlyE == 0:
		p.i++
	case (c == '+' || c == '-') && p.flags&cvtFlagNeedLetter == 0:
	default:
		return 0, false, false
	}

	p.skipLeading()

	negative := p.sign()
	n := 0

	for ; p.i < len(p.text); p.i++ {
		c := p.text[p.i]

		if skip, zero := p.filler(); skip {
			continue
		} else if zero {
			c = '0'
		}

		if c < '0' || c > '9' {
			break
		}

		n = min(n*10+int(c-'0'), 10*maxDecimalExponent)
	}

	if negative {
		n = -n
	}

	return n, true, true
}

// done reports whether the scanner has read the whole string.
func (p *floatScanner) done() bool { return p.i >= len(p.text) }

// truncateToFormat truncates the nonzero v toward zero to format f's
// precision, returning the value and the next ext bits of v's magnitude
// after the ones kept, with vaxfloat.Round's range errors.
func truncateToFormat(f vaxfloat.Format, v *big.Rat, ext int) (vaxfloat.Value, uint64, error) {
	a := new(big.Rat).Abs(v)
	p := int(f.Precision())

	// The binary exponent e with 2^(e-1) <= a < 2^e.
	e := new(big.Float).SetPrec(64).SetRat(a).MantExp(nil)

	for a.Cmp(pow2(e)) >= 0 {
		e++
	}

	for a.Cmp(pow2(e-1)) < 0 {
		e--
	}

	// The significand and the extension bits, together, as an integer.
	scaled := new(big.Rat).Mul(a, pow2(p+ext-e))
	m := new(big.Int).Quo(scaled.Num(), scaled.Denom())

	extension := new(big.Int).And(m, big.NewInt(1<<ext-1)).Uint64()
	m.Rsh(m, uint(ext))

	t := new(big.Rat).Mul(new(big.Rat).SetInt(m), pow2(e-p))
	if v.Sign() < 0 {
		t.Neg(t)
	}

	r, err := vaxfloat.Round(f, vaxfloat.FromRat(t))

	return r, extension, err
}

// pow2 returns 2^n as a rational, n of either sign.
func pow2(n int) *big.Rat {
	p := new(big.Int).Lsh(big.NewInt(1), uint(abs(n)))
	if n < 0 {
		return new(big.Rat).SetFrac(big.NewInt(1), p)
	}

	return new(big.Rat).SetInt(p)
}
