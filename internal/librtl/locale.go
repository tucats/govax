package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
)

// The locale routines (docs/PHASE-54, "Locale"): LIB$DIGIT_SEP,
// LIB$RADIX_POINT, LIB$CURRENCY, and LIB$LP_LINES. Each returns what a
// logical name says, for programs (BASIC's, implicitly) that format
// numbers and listings: a system manager defines the name system-wide,
// and a user can override it with a process logical name.

// localeName translates a locale logical name "as a process, group, or
// system logical name, in that order" (the manual's words for each
// routine): the first table of the three that has it gives its first
// equivalence string. ok is false when none does.
func localeName(env *corevms.Environment, name string) (string, bool) {
	db := env.Logicals
	if db == nil {
		return "", false
	}

	for _, table := range []string{lnm.ProcessTableName, db.GroupTableName, lnm.SystemTableName} {
		if table == "" {
			continue
		}

		e, err := db.Translate(table, name, lnm.User, 0)
		if err == nil && len(e.Equivalences) > 0 {
			return e.Equivalences[0].Value, true
		}
	}

	return "", false
}

// localeString is LIB$DIGIT_SEP's, LIB$RADIX_POINT's, and LIB$CURRENCY's
// work:
//
//	LIB$xxx string.wt.dx [,resultant-length.wwu.r]
//
// The logical name's translation, or fallback when it has none, is
// written to string by its class, and resultant-length (if given) gets
// the characters written, the truncated length when it was cut. It
// returns SS$_NORMAL, LIB$_STRTRU, LIB$_INVSTRDES, or LIB$_INSVIRMEM.
func localeString(env *corevms.Environment, argv []uint32, name, fallback string) (uint32, error) {
	text, ok := localeName(env, name)
	if !ok {
		text = fallback
	}

	stored, truncated, err := writeString(env, arg(argv, 0), text)
	if err != nil {
		return stringFailure(env, err)
	}

	if addr := arg(argv, 1); addr != 0 {
		if err := env.Memory().StoreWord(env.CPU(), addr, uint16(stored)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if truncated {
		return libStrTru, nil
	}

	return ssNormal, nil
}

// libDigitSep is LIB$DIGIT_SEP (LIB-119): SYS$DIGIT_SEP, or a comma, the
// separator of groups of three digits (3,006,854).
func libDigitSep(env *corevms.Environment, argv []uint32) (uint32, error) {
	return localeString(env, argv, "SYS$DIGIT_SEP", ",")
}

// libRadixPoint is LIB$RADIX_POINT (LIB-312): SYS$RADIX_POINT, or a
// period, the separator of a number's integer and fraction.
func libRadixPoint(env *corevms.Environment, argv []uint32) (uint32, error) {
	return localeString(env, argv, "SYS$RADIX_POINT", ".")
}

// libCurrency is LIB$CURRENCY (LIB-58): SYS$CURRENCY, or a dollar sign.
func libCurrency(env *corevms.Environment, argv []uint32) (uint32, error) {
	return localeString(env, argv, "SYS$CURRENCY", "$")
}

// Printer page lengths: the manual's range for SYS$LP_LINES, and the
// default, United States paper's 66 lines.
const (
	lpLinesMin     = 30
	lpLinesMax     = 255
	lpLinesDefault = 66
)

// libLpLines is LIB$LP_LINES (LIB-266), with no arguments: the number of
// lines on a printer page. SYS$LP_LINES is translated and converted to
// binary as LIB$CVT_DTB would (unconfirmed: the manual says only
// "convert the ASCII value obtained to a binary integer"); anything that
// fails, or a value outside 30 to 255, gives 66.
func libLpLines(env *corevms.Environment, _ []uint32) (uint32, error) {
	text, ok := localeName(env, "SYS$LP_LINES")
	if !ok {
		return lpLinesDefault, nil
	}

	value, ok := decimalText(text)
	if !ok || value < lpLinesMin || value > lpLinesMax {
		return lpLinesDefault, nil
	}

	return uint32(value), nil
}
