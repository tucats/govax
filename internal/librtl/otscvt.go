package librtl

import (
	"errors"
	"math/big"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The OTS$ integer conversions (Phase 72): the routines FORTRAN's
// formatted I/O calls for its I, O, Z, and L edit descriptors, and BASIC
// and the other languages for theirs, from the VMS 5.0 RTL General
// Purpose (OTS$) manual, OTS-5 to OTS-28. VMS 7.1's FORRTL.EXE, which a
// FORTRAN program activates, calls them in LIBRTL (DUNGEON's score line
// is OTS$CVT_L_TI's).
//
// To text, each takes the value by reference, its size in bytes, and a
// fixed-length output string by descriptor:
//
//	OTS$CVT_L_TI value.rx.r ,out-str.wt.ds [,int-digits.rl.v]
//	             [,value-size.rl.v] [,flags.rl.v]
//
// The digits are right-justified in the string, blanks before them, with
// at least int-digits digits (zeros added on the left; default 1): a
// zero with int-digits 0 is all blanks. A result longer than the string
// fills it with asterisks and returns OTS$_OUTCONERR. OTS$CVT_L_TI is
// signed (a byte or word sign-extended; flags bit 0 puts a + before a
// value that isn't negative); L_TU is unsigned decimal; L_TB, L_TO, and
// L_TZ are binary, octal, and hexadecimal of an unsigned value of any
// length. OTS$CVT_L_TL is FORTRAN's L: blanks, then T if the value's bit
// 0 is set, F if not.
//
// From text, each takes the input string by descriptor (any class) and
// the result by reference, with its size and flags by value:
//
//	OTS$CVT_TI_L in-str.rt.dx ,value.wx.r [,value-size.rl.v] [,flags.rl.v]
//
// Flags bit 0 ignores blanks, which are otherwise zeros (after the first
// character that isn't one, for TI_L, which always passes over leading
// blanks); bit 4 ignores tabs, which are otherwise invalid. An invalid
// character, a value that doesn't fit, or a bad size is
// OTS$_INPCONERR, and TI_L and TL_L then store zero. TI_L takes an
// optional sign; TB_L, TO_L, TU_L, and TZ_L none. TL_L reads blanks, an
// optional period, and T or F (either case), ignoring what follows: T is
// -1, F 0, and an empty or blank string 0.
//
// Unconfirmed (no probe yet; the manual doesn't say): a value size other
// than 1, 2, or 4 for L_TI (taken as 4, as L_TU's manual entry says of
// its own); a size of 0 or less for L_TB, L_TO, and L_TZ (taken as 4);
// a negative int-digits (taken as 0); that a + goes before a value
// written as all blanks (it doesn't); lowercase hexadecimal digits in
// TZ_L's input (accepted); tabs in TO_L's input (bit 4 as for the
// others); what TB_L, TO_L, TU_L, and TZ_L leave in the result on an
// error (nothing is stored); and the longest size L_TB/TO/TZ and
// TB/TO/TZ_L take (16 bytes, an octaword, here).

var (
	otsOutConErr = vmsdef.LibrarySymbols["OTS$_OUTCONERR"]
	otsInpConErr = vmsdef.LibrarySymbols["OTS$_INPCONERR"]
	otsInvStrDes = vmsdef.LibrarySymbols["OTS$_INVSTRDES"]
)

// maxOTSValueSize is the longest value the arbitrary-length routines
// convert, in bytes (unconfirmed: see above).
const maxOTSValueSize = 16

// errInputConversion is a text-to-integer routine's failure: an invalid
// character, a value too big for its size, or a bad size.
var errInputConversion = errors.New("input conversion error")

// The flags' bits.
const (
	otsFlagPlus       = 1 << 0 // L_TI: a + before a value that isn't negative
	otsFlagIgnoreBlnk = 1 << 0 // TB/TI/TO/TU/TZ_L: blanks ignored
	otsFlagIgnoreTab  = 1 << 4 // TB/TI/TU/TZ_L: tabs ignored
)

func otsCvtLTI(env *corevms.Environment, argv []uint32) (uint32, error) {
	size := sizeOneTwoFour(argv, 3)

	value, err := loadSized(env, arg(argv, 0), size)
	if err != nil {
		return memoryFault(env, err)
	}

	// Sign-extend a byte or word.
	n := new(big.Int).SetBytes(reverse(value))
	if top := uint(size * 8); n.Bit(int(top)-1) == 1 {
		n.Sub(n, new(big.Int).Lsh(big.NewInt(1), top))
	}

	digits := intDigits(argv, 2)

	text := zeroPadded(new(big.Int).Abs(n).Text(10), n.Sign() == 0, digits)
	if text != "" {
		switch {
		case n.Sign() < 0:
			text = "-" + text
		case len(argv) > 4 && argv[4]&otsFlagPlus != 0:
			text = "+" + text
		}
	}

	return storeRightJustified(env, arg(argv, 1), text)
}

func otsCvtLTU(env *corevms.Environment, argv []uint32) (uint32, error) {
	return unsignedToText(env, argv, 10, sizeOneTwoFour(argv, 3))
}

func otsCvtLTB(env *corevms.Environment, argv []uint32) (uint32, error) {
	return unsignedToText(env, argv, 2, anySize(argv, 3))
}

func otsCvtLTO(env *corevms.Environment, argv []uint32) (uint32, error) {
	return unsignedToText(env, argv, 8, anySize(argv, 3))
}

func otsCvtLTZ(env *corevms.Environment, argv []uint32) (uint32, error) {
	return unsignedToText(env, argv, 16, anySize(argv, 3))
}

func otsCvtLTL(env *corevms.Environment, argv []uint32) (uint32, error) {
	value, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	d, err := readOTSOutput(env, arg(argv, 1))
	if err != nil {
		return otsStringFailure(env, err)
	}

	// No room for the letter (the manual: the string is left empty).
	if d.length == 0 {
		return otsOutConErr, nil
	}

	letter := "F"
	if value&1 != 0 {
		letter = "T"
	}

	return storeRightJustified(env, arg(argv, 1), letter)
}

// unsignedToText is L_TU's, L_TB's, L_TO's, and L_TZ's work: the size
// bytes at argument 0, unsigned, in radix.
func unsignedToText(env *corevms.Environment, argv []uint32, radix, size int) (uint32, error) {
	value, err := loadSized(env, arg(argv, 0), size)
	if err != nil {
		return memoryFault(env, err)
	}

	n := new(big.Int).SetBytes(reverse(value))
	text := strings.ToUpper(zeroPadded(n.Text(radix), n.Sign() == 0, intDigits(argv, 2)))

	return storeRightJustified(env, arg(argv, 1), text)
}

// zeroPadded is digits with zeros before them to make at least minimum,
// and a zero value with minimum 0 no digits at all.
func zeroPadded(digits string, zero bool, minimum int) string {
	if zero {
		digits = ""
	}

	if len(digits) < minimum {
		digits = strings.Repeat("0", minimum-len(digits)) + digits
	}

	return digits
}

// intDigits is the int-digits argument at i: 1 when omitted, 0 for a
// negative count.
func intDigits(argv []uint32, i int) int {
	if i >= len(argv) {
		return 1
	}

	return max(int(int32(argv[i])), 0)
}

// sizeOneTwoFour is a value-size argument at i that must be 1, 2, or 4:
// 4 when omitted or anything else.
func sizeOneTwoFour(argv []uint32, i int) int {
	if i < len(argv) {
		switch argv[i] {
		case 1, 2, 4:
			return int(argv[i])
		}
	}

	return 4
}

// anySize is an arbitrary-length routine's value-size argument at i: 4
// when omitted or not positive, at most maxOTSValueSize.
func anySize(argv []uint32, i int) int {
	if i >= len(argv) || int32(argv[i]) <= 0 {
		return 4
	}

	return min(int(argv[i]), maxOTSValueSize)
}

// loadSized reads size bytes at addr, the low byte first.
func loadSized(env *corevms.Environment, addr uint32, size int) ([]byte, error) {
	mem, cpu := env.Memory(), env.CPU()
	value := make([]byte, size)

	for i := range value {
		var err error

		if value[i], err = mem.LoadByte(cpu, addr+uint32(i)); err != nil {
			return nil, err
		}
	}

	return value, nil
}

// storeSized writes n's low size bytes at addr, the low byte first (n
// in two's complement when negative).
func storeSized(env *corevms.Environment, addr uint32, n *big.Int, size int) error {
	mem, cpu := env.Memory(), env.CPU()

	m := new(big.Int).Set(n)
	if m.Sign() < 0 {
		m.Add(m, new(big.Int).Lsh(big.NewInt(1), uint(size*8)))
	}

	bytes := reverse(m.Bytes())

	for i := range size {
		var b byte
		if i < len(bytes) {
			b = bytes[i]
		}

		if err := mem.StoreByte(cpu, addr+uint32(i), b); err != nil {
			return writing(err)
		}
	}

	return nil
}

// reverse returns b's bytes in the other order: VAX memory holds a
// value's low byte first, math/big's byte slices its high byte first.
func reverse(b []byte) []byte {
	r := make([]byte, len(b))
	for i, c := range b {
		r[len(b)-1-i] = c
	}

	return r
}

// storeRightJustified writes text at the right of the fixed-length
// string the descriptor at addr describes, blanks before it, or fills
// the string with asterisks and returns OTS$_OUTCONERR when text is
// longer. A class readDescriptor doesn't know is a fixed string of its
// length and pointer alone, as readAnyString reads one: VMS's FORRTL
// sets only those two fields of the descriptors it passes for a
// list-directed value, leaving the rest as the stack had it.
func storeRightJustified(env *corevms.Environment, addr uint32, text string) (uint32, error) {
	d, err := readOTSOutput(env, addr)
	if err != nil {
		return otsStringFailure(env, err)
	}

	width := int(d.length)
	sts := ssNormal

	if len(text) > width {
		text, sts = strings.Repeat("*", width), otsOutConErr
	}

	padded := strings.Repeat(" ", width-len(text)) + text

	if d.class == dscClassUnknown {
		for i := range len(padded) {
			if err := env.Memory().StoreByte(env.CPU(), d.text+uint32(i), padded[i]); err != nil {
				return memoryFault(env, writing(err))
			}
		}

		return sts, nil
	}

	if _, _, err := writeString(env, addr, padded); err != nil {
		return otsStringFailure(env, err)
	}

	return sts, nil
}

// dscClassUnknown marks a stringDesc whose class readDescriptor refused.
const dscClassUnknown = 255

// readOTSOutput is readDescriptor for a conversion's output string: a
// class it doesn't know comes back as dscClassUnknown, with the
// descriptor's length and pointer.
func readOTSOutput(env *corevms.Environment, addr uint32) (stringDesc, error) {
	d, err := readDescriptor(env, addr)
	if !errors.Is(err, errInvStrDes) {
		return d, err
	}

	length, err := env.Memory().LoadWord(env.CPU(), addr)
	if err != nil {
		return d, err
	}

	d.length, d.limit, d.class = uint32(length), uint32(length), dscClassUnknown

	return d, nil
}

// otsStringFailure is stringFailure with OTS$'s status for a bad
// descriptor.
func otsStringFailure(env *corevms.Environment, err error) (uint32, error) {
	if errors.Is(err, errInvStrDes) {
		return otsInvStrDes, nil
	}

	return stringFailure(env, err)
}

func otsCvtTIL(env *corevms.Environment, argv []uint32) (uint32, error) {
	size, ok := strictSize(argv, 2)

	return textToInteger(env, argv, size, ok, true, func(text string, flags uint32) (*big.Int, error) {
		return parseDigits(text, 10, flags, true)
	})
}

func otsCvtTUL(env *corevms.Environment, argv []uint32) (uint32, error) {
	return textToInteger(env, argv, sizeOneTwoFour(argv, 2), true, false, func(text string, flags uint32) (*big.Int, error) {
		return parseDigits(text, 10, flags, false)
	})
}

func otsCvtTBL(env *corevms.Environment, argv []uint32) (uint32, error) {
	return unsignedFromText(env, argv, 2)
}

func otsCvtTOL(env *corevms.Environment, argv []uint32) (uint32, error) {
	return unsignedFromText(env, argv, 8)
}

func otsCvtTZL(env *corevms.Environment, argv []uint32) (uint32, error) {
	return unsignedFromText(env, argv, 16)
}

func otsCvtTLL(env *corevms.Environment, argv []uint32) (uint32, error) {
	size, ok := strictSize(argv, 2)

	return textToInteger(env, argv, size, ok, true, func(text string, _ uint32) (*big.Int, error) {
		text = strings.TrimLeft(text, " ")
		text = strings.TrimPrefix(text, ".")

		if text == "" {
			return new(big.Int), nil
		}

		switch text[0] {
		case 'T', 't':
			return big.NewInt(-1), nil
		case 'F', 'f':
			return new(big.Int), nil
		}

		return nil, errInputConversion
	})
}

// unsignedFromText is TB_L's, TO_L's, and TZ_L's work, in radix.
func unsignedFromText(env *corevms.Environment, argv []uint32, radix int) (uint32, error) {
	size, ok := 4, true
	if len(argv) > 2 {
		ok = int32(argv[2]) > 0
		size = min(max(int(int32(argv[2])), 1), maxOTSValueSize)
	}

	return textToInteger(env, argv, size, ok, false, func(text string, flags uint32) (*big.Int, error) {
		return parseDigits(text, radix, flags, false)
	})
}

// strictSize is TI_L's and TL_L's value-size argument at i: 4 when
// omitted, and ok false for anything but 1, 2, or 4.
func strictSize(argv []uint32, i int) (size int, ok bool) {
	if i >= len(argv) {
		return 4, true
	}

	switch argv[i] {
	case 1, 2, 4:
		return int(argv[i]), true
	}

	return 4, false
}

// textToInteger is the text-to-integer routines' common work: the
// string at argument 0 parsed by parse with the flags at argument 3, and
// the value, which must fit size bytes (signed or unsigned), stored at
// argument 1. sizeOK false is a bad size. The signed routines, TI_L and
// TL_L, are also the ones that store zero on a failure.
func textToInteger(env *corevms.Environment, argv []uint32, size int, sizeOK, signed bool,
	parse func(text string, flags uint32) (*big.Int, error),
) (uint32, error) {
	text, err := readAnyString(env, arg(argv, 0))
	if err != nil {
		return otsStringFailure(env, err)
	}

	n, perr := new(big.Int), error(nil)
	if !sizeOK {
		perr = errInputConversion
	} else if n, perr = parse(text, arg(argv, 3)); perr == nil && !fits(n, size, signed) {
		perr = errInputConversion
	}

	if perr != nil {
		if signed {
			if err := storeSized(env, arg(argv, 1), new(big.Int), size); err != nil {
				return memoryFault(env, err)
			}
		}

		return otsInpConErr, nil
	}

	if err := storeSized(env, arg(argv, 1), n, size); err != nil {
		return memoryFault(env, err)
	}

	return ssNormal, nil
}

// fits reports whether n fits size bytes, as a signed or unsigned value.
func fits(n *big.Int, size int, signed bool) bool {
	bits := uint(size * 8)

	if !signed {
		return n.Sign() >= 0 && n.BitLen() <= int(bits)
	}

	limit := new(big.Int).Lsh(big.NewInt(1), bits-1) // 2^(bits-1)

	return n.Cmp(new(big.Int).Neg(limit)) >= 0 && n.Cmp(limit) < 0
}

// parseDigits reads text as digits of radix by the flags (see this
// file's opening comment), with an optional sign when signed (TI_L,
// which also passes over leading blanks). No digits at all is zero.
func parseDigits(text string, radix int, flags uint32, signed bool) (*big.Int, error) {
	if signed {
		text = strings.TrimLeft(text, " ")
	}

	n := new(big.Int)
	r := big.NewInt(int64(radix))
	negative := false

	for i := range len(text) {
		c := text[i]

		switch {
		case c == ' ':
			if flags&otsFlagIgnoreBlnk != 0 {
				continue
			}

			c = '0'
		case c == '\t':
			if flags&otsFlagIgnoreTab != 0 {
				continue
			}

			return nil, errInputConversion
		case signed && i == 0 && (c == '+' || c == '-'):
			negative = c == '-'

			continue
		}

		digit, ok := digitValue(c)
		if !ok || digit >= uint64(radix) {
			return nil, errInputConversion
		}

		n.Mul(n, r).Add(n, big.NewInt(int64(digit)))
	}

	if negative {
		n.Neg(n)
	}

	return n, nil
}
