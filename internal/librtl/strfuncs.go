package librtl

import (
	"github.com/tucats/govax/internal/corevms"
)

// The string functions (docs/PHASE-54, "Characters and search"): LIB$LEN,
// LIB$CHAR, LIB$ICHAR, and LIB$INDEX, what FORTRAN's and BASIC's LEN,
// CHAR, ICHAR, and INDEX intrinsics compute in line. LEN, ICHAR, and
// INDEX return values, not statuses, so an invalid descriptor can't be
// reported: it reads as an empty string, as for LIB$LOCC (whose VMS run
// showed that, Phase 53).

// libLen is LIB$LEN (LIB-256): source-string.rt.dx. It returns the
// string's length, zero-extended: an array's ARSIZE, a varying string's
// CURLEN.
func libLen(env *corevms.Environment, argv []uint32) (uint32, error) {
	d, err := searchDescriptor(env, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	return d.length, nil
}

// libChar is LIB$CHAR (LIB-24): one-character-string.wt.dx,
// ascii-code.rbu.r. It writes the character as a string, blank-filled to
// a fixed destination's length (a dynamic one gets length 1), returning
// SS$_NORMAL, LIB$_STRTRU (a fixed destination of length 0, unconfirmed),
// LIB$_INVSTRDES, or LIB$_INSVIRMEM.
func libChar(env *corevms.Environment, argv []uint32) (uint32, error) {
	code, err := env.Memory().LoadByte(env.CPU(), arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	return copyString(env, string([]byte{code}), arg(argv, 0))
}

// libIchar is LIB$ICHAR (LIB-226): source-string.rt.dx. It returns the
// string's first character, zero-extended, or 0 for an empty string.
// (The manual's example says it lowercases; its own output shows it
// doesn't.)
func libIchar(env *corevms.Environment, argv []uint32) (uint32, error) {
	d, err := searchDescriptor(env, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if d.length == 0 {
		return 0, nil
	}

	ch, err := env.Memory().LoadByte(env.CPU(), d.text)
	if err != nil {
		return memoryFault(env, err)
	}

	return uint32(ch), nil
}

// libIndex is LIB$INDEX (LIB-228): source-string.rt.dx, sub-string.rt.dx.
// It returns the 1-based position of sub-string's first occurrence in
// source-string, or 0. An empty sub-string is found at 1, even in an
// empty source. It's LIB$MATCHC with its arguments the other way round.
func libIndex(env *corevms.Environment, argv []uint32) (uint32, error) {
	return libMatchc(env, []uint32{arg(argv, 1), arg(argv, 0)})
}
