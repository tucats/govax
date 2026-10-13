package librtl

import (
	"errors"
	"math/big"
	"regexp"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// LIB$CVT_DX_DX (docs/PHASE-54, "LIBRTL2: CVT_DX_DX"; LIB-60), LIBRTL2.EXE's
// general data-type conversion:
//
//	LIB$CVT_DX_DX source-item.rz.dx ,destination-item.wz.dx
//	              [,word-integer-dest-length.wwu.r]
//
// Each descriptor's data type and class say what the item is:
//
//   - an atomic integer (B, BU, W, WU, L, LU, Q) or floating value (F, D,
//     G, H), class S, or SD with a decimal scale (a value of n means the
//     integer times 10^scale);
//   - a numeric byte data string (NBDS), a number written as text: type
//     T with class S, D, VS, A, NCA, or SD, or type VT with class VS.
//
// The routine converts between any two of them, through the exact value:
// govax doesn't lose precision on the way (the manual: "intermediate
// values will not cause a loss of precision"), and rounds, half away from
// zero, only into the destination. The scope is the author's (Phase 54's
// open question): numeric byte arrays (BU with class A or NCA), packed
// decimal, and the other types aren't converted.

// The DSC$K_DTYPE_ codes the routine knows.
const (
	dtypeBU = 2
	dtypeWU = 3
	dtypeLU = 4
	dtypeB  = 6
	dtypeW  = 7
	dtypeL  = 8
	dtypeQ  = 9
	dtypeF  = 10
	dtypeD  = 11
	dtypeT  = dscDTypeT
	dtypeG  = 27
	dtypeH  = 28
	dtypeVT = 37
)

// dscScaleOffset is DSC$B_SCALE's offset in a class SD descriptor: a
// signed byte, the power of ten the stored integer is multiplied by.
const dscScaleOffset = 8

var (
	libInvClaDsc = vmsdef.LibrarySymbols["LIB$_INVCLADSC"]
	libInvClaDty = vmsdef.LibrarySymbols["LIB$_INVCLADTY"]
	libInvDtyDsc = vmsdef.LibrarySymbols["LIB$_INVDTYDSC"]
	libInvCvt    = vmsdef.LibrarySymbols["LIB$_INVCVT"]
	libInvNbds   = vmsdef.LibrarySymbols["LIB$_INVNBDS"]
	libOutStrTru = vmsdef.LibrarySymbols["LIB$_OUTSTRTRU"]
	libFltOvf    = vmsdef.LibrarySymbols["LIB$_FLTOVF"]
	libFltUnd    = vmsdef.LibrarySymbols["LIB$_FLTUND"]
	libROprand   = vmsdef.LibrarySymbols["LIB$_ROPRAND"]
)

// intType is an atomic integer type's size in bytes and signedness.
type intType struct {
	size   int
	signed bool
}

var intTypes = map[byte]intType{
	dtypeB: {1, true}, dtypeBU: {1, false},
	dtypeW: {2, true}, dtypeWU: {2, false},
	dtypeL: {4, true}, dtypeLU: {4, false},
	dtypeQ: {8, true},
}

var floatTypes = map[byte]vaxfloat.Format{
	dtypeF: vaxfloat.F, dtypeD: vaxfloat.D, dtypeG: vaxfloat.G, dtypeH: vaxfloat.H,
}

// cvtKind is what an item is.
type cvtKind int

const (
	cvtInteger cvtKind = iota
	cvtFloat
	cvtText // a numeric byte data string
)

// cvtItem is a source or destination, from its descriptor.
type cvtItem struct {
	desc  uint32 // the descriptor's address
	kind  cvtKind
	dtype byte
	class byte
	scale int // DSC$B_SCALE, for class SD; else 0
	ptr   uint32
}

// cvtStatus is a condition value the routine returns.
type cvtStatus uint32

func (s cvtStatus) Error() string { return "LIB$CVT_DX_DX status" }

// classify reads the descriptor at addr: LIB$_INVCLADSC for a class the
// routine doesn't take, LIB$_INVDTYDSC for a data type, LIB$_INVCLADTY for
// a pair of them that isn't an item.
func classify(env *corevms.Environment, addr uint32) (cvtItem, error) {
	mem, cpu := env.Memory(), env.CPU()
	it := cvtItem{desc: addr}

	var err error

	if it.dtype, err = mem.LoadByte(cpu, addr+2); err != nil {
		return it, err
	}

	if it.class, err = mem.LoadByte(cpu, addr+3); err != nil {
		return it, err
	}

	if it.ptr, err = mem.LoadLongword(cpu, addr+4); err != nil {
		return it, err
	}

	switch it.class {
	case dscClassS, dscClassD, dscClassA, dscClassSD, dscClassNCA, dscClassVS:
	default:
		return it, cvtStatus(libInvClaDsc)
	}

	if it.class == dscClassSD {
		scale, err := mem.LoadByte(cpu, addr+dscScaleOffset)
		if err != nil {
			return it, err
		}

		it.scale = int(int8(scale))
	}

	_, isInt := intTypes[it.dtype]
	_, isFloat := floatTypes[it.dtype]

	switch {
	case isInt || isFloat:
		if it.class != dscClassS && it.class != dscClassSD {
			return it, cvtStatus(libInvClaDty)
		}

		it.kind = cvtFloat
		if isInt {
			it.kind = cvtInteger
		}

	case it.dtype == dtypeT:
		it.kind = cvtText

	case it.dtype == dtypeVT:
		if it.class != dscClassVS {
			return it, cvtStatus(libInvClaDty)
		}

		it.kind = cvtText

	default:
		return it, cvtStatus(libInvDtyDsc)
	}

	return it, nil
}

// pow10 returns 10^n as a rational, n of either sign.
func pow10(n int) *big.Rat {
	p := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(n))), nil)
	if n < 0 {
		return new(big.Rat).SetFrac(big.NewInt(1), p)
	}

	return new(big.Rat).SetInt(p)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}

