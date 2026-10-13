package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// ASCII and EBCDIC (docs/PHASE-54, "Translation"): LIB$TRA_ASC_EBC and
// LIB$TRA_EBC_ASC translate a string through LIBRTL's own tables
// (tables.go), as MOVTC through LIB$AB_ASC_EBC or LIB$AB_EBC_ASC would,
// but also report characters the other code has no equivalent for.

var libInvCha = vmsdef.LibrarySymbols["LIB$_INVCHA"]

// translatable reports whether table translates c to a character the
// inverse table turns back into c: what an untranslatable character
// (ASCII 80 to FE; the EBCDIC codes outside the ASCII table's image)
// doesn't do. So ASCII 1A (SUB), which is EBCDIC 3F, the table's
// replacement character, is translatable, as is EBCDIC E0, ASCII's
// backslash, the other table's.
func translatable(table, inverse *[256]byte, c byte) bool {
	return inverse[table[c]] == c
}

// translateString is the two routines' work: the source string, through
// table, into the destination. A fixed-length destination (S, Z, SD, A,
// NCA) must be the source's length ("no filling is done"), and a varying
// one must have room for it (unconfirmed), else LIB$_INVARG with nothing
// written; a dynamic one is given the source's length. It returns
// SS$_NORMAL, LIB$_INVCHA when a character had no equivalent (the whole
// string still translated, each such one the table's replacement),
// LIB$_INVARG, LIB$_INVSTRDES, or LIB$_INSVIRMEM.
func translateString(env *corevms.Environment, argv []uint32, table, inverse *[256]byte) (uint32, error) {
	src, err := readString(env, arg(argv, 0))
	if err != nil {
		return stringFailure(env, err)
	}

	dst, err := readDescriptor(env, arg(argv, 1))
	if err != nil {
		return stringFailure(env, err)
	}

	switch dst.class {
	case dscClassD:
	case dscClassVS:
		if uint32(len(src)) > dst.limit {
			return libInvArg, nil
		}
	default:
		if uint32(len(src)) != dst.length {
			return libInvArg, nil
		}
	}

	out := make([]byte, len(src))
	status := ssNormal

	for i := range len(src) {
		out[i] = table[src[i]]

		if !translatable(table, inverse, src[i]) {
			status = libInvCha
		}
	}

	if _, _, err := writeString(env, arg(argv, 1), string(out)); err != nil {
		return stringFailure(env, err)
	}

	return status, nil
}

// libTraAscEbc is LIB$TRA_ASC_EBC (LIB-452): source-string.rt.dx,
// byte-integer-dest-string.wt.dx.
func libTraAscEbc(env *corevms.Environment, argv []uint32) (uint32, error) {
	return translateString(env, argv, &asciiToEBCDIC, &ebcdicToASCII)
}

// libTraEbcAsc is LIB$TRA_EBC_ASC (LIB-456): byte-integer-source-string.rt.dx,
// destination-string.wt.dx.
func libTraEbcAsc(env *corevms.Environment, argv []uint32) (uint32, error) {
	return translateString(env, argv, &ebcdicToASCII, &asciiToEBCDIC)
}
