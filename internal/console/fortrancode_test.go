package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// The FORTRAN code generator (docs/PHASE-75 - FORTRAN first light.md,
// subtask 6): a program compiled by FORTRAN, assembled by MACRO, and
// linked with a MACRO module that prints values (FTNRTL's output is
// subtask 7's) runs and computes what FORTRAN says it should.

// ftnPut prints its argument: PUTL an INTEGER in 12 columns, PUTF a
// REAL's F_floating bits in hexadecimal.
const ftnPut = `	.TITLE	PUT
	.PSECT	PUTDATA,WRT,NOEXE,LONG
BUF:	.BLKB	12
DEC:	.WORD	12
	.BYTE	14,1
	.ADDRESS BUF
HEX:	.WORD	8
	.BYTE	14,1
	.ADDRESS BUF
	.PSECT	PUTCODE,NOWRT,EXE,LONG
	.ENTRY	PUTL,^M<>
	PUSHAQ	DEC
	PUSHL	4(AP)
	CALLS	#2,G^OTS$CVT_L_TI
	PUSHAQ	DEC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
	.ENTRY	PUTF,^M<>
	PUSHL	#8
	PUSHAQ	HEX
	PUSHL	4(AP)
	CALLS	#3,G^OTS$CVT_L_TZ
	PUSHAQ	HEX
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
	.END
`

// ftnCodeProgram exercises the subset: each CALL PUTL's value is in the
// comment after it.
const ftnCodeProgram = `      PROGRAM RUN1
      INTEGER I, J, K, N, M(3,4), V(0:9), ISQ
      REAL X, Y, AVG
      LOGICAL L, P
C     Integer arithmetic, truncating division, and an expression too
C     big for the registers.
      I = 7
      J = -3
      CALL PUTL(I/J)
      CALL PUTL(I*J + 5 - (I - J)*2)
      K = ((I+1)*(J+2) + (I+3)*(J+4)) * ((I+5)*(J+6) + (I+7)*(J+8))
     1  - ((I-1)*(J-2) + (I-3)*(J-4)) * ((I-5)*(J-6) + (I-7)*(J-8))
      CALL PUTL(K)
C     REAL arithmetic and the conversions, truncating to INTEGER.
      X = 7.9
      N = X
      CALL PUTL(N)
      N = -X
      CALL PUTL(N)
      Y = I / 2
      N = Y * 10
      CALL PUTL(N)
      X = 1.5 * I + 0.25
      N = X * 4
      CALL PUTL(N)
      CALL PUTF(-2.5)
      CALL PUTF(X)
C     DO loops: a step, a negative step, no passes, nested with a
C     shared terminal statement, a REAL variable.
      N = 0
      DO 10 I = 1, 10, 3
         N = N + I
   10 CONTINUE
      CALL PUTL(N)
      CALL PUTL(I)
      N = 0
      DO 20 I = 10, 1, -4
   20 N = N + I
      CALL PUTL(N)
      CALL PUTL(I)
      J = 0
      DO 30 I = 5, 1
   30 J = J + 1
      CALL PUTL(J*100 + I)
      K = 3
      N = 0
      DO 40 I = 1, K
         DO 40 J = I, K
   40 N = N + 1
      CALL PUTL(N)
      N = 0
      DO 50 X = 0.5, 2.0, 0.5
   50 N = N + X*2
      CALL PUTL(N)
C     Arrays of one and two dimensions, and an array's tail passed.
      DO 60 I = 1, 3
      DO 60 J = 1, 4
   60 M(I,J) = I*10 + J
      CALL PUTL(M(2,3))
      CALL PUTL(M(3,4) + M(1,2))
      DO 70 I = 0, 9
   70 V(I) = I*I
      CALL PUTL(V(9) + V(0) + V(3))
      CALL TOTAL(V(5), 3, N)
      CALL PUTL(N)
C     Functions, and an argument changed or copied.
      CALL PUTL(ISQ(I+1))
      J = 4
      CALL BUMP(J)
      CALL BUMP((J))
      CALL PUTL(J)
      CALL PUTL(IFACT(10))
      N = AVG(V, 10) * 2
      CALL PUTL(N)
C     Logical values and conditions.
      L = I .GT. 5
      P = .NOT. L .OR. J .EQ. 5
      IF (L .AND. P) CALL PUTL(1)
      IF (L .AND. .NOT. P) CALL PUTL(2)
      IF (.NOT. (L .OR. P)) CALL PUTL(3)
      N = L .AND. P
      CALL PUTL(N)
      IF (I) CALL PUTL(99)
      IF (J) CALL PUTL(4)
      IF (Y - 3.0 .EQ. 0.0) CALL PUTL(5)
C     Block IF in a loop, and a jump to the loop's end.
      DO 80 I = 1, 4
         IF (I .EQ. 1) THEN
            N = 100
         ELSE IF (I .LT. 3) THEN
            N = 200
         ELSE IF (I .EQ. 3) THEN
            GO TO 80
         ELSE
            N = 400
         END IF
         CALL PUTL(N + I)
   80 CONTINUE
      STOP 'DONE'
      END
C
      INTEGER FUNCTION ISQ(N)
      ISQ = N*N
      END
C
      INTEGER FUNCTION IFACT(N)
      IFACT = 1
      DO 10 I = 2, N
   10 IFACT = IFACT * I
      END
C
      SUBROUTINE BUMP(K)
      K = K + 1
      END
C
      SUBROUTINE TOTAL(A, N, S)
      INTEGER A(*), N, S
      S = 0
      DO 10 I = 1, N
   10 S = S + A(I)
      END
C
      REAL FUNCTION AVG(A, N)
      INTEGER N, A(10)
      AVG = 0
      DO 10 I = 1, N
   10 AVG = AVG + A(I)
      AVG = AVG / N
      END
`

// ftnCodeRun is what RUN1 prints, and $STATUS after it.
var ftnCodeRun = []string{
	"-2", "-36", "-832", // integer arithmetic
	"7", "-7", "30", "43", "0000C120", "0000422C", // REAL
	"22", "13", "18", "-2", "5", "6", "10", // loops
	"23", "46", "90", "110", // arrays
	"121", "5", "3628800", "57", // functions and arguments
	"1", "-1", "4", "5", // logicals
	"101", "202", "404", // block IF
	"DONE",
	`$STATUS == "%X00000001"`,
}

func TestFortranCodeRuns(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		for name, text := range map[string]string{"RUN1.FOR": ftnCodeProgram, "PUT.MAR": ftnPut} {
			if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords, splitSource(text)); err != nil {
				t.Fatal(err)
			}
		}

		shareFindRun(t, d, "FORTRAN RUN1")
		shareFindRun(t, d, "MACRO RUN1")
		shareFindRun(t, d, "MACRO PUT")
		shareFindRun(t, d, "LINK RUN1,PUT")

		if out := buf.String(); out != "" {
			t.Fatalf("FORTRAN, MACRO, and LINK said:\n%s", out)
		}

		var got []string
		for _, line := range strings.Split(strings.TrimSpace(shareRun(t, d, buf, "RUN1")), "\n") {
			got = append(got, strings.TrimSpace(line))
		}

		if strings.Join(got, "\n") != strings.Join(ftnCodeRun, "\n") {
			t.Errorf("RUN1 printed:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(ftnCodeRun, "\n"))
		}
	})
}