// readInteger reads an atomic integer item's value, scale applied.
func readInteger(env *corevms.Environment, it cvtItem) (*big.Rat, error) {
	t := intTypes[it.dtype]
	mem, cpu := env.Memory(), env.CPU()

	var u uint64

	for i := t.size - 1; i >= 0; i-- {
		b, err := mem.LoadByte(cpu, it.ptr+uint32(i))
		if err != nil {
			return nil, err
		}

		u = u<<8 | uint64(b)
	}

	n := new(big.Int).SetUint64(u)

	if t.signed && u>>(8*t.size-1)&1 != 0 {
		n.Sub(n, new(big.Int).Lsh(big.NewInt(1), uint(8*t.size)))
	}

	r := new(big.Rat).SetInt(n)

	return r.Mul(r, pow10(it.scale)), nil
}

// nbdsSyntax is a numeric byte data string's number, once blanks and tabs
// are gone (OTS$CVT_T_z's rules with blanks and tabs ignored, as the
// manual has an NBDS source read): a sign, digits with an optional
// decimal point, and an optional exponent, its letter E, D, G, H, or Q.
var nbdsSyntax = regexp.MustCompile(`^([+-]?)(\d*\.?\d*)(?:[EDGHQ]([+-]?\d+))?$`)

// parseNBDS reads a numeric byte data string's value. ok is false for
// text that isn't a number. An empty string (or blanks) is 0.
func parseNBDS(text string) (*big.Rat, bool) {
	text = strings.ToUpper(strings.NewReplacer(" ", "", "\t", "").Replace(text))
	if text == "" {
		return new(big.Rat), true
	}

	m := nbdsSyntax.FindStringSubmatch(text)
	if m == nil || strings.Trim(m[2], ".") == "" && m[2] != "" || m[2] == "" && m[3] != "" {
		return nil, false
	}

	digits := m[2]
	if digits == "" {
		digits = "0"
	}

	r, ok := new(big.Rat).SetString(m[1] + digits)
	if !ok {
		return nil, false
	}

	if m[3] != "" {
		e, ok := new(big.Int).SetString(m[3], 10)
		if !ok || !e.IsInt64() || abs(int(e.Int64())) > 10000 {
			return nil, false
		}

		r.Mul(r, pow10(int(e.Int64())))
	}

	return r, true
}

// roundRat rounds r to an integer, half away from zero.
func roundRat(r *big.Rat) *big.Int {
	num, den := new(big.Int).Abs(r.Num()), r.Denom()
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int))

	if rem.Lsh(rem, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}

	if r.Sign() < 0 {
		q.Neg(q)
	}

	return q
}

