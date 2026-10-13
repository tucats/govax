package librtl

import (
	"github.com/tucats/govax/internal/corevms"
)

// Numeric text to binary (docs/PHASE-54, "Numeric text"): LIB$CVT_DTB,
// LIB$CVT_HTB, and LIB$CVT_OTB (LIB-75, "LIB$CVT_xTB"). They're meant for
// MACRO and BLISS, and unlike the rest of LIB$ take the byte count by
// value and the text by reference:
//
//	LIB$CVT_DTB byte-count.rl.v ,numeric-string.rt.r ,result.wl.r
//
// and return 1 for success and 0 for failure, not condition values.

// Their two statuses.
const (
	cvtFailure = 0
	cvtSuccess = 1
)

// textToBinary is the routines' work, in radix 10, 16, or 8. The text may
// start with a sign, + or -, and the rest must all be digits of the radix
// (either case for hexadecimal; unconfirmed for lowercase): a blank, a
// tab, a sign anywhere else, or any other character fails. So does a
// magnitude that doesn't fit 32 bits unsigned; the result is the
// magnitude negated for -, in two's complement, so "4294967295" is -1
// (unconfirmed). No digits at all (a count of 0, or a sign alone) is 0
// (unconfirmed), and a negative count fails (unconfirmed). result is
// written only on success (unconfirmed).
func textToBinary(env *corevms.Environment, argv []uint32, radix uint64) (uint32, error) {
	count := int32(arg(argv, 0))
	if count < 0 {
		return cvtFailure, nil
	}

	text, err := readText(env, arg(argv, 1), uint32(count))
	if err != nil {
		return memoryFault(env, err)
	}

	result, ok := parseBinary(text, radix)
	if !ok {
		return cvtFailure, nil
	}

	if err := env.Memory().StoreLongword(env.CPU(), arg(argv, 2), result); err != nil {
		return memoryFault(env, writing(err))
	}

	return cvtSuccess, nil
}

// parseBinary is textToBinary's conversion of text, in radix: ok is false
// for a character that isn't allowed or a magnitude over 32 bits.
func parseBinary(text string, radix uint64) (uint32, bool) {
	negative := false

	if text != "" && (text[0] == '+' || text[0] == '-') {
		negative, text = text[0] == '-', text[1:]
	}

	var value uint64

	for i := range len(text) {
		digit, ok := digitValue(text[i])
		if !ok || digit >= radix {
			return 0, false
		}

		if value = value*radix + digit; value > 0xFFFFFFFF {
			return 0, false
		}
	}

	result := uint32(value)
	if negative {
		result = -result
	}

	return result, true
}

// decimalText is LIB$CVT_DTB's conversion of text, as a signed value.
func decimalText(text string) (int32, bool) {
	v, ok := parseBinary(text, 10)

	return int32(v), ok
}

// digitValue is the value of a digit character of radix 16 or less: 0-9,
// A-F, or a-f.
func digitValue(ch byte) (uint64, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return uint64(ch - '0'), true
	case ch >= 'A' && ch <= 'F':
		return uint64(ch-'A') + 10, true
	case ch >= 'a' && ch <= 'f':
		return uint64(ch-'a') + 10, true
	default:
		return 0, false
	}
}

// libCvtDtb is LIB$CVT_DTB: decimal text to binary.
func libCvtDtb(env *corevms.Environment, argv []uint32) (uint32, error) {
	return textToBinary(env, argv, 10)
}

// libCvtHtb is LIB$CVT_HTB: hexadecimal text to binary.
func libCvtHtb(env *corevms.Environment, argv []uint32) (uint32, error) {
	return textToBinary(env, argv, 16)
}

// libCvtOtb is LIB$CVT_OTB: octal text to binary.
func libCvtOtb(env *corevms.Environment, argv []uint32) (uint32, error) {
	return textToBinary(env, argv, 8)
}
