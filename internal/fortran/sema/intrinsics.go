package sema

import "strings"

// intrinsics are the names of VAX FORTRAN's intrinsic functions, generic
// and specific (appendix D, Table D-3). A name the unit doesn't declare
// as an array or EXTERNAL that is used as a function and is
// one of these is the intrinsic function, which Phase 78 compiles; until
// then it is NOTIMPL rather than taken for an external function the
// program doesn't have.
var intrinsics = setOf(`
	SQRT DSQRT QSQRT CSQRT CDSQRT
	LOG ALOG DLOG QLOG CLOG CDLOG LOG10 ALOG10 DLOG10 QLOG10
	EXP DEXP QEXP CEXP CDEXP
	SIN DSIN QSIN CSIN CDSIN SIND DSIND QSIND
	COS DCOS QCOS CCOS CDCOS COSD DCOSD QCOSD
	TAN DTAN QTAN TAND DTAND QTAND
	ASIN DASIN QASIN ASIND DASIND QASIND
	ACOS DACOS QACOS ACOSD DACOSD QACOSD
	ATAN DATAN QATAN ATAND DATAND QATAND
	ATAN2 DATAN2 QATAN2 ATAN2D DATAN2D QATAN2D
	SINH DSINH QSINH COSH DCOSH QCOSH TANH DTANH QTANH
	ABS IIABS JIABS DABS QABS CABS CDABS IABS
	INT IINT JINT IIDINT JIDINT IIQINT JIQINT IDINT IQINT AINT DINT QINT
	NINT ININT JNINT IIDNNT JIDNNT IIQNNT JIQNNT IDNINT IQNINT ANINT DNINT QNINT
	ZEXT IZEXT JZEXT
	REAL FLOATI FLOATJ SNGL SNGLQ DBLE DBLEQ QEXT QEXTD
	IFIX IIFIX JIFIX FLOAT DFLOAT DFLOTI DFLOTJ QFLOAT
	CMPLX DCMPLX DREAL AIMAG DIMAG CONJG DCONJG DPROD
	MAX MAX0 IMAX0 JMAX0 AMAX1 DMAX1 QMAX1 MAX1 IMAX1 JMAX1 AMAX0 AIMAX0 AJMAX0
	MIN MIN0 IMIN0 JMIN0 AMIN1 DMIN1 QMIN1 MIN1 IMIN1 JMIN1 AMIN0 AIMIN0 AJMIN0
	DIM IIDIM JIDIM DDIM QDIM IDIM
	MOD IMOD JMOD AMOD DMOD QMOD
	SIGN IISIGN JISIGN DSIGN QSIGN ISIGN
	IAND IIAND JIAND IOR IIOR JIOR IEOR IIEOR JIEOR NOT INOT JNOT
	ISHFT IISHFT JISHFT IBITS IIBITS JIBITS IBSET IIBSET JIBSET
	BTEST BITEST BJTEST IBCLR IIBCLR JIBCLR ISHFTC IISHFTC JISHFTC
	LEN INDEX CHAR ICHAR LLT LLE LGT LGE NWORKERS SIZEOF
`)

// systemFunctions and systemSubroutines are the system subroutines of
// appendix D.4, which FORTRAN's run-time library provides (Phase 83).
var (
	systemFunctions   = setOf("RAN SECNDS")
	systemSubroutines = setOf("DATE IDATE ERRSNS EXIT TIME MVBITS")
)

// setOf is the set of the blank-separated names in list.
func setOf(list string) map[string]bool {
	set := map[string]bool{}

	for _, name := range strings.Fields(list) {
		set[name] = true
	}

	return set
}

// builtin is NOTIMPL's subject for a name that is one of FORTRAN's own
// functions (function set) or subroutines, or "" if it isn't.
func builtin(name string, function bool) string {
	switch {
	case function && intrinsics[name]:
		return "the intrinsic function " + name
	case function && systemFunctions[name]:
		return "the system function " + name
	case !function && systemSubroutines[name]:
		return "the system subroutine " + name
	}

	return ""
}