// writeInteger stores v in an atomic integer destination, its scale
// applied (the stored integer is v / 10^scale, rounded). A negative value
// for an unsigned type is LIB$_INVCVT; one out of range is overflow
// (LIB$_INTOVF, or LIB$_INVNBDS from a numeric string).
func writeInteger(env *corevms.Environment, it cvtItem, v *big.Rat, fromText bool) error {
	t := intTypes[it.dtype]
	n := roundRat(new(big.Rat).Mul(v, pow10(-it.scale)))

	if !t.signed && n.Sign() < 0 {
		return cvtStatus(libInvCvt)
	}

	bits := 8 * t.size
	lo, hi := big.NewInt(0), new(big.Int).Lsh(big.NewInt(1), uint(bits))

	if t.signed {
		lo = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)))
		hi = new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
	}

	if n.Cmp(lo) < 0 || n.Cmp(hi) >= 0 {
		if fromText {
			return cvtStatus(libInvNbds)
		}

		return cvtStatus(libIntOvf)
	}

	if n.Sign() < 0 {
		n.Add(n, new(big.Int).Lsh(big.NewInt(1), uint(bits)))
	}

	u := n.Uint64()
	mem, cpu := env.Memory(), env.CPU()

	for i := range t.size {
		if err := mem.StoreByte(cpu, it.ptr+uint32(i), byte(u>>(8*i))); err != nil {
			return writing(err)
		}
	}

	return nil
}

// writeFloating stores v in a floating destination, rounded to its
// format: overflow and underflow are LIB$_FLTOVF and LIB$_FLTUND, or
// LIB$_INVNBDS from a numeric string ("outside the range that can be
// represented by the destination").
func writeFloating(env *corevms.Environment, it cvtItem, v *big.Rat, fromText bool) error {
	f := floatTypes[it.dtype]

	r, err := vaxfloat.Round(f, vaxfloat.FromRat(v))
	if err != nil {
		switch {
		case fromText:
			return cvtStatus(libInvNbds)
		case errors.Is(err, vaxfloat.ErrOverflow):
			return cvtStatus(libFltOvf)
		default:
			return cvtStatus(libFltUnd)
		}
	}

	if err := storeFloat(env, f, it.ptr, r); err != nil {
		return writing(err)
	}

	return nil
}

// floatDigits is a floating type's significant digits in its text form,
// and its exponent's digits (the manual's Table LIB-3).
var floatDigits = map[vaxfloat.Format]struct{ digits, exponent int }{
	vaxfloat.F: {7, 2}, vaxfloat.D: {16, 2}, vaxfloat.G: {15, 3}, vaxfloat.H: {33, 4},
}

// floatText writes v as Table LIB-3's floating NBDS: s0.dddE+nn, the sign
// only when negative, with min(digits, width - room for the rest)
// significant digits, rounded. width is the destination's size in bytes,
// or 0 for none (a dynamic string): the full digits.
func floatText(v *big.Rat, f vaxfloat.Format, width int) string {
	fd := floatDigits[f]
	digits := fd.digits

	if width > 0 {
		digits = min(digits, width-(5+fd.exponent))
	}

	digits = max(digits, 1)

	sign := ""
	if v.Sign() < 0 {
		sign = "-"
	}

	a := new(big.Rat).Abs(v)
	exp := 0

	if a.Sign() != 0 {
		// The decimal exponent e with 0.1 <= a / 10^e < 1: first a guess
		// from the binary exponent, then corrected.
		bf := new(big.Float).SetPrec(64).SetRat(a)
		exp = int(float64(bf.MantExp(nil)) * 0.30102999566398) // log10(2)

		for a.Cmp(pow10(exp)) >= 0 {
			exp++
		}

		for a.Cmp(pow10(exp-1)) < 0 {
			exp--
		}
	}

	m := roundRat(new(big.Rat).Mul(a, pow10(digits-exp)))
	if m.Cmp(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)) >= 0 {
		m.Quo(m, big.NewInt(10))
		exp++
	}

	if a.Sign() == 0 {
		exp = 0
	}

	esign := "+"
	if exp < 0 {
		esign = "-"
	}

	return sign + "0." + leftPad(m.String(), digits) + "E" + esign + leftPad(big.NewInt(int64(abs(exp))).String(), fd.exponent)
}

// leftPad pads s on the left with zeros to n characters.
func leftPad(s string, n int) string {
	if len(s) >= n {
		return s
	}

	return strings.Repeat("0", n-len(s)) + s
}

// scaledText writes v as Table LIB-3's integer NBDS: its digits, a minus
// sign when negative, and a decimal point with -scale digits after it
// when scale is negative (the manual: "a decimal point only if source
// descriptor specifies DSC$B_SCALE less than 0"), v rounded to them.
func scaledText(v *big.Rat, scale int) string {
	n := roundRat(new(big.Rat).Mul(v, pow10(-min(scale, 0))))

	sign := ""
	if n.Sign() < 0 {
		sign = "-"
	}

	s := new(big.Int).Abs(n).String()
	if scale >= 0 {
		return sign + s
	}

	s = leftPad(s, -scale+1)

	return sign + s[:len(s)+scale] + "." + s[len(s)+scale:]
}

