package sema

import (
	"fmt"
	"strings"
)

// The intrinsic functions (section 6.3; appendix D, Table D-3), as a table
// (Decision 7 of docs/planning/PHASE-78 - FORTRAN expressions and types.md). Each row
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
//	MTH      a call of MTHRTL's routine (routines.go), with the arguments
//	         by reference
//	CVT      a conversion, as assignment converts (Table 3-1; footnote 11): a
//	         COMPLEX value's real part, converted; and the value itself, for
//	         INT of an integer or REAL of a REAL*4
//	ABS, MAX, MIN, DIM, SIGN, MOD, NINT, ZEXT, IMAG, CMPLX, CONJG,
//	SHIFT, BITS, BSET, BCLR, BTEST, LEN, INDEX, ICHAR, CHAR
//	         an IntrinsicRef of that operation, which lowering writes in line
//	         (but ISHFT by a count that isn't constant calls MTHRTL's
//	         MTH$IISHFT or MTH$JISHFT)
//	SHIFTC   a call of FTNRTL's FTN$IISHFTC or FTN$JISHFTC, since no
//	         instruction does a circular shift and MTHRTL has none
//	DPROD    a REAL*8 product of the two values made REAL*8, which is exact
//	AND, OR, XOR, NOT
//	         the operators .AND., .OR., .XOR., and .NOT., which are bit by
//	         bit on INTEGER operands (section 2.3.4)
//	LLT, LLE, LGT, LGE
//	         the relational operators on CHARACTER values, which compare in
//	         ASCII's order (section 6.3.2.2)
//	ONE      the constant 1: NWORKERS, which Decision 1 of docs/planning/PHASE-74 -
//	         FORTRAN plan.md makes one process
//	SIZEOF   the constant size of the argument
//
// Fortran 90's functions (fortran90Table, below) add three more:
//
//	INQ      an inquiry about the argument's type, answered when compiling
//	         (inquiry.go): KIND, HUGE, DIGITS, and the rest of the numeric
//	         model's
//	SIK, SRK SELECTED_INT_KIND and SELECTED_REAL_KIND, of constants
//
// The last two columns are the function's class, E elemental, I inquiry, or T
// transformational (Fortran 90's 13.2), and its dummy arguments' names
// (Decision 14 of docs/planning/PHASE-99 - Fortran 90.md), by which a reference
// may give its arguments as keywords, INT(A=X, KIND=2), IAND(I=5, J=3)
// (intrinsickeys.go). The names are the standard's 13.13, and for DEC's own
// functions the HP Fortran manual's; a specific name takes its generic's
// (docs/planning/PHASE-108 - Fortran 90 intrinsic procedures.md, finding 1).
// Every row of a generic name names the same dummies. A ? after a name makes
// the dummy optional; ... after the last says more may follow, by position
// only (MAX(A1, A2, A3, ...)); - is none (NWORKERS). A transformational or
// inquiry function's dummy is a scalar unless a rank follows it, [1] or [2] an
// array of that rank, [] an array of any rank, [*] anything: what a generic
// name's specific procedures are told apart from it by (generic.go's
// takenByIntrinsic). KIND, the kind of the result (the standard's 13.13), is
// never counted in the third column: a reference's KIND= is taken out before
// its row is chosen, and makes the result that kind.
//
// LATER is a function the phase hasn't brought yet, NOTIMPL by name: its
// argument and result types are left open (*), the rows to come.
const intrinsicTable = `
SQRT   SQRT     1   R4  R4  MTH    E X
SQRT   DSQRT    1   R8  R8  MTH    E X
SQRT   QSQRT    1   R16 R16 MTH    E X
SQRT   CSQRT    1   C8  C8  MTH    E X
SQRT   CDSQRT   1   C16 C16 MTH    E X
LOG    ALOG     1   R4  R4  MTH    E X
LOG    DLOG     1   R8  R8  MTH    E X
LOG    QLOG     1   R16 R16 MTH    E X
LOG    CLOG     1   C8  C8  MTH    E X
LOG    CDLOG    1   C16 C16 MTH    E X
LOG10  ALOG10   1   R4  R4  MTH    E X
LOG10  DLOG10   1   R8  R8  MTH    E X
LOG10  QLOG10   1   R16 R16 MTH    E X
EXP    EXP      1   R4  R4  MTH    E X
EXP    DEXP     1   R8  R8  MTH    E X
EXP    QEXP     1   R16 R16 MTH    E X
EXP    CEXP     1   C8  C8  MTH    E X
EXP    CDEXP    1   C16 C16 MTH    E X
SIN    SIN      1   R4  R4  MTH    E X
SIN    DSIN     1   R8  R8  MTH    E X
SIN    QSIN     1   R16 R16 MTH    E X
SIN    CSIN     1   C8  C8  MTH    E X
SIN    CDSIN    1   C16 C16 MTH    E X
SIND   SIND     1   R4  R4  MTH    E X
SIND   DSIND    1   R8  R8  MTH    E X
SIND   QSIND    1   R16 R16 MTH    E X
COS    COS      1   R4  R4  MTH    E X
COS    DCOS     1   R8  R8  MTH    E X
COS    QCOS     1   R16 R16 MTH    E X
COS    CCOS     1   C8  C8  MTH    E X
COS    CDCOS    1   C16 C16 MTH    E X
COSD   COSD     1   R4  R4  MTH    E X
COSD   DCOSD    1   R8  R8  MTH    E X
COSD   QCOSD    1   R16 R16 MTH    E X
TAN    TAN      1   R4  R4  MTH    E X
TAN    DTAN     1   R8  R8  MTH    E X
TAN    QTAN     1   R16 R16 MTH    E X
TAND   TAND     1   R4  R4  MTH    E X
TAND   DTAND    1   R8  R8  MTH    E X
TAND   QTAND    1   R16 R16 MTH    E X
ASIN   ASIN     1   R4  R4  MTH    E X
ASIN   DASIN    1   R8  R8  MTH    E X
ASIN   QASIN    1   R16 R16 MTH    E X
ASIND  ASIND    1   R4  R4  MTH    E X
ASIND  DASIND   1   R8  R8  MTH    E X
ASIND  QASIND   1   R16 R16 MTH    E X
ACOS   ACOS     1   R4  R4  MTH    E X
ACOS   DACOS    1   R8  R8  MTH    E X
ACOS   QACOS    1   R16 R16 MTH    E X
ACOSD  ACOSD    1   R4  R4  MTH    E X
ACOSD  DACOSD   1   R8  R8  MTH    E X
ACOSD  QACOSD   1   R16 R16 MTH    E X
ATAN   ATAN     1   R4  R4  MTH    E X
ATAN   DATAN    1   R8  R8  MTH    E X
ATAN   QATAN    1   R16 R16 MTH    E X
ATAND  ATAND    1   R4  R4  MTH    E X
ATAND  DATAND   1   R8  R8  MTH    E X
ATAND  QATAND   1   R16 R16 MTH    E X
ATAN2  ATAN2    2   R4  R4  MTH    E Y,X
ATAN2  DATAN2   2   R8  R8  MTH    E Y,X
ATAN2  QATAN2   2   R16 R16 MTH    E Y,X
ATAN2D ATAN2D   2   R4  R4  MTH    E Y,X
ATAN2D DATAN2D  2   R8  R8  MTH    E Y,X
ATAN2D QATAN2D  2   R16 R16 MTH    E Y,X
SINH   SINH     1   R4  R4  MTH    E X
SINH   DSINH    1   R8  R8  MTH    E X
SINH   QSINH    1   R16 R16 MTH    E X
COSH   COSH     1   R4  R4  MTH    E X
COSH   DCOSH    1   R8  R8  MTH    E X
COSH   QCOSH    1   R16 R16 MTH    E X
TANH   TANH     1   R4  R4  MTH    E X
TANH   DTANH    1   R8  R8  MTH    E X
TANH   QTANH    1   R16 R16 MTH    E X

ABS    IIABS    1   I2  I2  ABS    E A
ABS    JIABS    1   I4  I4  ABS    E A
ABS    ABS      1   R4  R4  ABS    E A
ABS    DABS     1   R8  R8  ABS    E A
ABS    QABS     1   R16 R16 ABS    E A
ABS    CABS     1   C8  R4  MTH    E A
ABS    CDABS    1   C16 R8  MTH    E A
IABS   IIABS    1   I2  I2  ABS    E A
IABS   JIABS    1   I4  I4  ABS    E A

INT    -        1   I2  I2  CVT    E A,KIND?
INT    -        1   I2  I4  CVT    E A,KIND?
INT    -        1   I4  I2  CVT    E A,KIND?
INT    -        1   I4  I4  CVT    E A,KIND?
INT    IINT     1   R4  I2  CVT    E A,KIND?
INT    JINT     1   R4  I4  CVT    E A,KIND?
INT    IIDINT   1   R8  I2  CVT    E A,KIND?
INT    JIDINT   1   R8  I4  CVT    E A,KIND?
INT    IIQINT   1   R16 I2  CVT    E A,KIND?
INT    JIQINT   1   R16 I4  CVT    E A,KIND?
INT    -        1   C8  I2  CVT    E A,KIND?
INT    -        1   C8  I4  CVT    E A,KIND?
INT    -        1   C16 I2  CVT    E A,KIND?
INT    -        1   C16 I4  CVT    E A,KIND?
IDINT  IIDINT   1   R8  I2  CVT    E A
IDINT  JIDINT   1   R8  I4  CVT    E A
IQINT  IIQINT   1   R16 I2  CVT    E A
IQINT  JIQINT   1   R16 I4  CVT    E A
IFIX   IIFIX    1   R4  I2  CVT    E A
IFIX   JIFIX    1   R4  I4  CVT    E A
AINT   AINT     1   R4  R4  MTH    E A,KIND?
AINT   DINT     1   R8  R8  MTH    E A,KIND?
AINT   QINT     1   R16 R16 MTH    E A,KIND?

NINT   ININT    1   R4  I2  NINT   E A,KIND?
NINT   JNINT    1   R4  I4  NINT   E A,KIND?
NINT   IIDNNT   1   R8  I2  NINT   E A,KIND?
NINT   JIDNNT   1   R8  I4  NINT   E A,KIND?
NINT   IIQNNT   1   R16 I2  NINT   E A,KIND?
NINT   JIQNNT   1   R16 I4  NINT   E A,KIND?
IDNINT IIDNNT   1   R8  I2  NINT   E A
IDNINT JIDNNT   1   R8  I4  NINT   E A
IQNINT IIQNNT   1   R16 I2  NINT   E A
IQNINT JIQNNT   1   R16 I4  NINT   E A
ANINT  ANINT    1   R4  R4  MTH    E A,KIND?
ANINT  DNINT    1   R8  R8  MTH    E A,KIND?
ANINT  QNINT    1   R16 R16 MTH    E A,KIND?

ZEXT   IZEXT    1   L1  I2  ZEXT   E X
ZEXT   IZEXT    1   L2  I2  ZEXT   E X
ZEXT   IZEXT    1   I2  I2  ZEXT   E X
ZEXT   JZEXT    1   L1  I4  ZEXT   E X
ZEXT   JZEXT    1   L2  I4  ZEXT   E X
ZEXT   JZEXT    1   L4  I4  ZEXT   E X
ZEXT   JZEXT    1   I2  I4  ZEXT   E X
ZEXT   JZEXT    1   I4  I4  ZEXT   E X

REAL   FLOATI   1   I2  R4  CVT    E A,KIND?
REAL   FLOATJ   1   I4  R4  CVT    E A,KIND?
REAL   -        1   R4  R4  CVT    E A,KIND?
REAL   SNGL     1   R8  R4  CVT    E A,KIND?
REAL   SNGLQ    1   R16 R4  CVT    E A,KIND?
REAL   REAL     1   C8  R4  CVT    E A,KIND?
REAL   -        1   C16 R4  CVT    E A,KIND?
-      DREAL    1   C16 R8  CVT    E A
DBLE   -        1   I2  R8  CVT    E A
DBLE   -        1   I4  R8  CVT    E A
DBLE   DBLE     1   R4  R8  CVT    E A
DBLE   -        1   R8  R8  CVT    E A
DBLE   DBLEQ    1   R16 R8  CVT    E A
DBLE   -        1   C8  R8  CVT    E A
DBLE   -        1   C16 R8  CVT    E A
QEXT   -        1   I2  R16 CVT    E A
QEXT   -        1   I4  R16 CVT    E A
QEXT   QEXT     1   R4  R16 CVT    E A
QEXT   QEXTD    1   R8  R16 CVT    E A
QEXT   -        1   R16 R16 CVT    E A
QEXT   -        1   C8  R16 CVT    E A
QEXT   -        1   C16 R16 CVT    E A
FLOAT  FLOATI   1   I2  R4  CVT    E A
FLOAT  FLOATJ   1   I4  R4  CVT    E A
DFLOAT DFLOTI   1   I2  R8  CVT    E A
DFLOAT DFLOTJ   1   I4  R8  CVT    E A
QFLOAT -        1   I2  R16 CVT    E A
QFLOAT -        1   I4  R16 CVT    E A

CMPLX  -        1,2 I2  C8  CMPLX  E X,Y?,KIND?
CMPLX  -        1,2 I4  C8  CMPLX  E X,Y?,KIND?
CMPLX  -        1,2 R4  C8  CMPLX  E X,Y?,KIND?
CMPLX  -        1,2 R8  C8  CMPLX  E X,Y?,KIND?
CMPLX  -        1,2 R16 C8  CMPLX  E X,Y?,KIND?
CMPLX  -        1   C8  C8  CMPLX  E X,Y?,KIND?
CMPLX  -        1   C16 C8  CMPLX  E X,Y?,KIND?
DCMPLX -        1,2 I2  C16 CMPLX  E X,Y?
DCMPLX -        1,2 I4  C16 CMPLX  E X,Y?
DCMPLX -        1,2 R4  C16 CMPLX  E X,Y?
DCMPLX -        1,2 R8  C16 CMPLX  E X,Y?
DCMPLX -        1,2 R16 C16 CMPLX  E X,Y?
DCMPLX -        1   C8  C16 CMPLX  E X,Y?
DCMPLX -        1   C16 C16 CMPLX  E X,Y?
-      AIMAG    1   C8  R4  IMAG   E Z
-      DIMAG    1   C16 R8  IMAG   E Z
CONJG  CONJG    1   C8  C8  CONJG  E Z
CONJG  DCONJG   1   C16 C16 CONJG  E Z
-      DPROD    2   R4  R8  DPROD  E X,Y

MAX    IMAX0    2+  I2  I2  MAX    E A1,A2,...
MAX    JMAX0    2+  I4  I4  MAX    E A1,A2,...
MAX    AMAX1    2+  R4  R4  MAX    E A1,A2,...
MAX    DMAX1    2+  R8  R8  MAX    E A1,A2,...
MAX    QMAX1    2+  R16 R16 MAX    E A1,A2,...
MAX0   IMAX0    2+  I2  I2  MAX    E A1,A2,...
MAX0   JMAX0    2+  I4  I4  MAX    E A1,A2,...
MAX1   IMAX1    2+  R4  I2  MAX    E A1,A2,...
MAX1   JMAX1    2+  R4  I4  MAX    E A1,A2,...
AMAX0  AIMAX0   2+  I2  R4  MAX    E A1,A2,...
AMAX0  AJMAX0   2+  I4  R4  MAX    E A1,A2,...
MIN    IMIN0    2+  I2  I2  MIN    E A1,A2,...
MIN    JMIN0    2+  I4  I4  MIN    E A1,A2,...
MIN    AMIN1    2+  R4  R4  MIN    E A1,A2,...
MIN    DMIN1    2+  R8  R8  MIN    E A1,A2,...
MIN    QMIN1    2+  R16 R16 MIN    E A1,A2,...
MIN0   IMIN0    2+  I2  I2  MIN    E A1,A2,...
MIN0   JMIN0    2+  I4  I4  MIN    E A1,A2,...
MIN1   IMIN1    2+  R4  I2  MIN    E A1,A2,...
MIN1   JMIN1    2+  R4  I4  MIN    E A1,A2,...
AMIN0  AIMIN0   2+  I2  R4  MIN    E A1,A2,...
AMIN0  AJMIN0   2+  I4  R4  MIN    E A1,A2,...
DIM    IIDIM    2   I2  I2  DIM    E X,Y
DIM    JIDIM    2   I4  I4  DIM    E X,Y
DIM    DIM      2   R4  R4  DIM    E X,Y
DIM    DDIM     2   R8  R8  DIM    E X,Y
DIM    QDIM     2   R16 R16 DIM    E X,Y
IDIM   IIDIM    2   I2  I2  DIM    E X,Y
IDIM   JIDIM    2   I4  I4  DIM    E X,Y
MOD    IMOD     2   I2  I2  MOD    E A,P
MOD    JMOD     2   I4  I4  MOD    E A,P
MOD    AMOD     2   R4  R4  MTH    E A,P
MOD    DMOD     2   R8  R8  MTH    E A,P
MOD    QMOD     2   R16 R16 MTH    E A,P
SIGN   IISIGN   2   I2  I2  SIGN   E A,B
SIGN   JISIGN   2   I4  I4  SIGN   E A,B
SIGN   SIGN     2   R4  R4  SIGN   E A,B
SIGN   DSIGN    2   R8  R8  SIGN   E A,B
SIGN   QSIGN    2   R16 R16 SIGN   E A,B
ISIGN  IISIGN   2   I2  I2  SIGN   E A,B
ISIGN  JISIGN   2   I4  I4  SIGN   E A,B

IAND   IIAND    2   I2  I2  AND    E I,J
IAND   JIAND    2   I4  I4  AND    E I,J
IOR    IIOR     2   I2  I2  OR     E I,J
IOR    JIOR     2   I4  I4  OR     E I,J
IEOR   IIEOR    2   I2  I2  XOR    E I,J
IEOR   JIEOR    2   I4  I4  XOR    E I,J
NOT    INOT     1   I2  I2  NOT    E I
NOT    JNOT     1   I4  I4  NOT    E I
ISHFT  IISHFT   2   I2  I2  SHIFT  E I,SHIFT
ISHFT  JISHFT   2   I4  I4  SHIFT  E I,SHIFT
IBITS  IIBITS   3   I2  I4  BITS   E I,POS,LEN
IBITS  JIBITS   3   I4  I4  BITS   E I,POS,LEN
IBSET  IIBSET   2   I2  I2  BSET   E I,POS
IBSET  JIBSET   2   I4  I4  BSET   E I,POS
BTEST  BITEST   2   I2  L2  BTEST  E I,POS
BTEST  BJTEST   2   I4  L4  BTEST  E I,POS
IBCLR  IIBCLR   2   I2  I4  BCLR   E I,POS
IBCLR  JIBCLR   2   I4  I4  BCLR   E I,POS
ISHFTC IISHFTC  2,3 I2  I4  SHIFTC E I,SHIFT,SIZE?
ISHFTC JISHFTC  2,3 I4  I4  SHIFTC E I,SHIFT,SIZE?

-      LEN      1   CH  I4  LEN    I STRING
-      INDEX    2,3 CH  I4  INDEX  E STRING,SUBSTRING,BACK?
-      CHAR     1   L1  CH1 CHAR   E I,KIND?
-      CHAR     1   I2  CH1 CHAR   E I,KIND?
-      CHAR     1   I4  CH1 CHAR   E I,KIND?
-      ICHAR    1   CH  I4  ICHAR  E C
-      LLT      2   CH  L4  LLT    E STRING_A,STRING_B
-      LLE      2   CH  L4  LLE    E STRING_A,STRING_B
-      LGT      2   CH  L4  LGT    E STRING_A,STRING_B
-      LGE      2   CH  L4  LGE    E STRING_A,STRING_B
-      NWORKERS 0   *   I4  ONE    I -
-      SIZEOF   1   *   I4  SIZEOF I X
`

