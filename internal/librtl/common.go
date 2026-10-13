package librtl

import "github.com/tucats/govax/internal/corevms"

// The common area (docs/PHASE-57, "The common area"): 256 bytes of
// per-process storage that outlasts each image, a longword length and
// at most 252 characters of text, through which one image leaves text
// for the next (LIB$RUN_PROGRAM's, say). govax keeps the text in Go on
// the process (corevms.Environment.Common) rather than in P1 space,
// since a program reaches it only through these two routines. Neither
// needs a CLI: the manual's entries don't list LIB$_NOCLI (unconfirmed,
// a probe case).

// maxCommon is the most characters the common area holds: its 256
// bytes less the longword that holds the length.
const maxCommon = 256 - 4

// libPutCommon is LIB$PUT_COMMON (LIB-309):
//
//	LIB$PUT_COMMON source-string [,resultant-length]
//
// source-string.rt.dx, resultant-length.ww.r. It copies source-string
// into the common area, replacing what was there; resultant-length gets
// the number of characters copied.
//
// It returns SS$_NORMAL; LIB$_STRTRU (a success) when the string was
// longer than the area's 252 characters, of which the first 252 are
// kept; or LIB$_INVSTRDES. An argument it can't read or write is an
// access violation, signaled (memoryFault).
func libPutCommon(env *corevms.Environment, argv []uint32) (uint32, error) {
	text, err := readString(env, arg(argv, 0))
	if err != nil {
		return stringFailure(env, err)
	}

	status := ssNormal
	if len(text) > maxCommon {
		text, status = text[:maxCommon], libStrTru
	}

	if a := arg(argv, 1); a != 0 {
		if err := env.Memory().StoreWord(env.CPU(), a, uint16(len(text))); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	env.Common = text

	return status, nil
}

// libGetCommon is LIB$GET_COMMON (LIB-202):
//
//	LIB$GET_COMMON resultant-string [,resultant-length]
//
// resultant-string.wt.dx, resultant-length.ww.r. It copies the common
// area's text into resultant-string, by its class's rules (a fixed
// string padded with blanks); resultant-length gets the number of
// characters copied. Before any LIB$PUT_COMMON the area is empty
// (unconfirmed).
//
// It returns SS$_NORMAL; LIB$_STRTRU (a success) when the text didn't
// fit a fixed-length resultant-string; LIB$_INVSTRDES; or
// LIB$_INSVIRMEM. An argument it can't write is an access violation,
// signaled (memoryFault).
func libGetCommon(env *corevms.Environment, argv []uint32) (uint32, error) {
	stored, truncated, err := writeString(env, arg(argv, 0), env.Common)
	if err != nil {
		return stringFailure(env, err)
	}

	if a := arg(argv, 1); a != 0 {
		if err := env.Memory().StoreWord(env.CPU(), a, uint16(stored)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if truncated {
		return libStrTru, nil
	}

	return ssNormal, nil
}