// textWidth is a string destination's size in bytes, for a floating
// value's digits: a fixed string's length, a varying string's MAXSTRLEN,
// or 0 for a dynamic string (no limit).
func textWidth(env *corevms.Environment, it cvtItem) (int, error) {
	if it.class == dscClassD {
		return 0, nil
	}

	d, err := readDescriptor(env, it.desc)
	if err != nil {
		return 0, err
	}

	return int(d.limit), nil
}

// convert is LIB$CVT_DX_DX's work, returning the length to report (for a
// string destination) and an error: a cvtStatus to return, or a memory or
// descriptor helper's error.
func convert(env *corevms.Environment, src, dst cvtItem) (int, error) {
	var (
		value    *big.Rat
		format   vaxfloat.Format
		fromText = src.kind == cvtText
	)

	switch src.kind {
	case cvtInteger:
		v, err := readInteger(env, src)
		if err != nil {
			return 0, err
		}

		value = v

	case cvtFloat:
		format = floatTypes[src.dtype]

		v, err := loadFloat(env, format, src.ptr)
		if errors.Is(err, vaxfloat.ErrReserved) {
			return 0, cvtStatus(libROprand)
		} else if err != nil {
			return 0, err
		}

		value = v.Rat()

	case cvtText:
		s, err := readString(env, src.desc)
		if err != nil {
			return 0, err
		}

		// An NBDS to an NBDS, unscaled, is a copy.
		if dst.kind == cvtText && src.scale == 0 && dst.scale == 0 {
			stored, truncated, err := writeString(env, dst.desc, s)
			if err != nil {
				return 0, err
			}

			if truncated {
				return stored, cvtStatus(libOutStrTru)
			}

			return stored, nil
		}

		v, ok := parseNBDS(s)
		if !ok {
			return 0, cvtStatus(libInvNbds)
		}

		value = v.Mul(v, pow10(src.scale))
	}

	switch dst.kind {
	case cvtInteger:
		return 0, writeInteger(env, dst, value, fromText)

	case cvtFloat:
		return 0, writeFloating(env, dst, value, fromText)
	}

	// To text: a floating value in E format, anything else as a (scaled)
	// integer.
	var text string

	if src.kind == cvtFloat {
		width, err := textWidth(env, dst)
		if err != nil {
			return 0, err
		}

		text = floatText(value, format, width)
	} else {
		// A scaled destination string holds the value over 10^scale.
		text = scaledText(new(big.Rat).Mul(value, pow10(-dst.scale)), src.scale)
	}

	stored, truncated, err := writeString(env, dst.desc, text)
	if err != nil {
		return 0, err
	}

	if truncated {
		return stored, cvtStatus(libOutStrTru)
	}

	return stored, nil
}

// libCvtDxDx is LIB$CVT_DX_DX. It returns SS$_NORMAL; LIB$_INVCLADSC,
// LIB$_INVDTYDSC, or LIB$_INVCLADTY for a descriptor it doesn't take;
// LIB$_INVNBDS for a numeric string that isn't a number or whose value
// the destination can't hold; LIB$_INVCVT for a negative value into an
// unsigned type; LIB$_INTOVF, LIB$_FLTOVF, LIB$_FLTUND for a value out of
// an atomic destination's range; LIB$_ROPRAND for a reserved floating
// source; LIB$_OUTSTRTRU when a string destination couldn't hold the text
// (the manual has it only for a copy of one numeric string to another;
// govax uses it for any text cut short: unconfirmed); LIB$_INVSTRDES or
// LIB$_INSVIRMEM from a string destination. word-integer-dest-length, if
// given, gets the characters written to a string destination, without
// padding.
func libCvtDxDx(env *corevms.Environment, argv []uint32) (uint32, error) {
	src, err := classify(env, arg(argv, 0))
	if err == nil {
		var dst cvtItem

		if dst, err = classify(env, arg(argv, 1)); err == nil {
			var stored int

			stored, err = convert(env, src, dst)

			if addr := arg(argv, 2); addr != 0 && dst.kind == cvtText && (err == nil || errors.Is(err, cvtStatus(libOutStrTru))) {
				if serr := env.Memory().StoreWord(env.CPU(), addr, uint16(stored)); serr != nil {
					return memoryFault(env, writing(serr))
				}
			}
		}
	}

	var status cvtStatus

	switch {
	case err == nil:
		return ssNormal, nil
	case errors.As(err, &status):
		return uint32(status), nil
	default:
		return stringFailure(env, err)
	}
}