// fortran90Table is Fortran 90's intrinsic functions (the standard's 13.13)
// that Table D-3 hasn't, in the same columns, as far as the phases have
// brought them: LOGICAL, and the kind and numeric inquiry functions
// (docs/planning/PHASE-100 - Fortran 90 declarations and kinds.md, subtask
// 6). Their names are intrinsic under /F90, and otherwise only when an
// INTRINSIC statement names them (Decision 2 of docs/planning/PHASE-99 -
// Fortran 90.md), so an old program's own KIND or RANGE is still called. A
// result of * is the argument's type. LOGICAL's rows give the default
// LOGICAL, as INT's give the default INTEGER.
//
// The rows after the inquiries take a COMPLEX*32, Fortran 90's COMPLEX(16)
// (subtask 7), which Table D-3 can't: each is a conversion or a part the
// compiler writes in line. REAL of one is its real part, a REAL*16, as the
// standard has it (13.13.84: of a COMPLEX, the real kind of its own kind),
// where Table D-3's REAL of a COMPLEX*16 is REAL*4; AIMAG is made generic for
// it, its COMPLEX*8 row the specific AIMAG's. The functions of a COMPLEX(16)
// that MTHRTL would compute (ABS, SQRT, EXP, ...) are NOTIMPL until Phases
// 108 and 109 (the author's answer 1 to the survey).
const fortran90Table = `
LOGICAL            -  1   L1  L2  CVT E L,KIND?
LOGICAL            -  1   L1  L4  CVT E L,KIND?
LOGICAL            -  1   L2  L2  CVT E L,KIND?
LOGICAL            -  1   L2  L4  CVT E L,KIND?
LOGICAL            -  1   L4  L2  CVT E L,KIND?
LOGICAL            -  1   L4  L4  CVT E L,KIND?

KIND               -  1   *   I4  INQ I X[*]
SELECTED_INT_KIND  -  1   I4  I4  SIK T R
SELECTED_REAL_KIND -  0,2 I4  I4  SRK T P?,R?
HUGE               -  1   *   *   INQ I X[*]
TINY               -  1   *   *   INQ I X[*]
EPSILON            -  1   *   *   INQ I X[*]
DIGITS             -  1   *   I4  INQ I X[*]
PRECISION          -  1   *   I4  INQ I X[*]
RANGE              -  1   *   I4  INQ I X[*]
RADIX              -  1   *   I4  INQ I X[*]
MAXEXPONENT        -  1   *   I4  INQ I X[*]
MINEXPONENT        -  1   *   I4  INQ I X[*]
BIT_SIZE           -  1   *   *   INQ I I[*]

INT                -  1   C32 I2  CVT E A,KIND?
INT                -  1   C32 I4  CVT E A,KIND?
REAL               -  1   C32 R16 CVT E A,KIND?
DBLE               -  1   C32 R8  CVT E A
QEXT               -  1   C32 R16 CVT E A
CMPLX              -  1   C32 C8  CMPLX E X,Y?,KIND?
DCMPLX             -  1   C32 C16 CMPLX E X,Y?
AIMAG              -  1   C8  R4  IMAG E Z
AIMAG              -  1   C32 R16 IMAG E Z
CONJG              -  1   C32 C32 CONJG E Z

MERGE              -  3   *   *   MERGE E TSOURCE,FSOURCE,MASK

SUM                -  1,3 *   *   REDUCE T ARRAY[],DIM?,MASK?[*]
PRODUCT            -  1,3 *   *   REDUCE T ARRAY[],DIM?,MASK?[*]
MAXVAL             -  1,3 *   *   REDUCE T ARRAY[],DIM?,MASK?[*]
MINVAL             -  1,3 *   *   REDUCE T ARRAY[],DIM?,MASK?[*]
ALL                -  1,2 *   *   REDUCE T MASK[],DIM?
ANY                -  1,2 *   *   REDUCE T MASK[],DIM?
COUNT              -  1,2 *   I4  REDUCE T MASK[],DIM?
DOT_PRODUCT        -  2   *   *   REDUCE T VECTOR_A[1],VECTOR_B[1]
MAXLOC             -  1,2 *   I4  REDUCE T ARRAY[],MASK?[*]
MINLOC             -  1,2 *   I4  REDUCE T ARRAY[],MASK?[*]
MATMUL             -  2   *   *   REDUCE T MATRIX_A[],MATRIX_B[]
TRANSPOSE          -  1   *   *   REDUCE T MATRIX[2]
SPREAD             -  3   *   *   REDUCE T SOURCE[*],DIM,NCOPIES
CSHIFT             -  2,3 *   *   REDUCE T ARRAY[],SHIFT[*],DIM?
EOSHIFT            -  2,4 *   *   REDUCE T ARRAY[],SHIFT[*],BOUNDARY?[*],DIM?
PACK               -  2,3 *   *   REDUCE T ARRAY[],MASK[*],VECTOR?[1]
UNPACK             -  3   *   *   REDUCE T VECTOR[1],MASK[],FIELD[*]

RESHAPE            -  2,4 *   *   ARRAY T SOURCE[],SHAPE[1],PAD?[],ORDER?[1]
SHAPE              -  1   *   I4  ARRAY I SOURCE[*]
SIZE               -  1,2 *   I4  ARRAY I ARRAY[],DIM?
LBOUND             -  1,2 *   I4  ARRAY I ARRAY[],DIM?
UBOUND             -  1,2 *   I4  ARRAY I ARRAY[],DIM?

ACHAR              -  1   I2  CH1 CHAR E I
ACHAR              -  1   I4  CH1 CHAR E I
IACHAR             -  1   CH  I4  ICHAR E C
ADJUSTL            -  1   CH  *   LATER E STRING
ADJUSTR            -  1   CH  *   LATER E STRING
LEN_TRIM           -  1   CH  *   LATER E STRING
SCAN               -  2,3 CH  *   LATER E STRING,SET,BACK?
VERIFY             -  2,3 CH  *   LATER E STRING,SET,BACK?
TRIM               -  1   CH  *   LATER T STRING
REPEAT             -  2   CH  *   LATER T STRING,NCOPIES

CEILING            -  1   *   *   LATER E A
FLOOR              -  1   *   *   LATER E A
MODULO             -  2   *   *   LATER E A,P

EXPONENT           -  1   *   *   LATER E X
FRACTION           -  1   *   *   LATER E X
NEAREST            -  2   *   *   LATER E X,S
RRSPACING          -  1   *   *   LATER E X
SCALE              -  2   *   *   LATER E X,I
SET_EXPONENT       -  2   *   *   LATER E X,I
SPACING            -  1   *   *   LATER E X

TRANSFER           -  2,3 *   *   LATER T SOURCE[*],MOLD[*],SIZE?
`

