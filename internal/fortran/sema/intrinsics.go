package sema

import (
	"fmt"
	"strings"
)

// The intrinsic functions (section 6.3; appendix D, Table D-3), as a table
// (Decision 7 of docs/PHASE-78 - FORTRAN expressions and types.md). Each row
// is one line of Table D-3: a generic name, the specific name it selects, how
// many arguments it takes, the type of each argument and of the result, and
// how the function is compiled. A generic name has a row for each type of
// argument it takes, and selects the row by the arguments' type (section
// 6.3.1.1); a specific name has a row for each type it takes, which is
// usually one. "-" is a generic name the table doesn't give (DPROD is only a
// specific name), or a specific name it doesn't (INT of a COMPLEX has none).
//
// The types are written I2, I4 (INTEGER*2, *4), R4, R8, R16 (REAL), C8, C16
// (COMPLEX), L1, L2, L4 (LOGICAL), CH (a CHARACTER value of any length), CH1
// (CHARACTER*1), and * (anything). The counts are a number, "1,2" for one or
// two, "2+" for two or more.
//
// Where two rows of a generic name take the same type and differ only in the
// result's INTEGER length (INT of a REAL*4 is IINT or JINT), the generic
// selects the one of the unit's default integer: footnote 10's functions
// "return INTEGER*4 values if the /I4 command qualifier is in effect,
// INTEGER*2 values if the /NOI4 qualifier is in effect".
//
// How each is compiled:
//
//	MTH      a routine of MTHRTL (Phase 78's subtask 9)
//	CVT      a conversion, as assignment converts (Table 3-1; footnote 11): a
//	         COMPLEX value's real part, converted; and the value itself, for
//	         INT of an integer or REAL of a REAL*4
//	ABS, MAX, MIN, DIM, SIGN, MOD, NINT, ZEXT, IMAG, CMPLX, CONJG,
//	SHIFT, BITS, BSET, BCLR, BTEST, SHIFTC, LEN, INDEX, ICHAR, CHAR
//	         an IntrinsicRef of that operation, which lowering writes in line
//	         (or, for SHIFTC, as a call of FTNRTL's routine)
//	DPROD    a REAL*8 product of the two values made REAL*8, which is exact
//	AND, OR, XOR, NOT
//	         the operators .AND., .OR., .XOR., and .NOT., which are bit by
//	         bit on INTEGER operands (section 2.3.4)
//	LLT, LLE, LGT, LGE
//	         the relational operators on CHARACTER values, which compare in
//	         ASCII's order (section 6.3.2.2)
//	ONE      the constant 1: NWORKERS, which Decision 1 of docs/PHASE-74 -
//	         FORTRAN plan.md makes one process
//	SIZEOF   the constant size of the argument
const intrinsicTable = `
SQRT   SQRT    1 R4  R4  MTH
SQRT   DSQRT   1 R8  R8  MTH
SQRT   QSQRT   1 R16 R16 MTH
SQRT   CSQRT   1 C8  C8  MTH
SQRT   CDSQRT  1 C16 C16 MTH
LOG    ALOG    1 R4  R4  MTH
LOG    DLOG    1 R8  R8  MTH
LOG    QLOG    1 R16 R16 MTH
LOG    CLOG    1 C8  C8  MTH
LOG    CDLOG   1 C16 C16 MTH
LOG10  ALOG10  1 R4  R4  MTH
LOG10  DLOG10  1 R8  R8  MTH
LOG10  QLOG10  1 R16 R16 MTH
EXP    EXP     1 R4  R4  MTH
EXP    DEXP    1 R8  R8  MTH
EXP    QEXP    1 R16 R16 MTH
EXP    CEXP    1 C8  C8  MTH
EXP    CDEXP   1 C16 C16 MTH
SIN    SIN     1 R4  R4  MTH
SIN    DSIN    1 R8  R8  MTH
SIN    QSIN    1 R16 R16 MTH
SIN    CSIN    1 C8  C8  MTH
SIN    CDSIN   1 C16 C16 MTH
SIND   SIND    1 R4  R4  MTH
SIND   DSIND   1 R8  R8  MTH
SIND   QSIND   1 R16 R16 MTH
COS    COS     1 R4  R4  MTH
COS    DCOS    1 R8  R8  MTH
COS    QCOS    1 R16 R16 MTH
COS    CCOS    1 C8  C8  MTH
COS    CDCOS   1 C16 C16 MTH
COSD   COSD    1 R4  R4  MTH
COSD   DCOSD   1 R8  R8  MTH
COSD   QCOSD   1 R16 R16 MTH
TAN    TAN     1 R4  R4  MTH
TAN    DTAN    1 R8  R8  MTH
TAN    QTAN    1 R16 R16 MTH
TAND   TAND    1 R4  R4  MTH
TAND   DTAND   1 R8  R8  MTH
TAND   QTAND   1 R16 R16 MTH
ASIN   ASIN    1 R4  R4  MTH
ASIN   DASIN   1 R8  R8  MTH
ASIN   QASIN   1 R16 R16 MTH
ASIND  ASIND   1 R4  R4  MTH
ASIND  DASIND  1 R8  R8  MTH
ASIND  QASIND  1 R16 R16 MTH
ACOS   ACOS    1 R4  R4  MTH
ACOS   DACOS   1 R8  R8  MTH
ACOS   QACOS   1 R16 R16 MTH
ACOSD  ACOSD   1 R4  R4  MTH
ACOSD  DACOSD  1 R8  R8  MTH
ACOSD  QACOSD  1 R16 R16 MTH
ATAN   ATAN    1 R4  R4  MTH
ATAN   DATAN   1 R8  R8  MTH
ATAN   QATAN   1 R16 R16 MTH
ATAND  ATAND   1 R4  R4  MTH
ATAND  DATAND  1 R8  R8  MTH
ATAND  QATAND  1 R16 R16 MTH
ATAN2  ATAN2   2 R4  R4  MTH
ATAN2  DATAN2  2 R8  R8  MTH
ATAN2  QATAN2  2 R16 R16 MTH
ATAN2D ATAN2D  2 R4  R4  MTH
ATAN2D DATAN2D 2 R8  R8  MTH
ATAN2D QATAN2D 2 R16 R16 MTH
SINH   SINH    1 R4  R4  MTH
SINH   DSINH   1 R8  R8  MTH
SINH   QSINH   1 R16 R16 MTH
COSH   COSH    1 R4  R4  MTH
COSH   DCOSH   1 R8  R8  MTH
COSH   QCOSH   1 R16 R16 MTH
TANH   TANH    1 R4  R4  MTH
TANH   DTANH   1 R8  R8  MTH
TANH   QTANH   1 R16 R16 MTH

ABS    IIABS   1 I2  I2  ABS
ABS    JIABS   1 I4  I4  ABS
ABS    ABS     1 R4  R4  ABS
ABS    DABS    1 R8  R8  ABS
ABS    QABS    1 R16 R16 ABS
ABS    CABS    1 C8  R4  MTH
ABS    CDABS   1 C16 R8  MTH
IABS   IIABS   1 I2  I2  ABS
IABS   JIABS   1 I4  I4  ABS

INT    -       1 I2  I2  CVT
INT    -       1 I2  I4  CVT
INT    -       1 I4  I2  CVT
INT    -       1 I4  I4  CVT
INT    IINT    1 R4  I2  CVT
INT    JINT    1 R4  I4  CVT
INT    IIDINT  1 R8  I2  CVT
INT    JIDINT  1 R8  I4  CVT
INT    IIQINT  1 R16 I2  CVT
INT    JIQINT  1 R16 I4  CVT
INT    -       1 C8  I2  CVT
INT    -       1 C8  I4  CVT
INT    -       1 C16 I2  CVT
INT    -       1 C16 I4  CVT
IDINT  IIDINT  1 R8  I2  CVT
IDINT  JIDINT  1 R8  I4  CVT
IQINT  IIQINT  1 R16 I2  CVT
IQINT  JIQINT  1 R16 I4  CVT
IFIX   IIFIX   1 R4  I2  CVT
IFIX   JIFIX   1 R4  I4  CVT
AINT   AINT    1 R4  R4  MTH
AINT   DINT    1 R8  R8  MTH
AINT   QINT    1 R16 R16 MTH

NINT   ININT   1 R4  I2  NINT
NINT   JNINT   1 R4  I4  NINT
NINT   IIDNNT  1 R8  I2  NINT
NINT   JIDNNT  1 R8  I4  NINT
NINT   IIQNNT  1 R16 I2  NINT
NINT   JIQNNT  1 R16 I4  NINT
IDNINT IIDNNT  1 R8  I2  NINT
IDNINT JIDNNT  1 R8  I4  NINT
IQNINT IIQNNT  1 R16 I2  NINT
IQNINT JIQNNT  1 R16 I4  NINT
ANINT  ANINT   1 R4  R4  MTH
ANINT  DNINT   1 R8  R8  MTH
ANINT  QNINT   1 R16 R16 MTH

ZEXT   IZEXT   1 L1  I2  ZEXT
ZEXT   IZEXT   1 L2  I2  ZEXT
ZEXT   IZEXT   1 I2  I2  ZEXT
ZEXT   JZEXT   1 L1  I4  ZEXT
ZEXT   JZEXT   1 L2  I4  ZEXT
ZEXT   JZEXT   1 L4  I4  ZEXT
ZEXT   JZEXT   1 I2  I4  ZEXT
ZEXT   JZEXT   1 I4  I4  ZEXT

REAL   FLOATI  1 I2  R4  CVT
REAL   FLOATJ  1 I4  R4  CVT
REAL   -       1 R4  R4  CVT
REAL   SNGL    1 R8  R4  CVT
REAL   SNGLQ   1 R16 R4  CVT
REAL   REAL    1 C8  R4  CVT
REAL   -       1 C16 R4  CVT
-      DREAL   1 C16 R8  CVT
DBLE   -       1 I2  R8  CVT
DBLE   -       1 I4  R8  CVT
DBLE   DBLE    1 R4  R8  CVT
DBLE   -       1 R8  R8  CVT
DBLE   DBLEQ   1 R16 R8  CVT
DBLE   -       1 C8  R8  CVT
DBLE   -       1 C16 R8  CVT
QEXT   -       1 I2  R16 CVT
QEXT   -       1 I4  R16 CVT
QEXT   QEXT    1 R4  R16 CVT
QEXT   QEXTD   1 R8  R16 CVT
QEXT   -       1 R16 R16 CVT
QEXT   -       1 C8  R16 CVT
QEXT   -       1 C16 R16 CVT
FLOAT  FLOATI  1 I2  R4  CVT
FLOAT  FLOATJ  1 I4  R4  CVT
DFLOAT DFLOTI  1 I2  R8  CVT
DFLOAT DFLOTJ  1 I4  R8  CVT
QFLOAT -       1 I2  R16 CVT
QFLOAT -       1 I4  R16 CVT

CMPLX  -       1,2 I2  C8  CMPLX
CMPLX  -       1,2 I4  C8  CMPLX
CMPLX  -       1,2 R4  C8  CMPLX
CMPLX  -       1,2 R8  C8  CMPLX
CMPLX  -       1,2 R16 C8  CMPLX
CMPLX  -       1   C8  C8  CMPLX
CMPLX  -       1   C16 C8  CMPLX
DCMPLX -       1,2 I2  C16 CMPLX
DCMPLX -       1,2 I4  C16 CMPLX
DCMPLX -       1,2 R4  C16 CMPLX
DCMPLX -       1,2 R8  C16 CMPLX
DCMPLX -       1,2 R16 C16 CMPLX
DCMPLX -       1   C8  C16 CMPLX
DCMPLX -       1   C16 C16 CMPLX
-      AIMAG   1 C8  R4  IMAG
-      DIMAG   1 C16 R8  IMAG
CONJG  CONJG   1 C8  C8  CONJG
CONJG  DCONJG  1 C16 C16 CONJG
-      DPROD   2 R4  R8  DPROD

MAX    IMAX0   2+ I2  I2  MAX
MAX    JMAX0   2+ I4  I4  MAX
MAX    AMAX1   2+ R4  R4  MAX
MAX    DMAX1   2+ R8  R8  MAX
MAX    QMAX1   2+ R16 R16 MAX
MAX0   IMAX0   2+ I2  I2  MAX
MAX0   JMAX0   2+ I4  I4  MAX
MAX1   IMAX1   2+ R4  I2  MAX
MAX1   JMAX1   2+ R4  I4  MAX
AMAX0  AIMAX0  2+ I2  R4  MAX
AMAX0  AJMAX0  2+ I4  R4  MAX
MIN    IMIN0   2+ I2  I2  MIN
MIN    JMIN0   2+ I4  I4  MIN
MIN    AMIN1   2+ R4  R4  MIN
MIN    DMIN1   2+ R8  R8  MIN
MIN    QMIN1   2+ R16 R16 MIN
MIN0   IMIN0   2+ I2  I2  MIN
MIN0   JMIN0   2+ I4  I4  MIN
MIN1   IMIN1   2+ R4  I2  MIN
MIN1   JMIN1   2+ R4  I4  MIN
AMIN0  AIMIN0  2+ I2  R4  MIN
AMIN0  AJMIN0  2+ I4  R4  MIN
DIM    IIDIM   2 I2  I2  DIM
DIM    JIDIM   2 I4  I4  DIM
DIM    DIM     2 R4  R4  DIM
DIM    DDIM    2 R8  R8  DIM
DIM    QDIM    2 R16 R16 DIM
IDIM   IIDIM   2 I2  I2  DIM
IDIM   JIDIM   2 I4  I4  DIM
MOD    IMOD    2 I2  I2  MOD
MOD    JMOD    2 I4  I4  MOD
MOD    AMOD    2 R4  R4  MTH
MOD    DMOD    2 R8  R8  MTH
MOD    QMOD    2 R16 R16 MTH
SIGN   IISIGN  2 I2  I2  SIGN
SIGN   JISIGN  2 I4  I4  SIGN
SIGN   SIGN    2 R4  R4  SIGN
SIGN   DSIGN   2 R8  R8  SIGN
SIGN   QSIGN   2 R16 R16 SIGN
ISIGN  IISIGN  2 I2  I2  SIGN
ISIGN  JISIGN  2 I4  I4  SIGN

IAND   IIAND   2 I2  I2  AND
IAND   JIAND   2 I4  I4  AND
IOR    IIOR    2 I2  I2  OR
IOR    JIOR    2 I4  I4  OR
IEOR   IIEOR   2 I2  I2  XOR
IEOR   JIEOR   2 I4  I4  XOR
NOT    INOT    1 I2  I2  NOT
NOT    JNOT    1 I4  I4  NOT
ISHFT  IISHFT  2 I2  I2  SHIFT
ISHFT  JISHFT  2 I4  I4  SHIFT
IBITS  IIBITS  3 I2  I4  BITS
IBITS  JIBITS  3 I4  I4  BITS
IBSET  IIBSET  2 I2  I2  BSET
IBSET  JIBSET  2 I4  I4  BSET
BTEST  BITEST  2 I2  L2  BTEST
BTEST  BJTEST  2 I4  L4  BTEST
IBCLR  IIBCLR  2 I2  I4  BCLR
IBCLR  JIBCLR  2 I4  I4  BCLR
ISHFTC IISHFTC 3 I2  I4  SHIFTC
ISHFTC JISHFTC 3 I4  I4  SHIFTC

-      LEN      1 CH  I4  LEN
-      INDEX    2 CH  I4  INDEX
-      CHAR     1 L1  CH1 CHAR
-      CHAR     1 I2  CH1 CHAR
-      CHAR     1 I4  CH1 CHAR
-      ICHAR    1 CH  I4  ICHAR
-      LLT      2 CH  L4  LLT
-      LLE      2 CH  L4  LLE
-      LGT      2 CH  L4  LGT
-      LGE      2 CH  L4  LGE
-      NWORKERS 0 *   I4  ONE
-      SIZEOF   1 *   I4  SIZEOF
`

