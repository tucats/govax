package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// Intrinsic functions' names passed as actual arguments (docs/PHASE-78 -
// FORTRAN expressions and types.md, subtask 9): a FORTRAN program passes
// MTHRTL's routines and each of FTNRTL's for the names MTHRTL lacks to a
// MACRO routine that calls the routine it is given, and prints each value
// beside the function's value written in line, which must agree. Calling
// through a FORTRAN dummy argument is Phase 79's.

// ftnForward is the MACRO routine: each entry, one per FORTRAN type of
// value, calls its first argument with the rest of its arguments, as they
// are (their addresses), and returns what that returns in R0 and R1.
// SCALL, a subroutine, calls a function whose value goes through a hidden
// first argument.
const ftnForward = `	.TITLE	CALLF
	.PSECT	CALLF_CODE,NOWRT,EXE,LONG
	.ENTRY	ICALL,^M<R2>
	BRB	FORWARD
	.ENTRY	I2CALL,^M<R2>
	BRB	FORWARD
	.ENTRY	RCALL,^M<R2>
	BRB	FORWARD
	.ENTRY	DCALL,^M<R2>
	BRB	FORWARD
	.ENTRY	CCALL,^M<R2>
	BRB	FORWARD
	.ENTRY	LCALL,^M<R2>
	BRB	FORWARD
	.ENTRY	L2CALL,^M<R2>
	BRB	FORWARD
	.ENTRY	SCALL,^M<R2>
FORWARD:
	MOVZBL	(AP),R1			; the arguments, less the routine
	DECL	R1
	MOVL	R1,R2
	BEQL	20$
10$:	PUSHL	4(AP)[R2]		; the last first
	SOBGTR	R2,10$
20$:	CALLS	R1,@4(AP)
	RET
	.END
`

// ftnPassedProgram passes the names. Each PRINT shows the function's
// value in line, then through the routine passed.
const ftnPassedProgram = `      PROGRAM PASSED
      INTRINSIC SIN, DSQRT, QSQRT, CSQRT, IABS, MAX, MOD, INT, CMPLX
      INTRINSIC LEN, INDEX, ICHAR, CHAR, LLT, LLE, LGT, LGE
      INTRINSIC BITEST, BJTEST, IIBITS, JIBITS, IIBSET, JIBSET
      INTRINSIC IIBCLR, JIBCLR, IISHFTC, JISHFTC, IZEXT, JZEXT
      INTRINSIC SNGLQ, DBLEQ, QEXT, QEXTD, QFLOAT, NWORKERS
      INTEGER*2 I2CALL, I2, J2, K2
      DOUBLE PRECISION DCALL, D
      COMPLEX CCALL
      LOGICAL LCALL
      LOGICAL*2 L2CALL
      REAL*16 Q, QR
      CHARACTER*8 S
      CHARACTER*1 T
C     MTHRTL's routines: a value in R0, in R0 and R1, through a hidden
C     argument; generic names passed as their specific names
      X = 1.0
      PRINT *, SIN(X), RCALL(SIN, X)
      D = 2D0
      PRINT *, DSQRT(D), DCALL(DSQRT, D)
      Q = 2
      CALL SCALL(QSQRT, QR, Q)
      PRINT *, QSQRT(Q)
      PRINT *, QR
      PRINT *, CSQRT((3.0, 4.0)), CCALL(CSQRT, (3.0, 4.0))
      PRINT *, IABS(-5), ICALL(IABS, -5), MAX(3, 7), ICALL(MAX, 3, 7)
      PRINT *, MOD(17, 5), ICALL(MOD, 17, 5), INT(-2.5),
     1  ICALL(INT, -2.5)
      PRINT *, CMPLX(1.0, 2.0), CCALL(CMPLX, 1.0, 2.0)
C     FTNRTL's: the character functions
      S = 'ABCDEF'
      PRINT *, LEN(S), ICALL(LEN, S), INDEX(S, 'CD'),
     1  ICALL(INDEX, S, 'CD')
      PRINT *, INDEX(S, 'X'), ICALL(INDEX, S, 'X'), ICHAR(S),
     1  ICALL(ICHAR, S)
      CALL SCALL(CHAR, T, 73)
      PRINT *, CHAR(72), T
      PRINT *, LLT(S, 'ABD'), LCALL(LLT, S, 'ABD'), LLE(S, 'ABC'),
     1  LCALL(LLE, S, 'ABC')
      PRINT *, LGT(S, 'ABC'), LCALL(LGT, S, 'ABC'), LGE('A', 'B'),
     1  LCALL(LGE, 'A', 'B')
C     The bit functions
      I2 = 10
      J2 = 3
      PRINT *, BTEST(I2, J2), L2CALL(BITEST, I2, J2), BTEST(10, 2),
     1  LCALL(BJTEST, 10, 2)
      PRINT *, IBITS(I2, 1, 3), ICALL(IIBITS, I2, 1, 3),
     1  IBITS(-1, 4, 8), ICALL(JIBITS, -1, 4, 8)
      PRINT *, IBSET(I2, 0), I2CALL(IIBSET, I2, 0), IBSET(0, 31),
     1  ICALL(JIBSET, 0, 31)
      PRINT *, IBCLR(I2, 1), ICALL(IIBCLR, I2, 1), IBCLR(-1, 0),
     1  ICALL(JIBCLR, -1, 0)
      PRINT *, ISHFTC(I2, 1, 4), ICALL(IISHFTC, I2, 1, 4),
     1  ISHFTC(1, -1, 32), ICALL(JISHFTC, 1, -1, 32)
      K2 = -1
      PRINT *, ZEXT(K2), ICALL(JZEXT, K2), IZEXT(K2), I2CALL(IZEXT, K2)
C     REAL*16 conversions, the widening ones through a hidden argument
      Q = 1Q0/3Q0
      PRINT *, SNGLQ(Q), RCALL(SNGLQ, Q), DBLEQ(Q), DCALL(DBLEQ, Q)
      X = 0.1
      CALL SCALL(QEXT, QR, X)
      PRINT *, QEXT(X)
      PRINT *, QR
      D = 0.1D0
      CALL SCALL(QEXTD, QR, D)
      PRINT *, QEXTD(D)
      PRINT *, QR
      I = 7
      CALL SCALL(QFLOAT, QR, I)
      PRINT *, QFLOAT(I)
      PRINT *, QR
      PRINT *, NWORKERS(), ICALL(NWORKERS)
      CALL GUNIT
      END
C
      OPTIONS /G_FLOATING
      SUBROUTINE GUNIT
      INTRINSIC DSQRT, DBLEQ, QEXTD
      DOUBLE PRECISION DCALL, D
      REAL*16 Q, QR
      D = 2D0
      PRINT *, DSQRT(D), DCALL(DSQRT, D)
      Q = 1Q0/3Q0
      PRINT *, DBLEQ(Q), DCALL(DBLEQ, Q)
      D = 0.1D0
      CALL SCALL(QEXTD, QR, D)
      PRINT *, QEXTD(D)
      PRINT *, QR
      END
`