// form is one row of the table.
type form struct {
	generic, specific string

	// min and max are how many arguments it takes, KIND not counted; max is
	// -1 for any number.
	min, max int

	// arg is each argument's type: Kind Character with Size 0 for any
	// CHARACTER value, and the Invalid type for anything.
	arg, result Type

	// how is the table's sixth column.
	how string

	// class is the seventh, 'E', 'I', or 'T'; dummies the eighth, the dummy
	// arguments in order (empty for none), and variadic says more may follow
	// them by position.
	class    byte
	dummies  []dummyArg
	variadic bool
}

// dummyArg is an intrinsic procedure's dummy argument: its name, which a
// keyword argument gives, whether a reference may leave it out, and its rank,
// as generic.go's dummyShape has it (-1 any, -2 any array; 0, a scalar, for
// an elemental function's, which take arrays of any rank by being
// elemental).
type dummyArg struct {
	name     string
	optional bool
	rank     int
}

// intrinsicForms are the tables' rows; intrinsics the names of VAX FORTRAN's
// intrinsic functions, generic and specific, Table D-3's names; and
// f90Intrinsics the names only Fortran 90 has, fortran90Table's.
var intrinsicForms, intrinsics, f90Intrinsics = readIntrinsics()

// readIntrinsics reads intrinsicTable and fortran90Table.
func readIntrinsics() ([]form, map[string]bool, map[string]bool) {
	forms, names := readTable(intrinsicTable, nil)
	more, newNames := readTable(fortran90Table, names)

	// PRESENT has no row: what it is given is an OPTIONAL dummy argument's
	// name, never a value (optional.go).
	newNames["PRESENT"] = true

	// Nor has ALLOCATED, given an allocatable array's name (allocate.go).
	newNames["ALLOCATED"] = true

	// Nor has ASSOCIATED, given a pointer itself (pointer.go).
	newNames["ASSOCIATED"] = true

	return append(forms, more...), names, newNames
}