// form is one row of the table.
type form struct {
	generic, specific string

	// min and max are how many arguments it takes; max is -1 for any number.
	min, max int

	// arg is each argument's type: Kind Character with Size 0 for any
	// CHARACTER value, and the Invalid type for anything.
	arg, result Type

	// how is the table's last column.
	how string
}

// intrinsicForms are the table's rows, and intrinsics the names of VAX
// FORTRAN's intrinsic functions, generic and specific, the table's names.
var intrinsicForms, intrinsics = readIntrinsics()

// readIntrinsics reads intrinsicTable.
func readIntrinsics() ([]form, map[string]bool) {
	var forms []form

	names := map[string]bool{}

	for line := range strings.SplitSeq(intrinsicTable, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}

		if len(f) != 6 {
			panic("sema: an intrinsic table row of " + fmt.Sprint(len(f)) + " fields: " + line)
		}

		r := form{generic: f[0], specific: f[1], arg: tableType(f[3]), result: tableType(f[4]), how: f[5]}

		switch f[2] {
		case "1,2":
			r.min, r.max = 1, 2
		case "2+":
			r.min, r.max = 2, -1
		default:
			r.min = int(f[2][0] - '0')
			r.max = r.min
		}

		if r.generic == "-" {
			r.generic = ""
		}

		if r.specific == "-" {
			r.specific = ""
		}

		for _, name := range []string{r.generic, r.specific} {
			if name != "" {
				names[name] = true
			}
		}

		forms = append(forms, r)
	}

	return forms, names
}