// ftnPassedRun is what PASSED prints, worked by hand: each value twice.
// The REAL*16 values of 0.1 are its REAL*4's, D_floating's, and
// G_floating's bits, exactly.
var ftnPassedRun = []string{
	"  0.8414710      0.8414710",
	"   1.414213562373095       1.414213562373095",
	"   1.41421356237309504880168872420970",
	"   1.41421356237309504880168872420970",
	" (2.000000,1.000000) (2.000000,1.000000)",
	"           5           5           7           7",
	"           2           2          -2          -2",
	" (1.000000,2.000000) (1.000000,2.000000)",
	"           8           8           3           3",
	"           0           0          65          65",
	"HI",
	" T T F F",
	" T T F F",
	" T T F F",
	"           5           5         255         255",
	"     11     11 -2147483648 -2147483648",
	"           8           8          -2          -2",
	"           5           5 -2147483648 -2147483648",
	"       65535       65535     -1     -1",
	"  0.3333333      0.3333333      0.3333333333333333      0.3333333333333333",
	"  0.100000001490116119384765625000000",
	"  0.100000001490116119384765625000000",
	"  0.100000000000000000346944695195361",
	"  0.100000000000000000346944695195361",
	"   7.00000000000000000000000000000000",
	"   7.00000000000000000000000000000000",
	"           1           1",
	"   1.41421356237310        1.41421356237310",
	"  0.333333333333333       0.333333333333333",
	"  0.100000000000000005551115123125783",
	"  0.100000000000000005551115123125783",
	`  $STATUS == "%X00000001"`,
}

func TestFortranPassedIntrinsics(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		for name, text := range map[string]string{"PASSED.FOR": ftnPassedProgram, "CALLF.MAR": ftnForward} {
			if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords, splitSource(text)); err != nil {
				t.Fatal(err)
			}
		}

		shareFindRun(t, d, "FORTRAN PASSED")
		shareFindRun(t, d, "MACRO PASSED")
		shareFindRun(t, d, "MACRO CALLF")
		shareFindRun(t, d, "LINK PASSED,CALLF")

		if out := buf.String(); out != "" {
			t.Fatalf("FORTRAN, MACRO, and LINK said:\n%s", out)
		}

		var got []string
		for _, line := range strings.Split(strings.TrimSuffix(shareRun(t, d, buf, "PASSED"), "\n"), "\n") {
			got = append(got, strings.TrimRight(line, " "))
		}

		if strings.Join(got, "\n") != strings.Join(ftnPassedRun, "\n") {
			t.Errorf("PASSED printed:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(ftnPassedRun, "\n"))
		}
	})
}