// readTable reads one table's rows, and the names in it that old (if not
// nil) hasn't.
func readTable(table string, old map[string]bool) ([]form, map[string]bool) {
	var forms []form

	names := map[string]bool{}

	for line := range strings.SplitSeq(table, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}

		if len(f) != 8 {
			panic("sema: an intrinsic table row of " + fmt.Sprint(len(f)) + " fields: " + line)
		}

		r := form{generic: f[0], specific: f[1], arg: tableType(f[3]), result: tableType(f[4]), how: f[5]}

		switch lo, hi, found := strings.Cut(f[2], ","); {
		case found:
			r.min, r.max = int(lo[0]-'0'), int(hi[0]-'0')
		case f[2] == "2+":
			r.min, r.max = 2, -1
		default:
			r.min = int(f[2][0] - '0')
			r.max = r.min
		}

		r.class = f[6][0]
		r.dummies, r.variadic = readDummies(f[7])

		if r.generic == "-" {
			r.generic = ""
		}

		if r.specific == "-" {
			r.specific = ""
		}

		for _, name := range []string{r.generic, r.specific} {
			if name != "" && !old[name] {
				names[name] = true
			}
		}

		forms = append(forms, r)
	}

	return forms, names
}

// readDummies reads a row's last column: the dummies, and whether more may
// follow by position.
func readDummies(text string) ([]dummyArg, bool) {
	dummies := []dummyArg{}

	if text == "-" {
		return dummies, false
	}

	variadic := false

	for name := range strings.SplitSeq(text, ",") {
		if name == "..." {
			variadic = true

			continue
		}

		d := dummyArg{}

		if base, shape, ok := strings.Cut(name, "["); ok {
			switch shape = strings.TrimSuffix(shape, "]"); shape {
			case "*":
				d.rank = -1
			case "":
				d.rank = -2
			default:
				d.rank = int(shape[0] - '0')
			}

			name = base
		}

		d.name = strings.TrimSuffix(name, "?")
		d.optional = d.name != name
		dummies = append(dummies, d)
	}

	return dummies, variadic
}

// intrinsicNamed reports whether name, where the unit hasn't made it
// something else, is an intrinsic function's: one of Table D-3's, or one of
// Fortran 90's under /F90 or named in an INTRINSIC statement (Decision 2 of
// docs/planning/PHASE-99 - Fortran 90.md).
func (a *analyzer) intrinsicNamed(name string) bool {
	switch {
	case intrinsics[name]:
		return true
	case !f90Intrinsics[name]:
		return false
	case a.f90:
		return true
	}

	s := a.scope.find(name)

	return s != nil && s.Class == Intrinsic
}

// tableType is a type as the table writes it.
func tableType(text string) Type {
	switch text {
	case "*":
		return invalid
	case "CH":
		return Type{Kind: Character, Size: 0}
	case "CH1":
		return Type{Kind: Character, Size: 1}
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

	return Type{Kind: k, Size: n}
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

// setOf is the set of the blank-separated names in list.
func setOf(list string) map[string]bool {
	set := map[string]bool{}

	for _, name := range strings.Fields(list) {
		set[name] = true
	}

	return set
}