// tableType is a type as the table writes it.
func tableType(text string) Type {
	switch text {
	case "*":
		return invalid
	case "CH":
		return Type{Character, 0}
	case "CH1":
		return Type{Character, 1}
	}

	kinds := map[byte]Kind{'I': Integer, 'R': Real, 'C': Complex, 'L': Logical}

	k, ok := kinds[text[0]]
	if !ok {
		panic("sema: an intrinsic table type " + text)
	}

	n := 0
	for _, c := range text[1:] {
		n = n*10 + int(c-'0')
	}

	return Type{k, n}
}

// formsNamed are the rows a reference to name selects from: a generic
// name's, if name is one (a name both generic and specific, such as ABS, is
// generic in a reference, section 6.3.1.1), or else a specific name's.
func formsNamed(name string) []form {
	var generic, specific []form

	for _, r := range intrinsicForms {
		if r.generic == name {
			generic = append(generic, r)
		}

		if r.specific == name {
			specific = append(specific, r)
		}
	}

	if len(generic) > 0 {
		return generic
	}

	return specific
}

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

// builtin is NOTIMPL's subject for a name that is one of FORTRAN's system
// functions (function set) or subroutines, or "" if it isn't.
func builtin(name string, function bool) string {
	switch {
	case function && systemFunctions[name]:
		return "the system function " + name
	case !function && systemSubroutines[name]:
		return "the system subroutine " + name
	}

	return ""
}
