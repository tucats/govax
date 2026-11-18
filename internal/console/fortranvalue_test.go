package console

import (
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// An array-valued function of run-time shape called from MACRO
// (docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md, finding 12, subtask
// 7b): the caller passes a class NCA descriptor with its prototype block and a
// POINTER of 0 as the hidden first argument; the function fills it in (POINTER,
// ARSIZE, A0, strides, bounds) with storage from the heap, contiguous, so that
// the caller may use POINTER and ARSIZE alone and give the block back with
// LIB$FREE_VM. A CHARACTER value of a run-time length is a dynamic string the
// function gets with LIB$SGET1_DD in the caller's class D descriptor, which the
// caller frees with LIB$SFREE1_DD (subtask 7c).

// ftnValueRoutines is the FORTRAN side: RAMP2(M, N), a value of bounds (2:M+1,
// N), and routines that print a descriptor's fields, the elements at an
// address, and a status's low bit.
const ftnValueRoutines = `      OPTIONS /F90
      FUNCTION RAMP2(M, N)
      INTEGER M, N, I, J
      REAL RAMP2(2:M+1, N)
      DO J = 1, N
        DO I = 2, M + 1
          RAMP2(I, J) = 10 * I + J
        END DO
      END DO
      END
      FUNCTION WORDS(N)
      INTEGER N
      CHARACTER(LEN=N) WORDS
      WORDS = 'abcdefgh'
      END
      SUBROUTINE SHOWD(D)
      INTEGER D(11)
      PRINT '(1X, I10, I9, I3, I4, 6I3)', D(1), D(3), D(4), D(5) - D(2),
     1  D(6), D(7), D(8), D(9), D(10), D(11)
      END
      SUBROUTINE SHOWE(V, N)
      INTEGER N
      REAL V(N)
      PRINT '(1X, 6F5.1)', V
      END
      SUBROUTINE SHOWS(ST)
      INTEGER ST
      PRINT '(1X, A, I2)', 'freed', MOD(ST, 2)
      END
`

// ftnValueCaller is the MACRO program: RAMP2(2, 3) into its own descriptor, the
// descriptor shown, the six elements at POINTER, and the block given back.
const ftnValueCaller = `	.TITLE	VALUEM
	.PSECT	DATA,NOEXE,WRT,LONG
DESC:	.WORD	4			; REAL*4, class NCA, rank 2
	.BYTE	10,10
	.LONG	0			; POINTER: none yet
	.BYTE	0,0,0,2
	.LONG	0,0,0,0,0,0,0,0
DESCD:	.WORD	0			; a dynamic string (DTYPE_T, CLASS_D)
	.BYTE	14,2
	.LONG	0
M:	.LONG	2
N:	.LONG	3
SIX:	.LONG	6
ST:	.LONG	0
	.PSECT	CODE,EXE,NOWRT
	.ENTRY	VALUEM,^M<>
	PUSHAB	N
	PUSHAB	M
	PUSHAB	DESC
	CALLS	#3,G^RAMP2
	PUSHAB	DESC
	CALLS	#1,G^SHOWD
	PUSHAB	SIX
	PUSHL	DESC+4
	CALLS	#2,G^SHOWE
	PUSHAL	DESC+4
	PUSHAL	DESC+12
	CALLS	#2,G^LIB$FREE_VM
	MOVL	R0,ST
	PUSHAB	ST
	CALLS	#1,G^SHOWS
	PUSHAB	N			; WORDS(3) into DESCD
	PUSHAB	DESCD
	CALLS	#2,G^WORDS
	PUSHAB	DESCD
	CALLS	#1,G^LIB$PUT_OUTPUT
	PUSHAB	DESCD
	CALLS	#1,G^LIB$SFREE1_DD
	MOVL	R0,ST
	PUSHAB	ST
	CALLS	#1,G^SHOWS
	MOVL	#1,R0			; success
	RET
	.END	VALUEM
`

func TestFortranValueFromMACRO(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	for name, text := range map[string]string{"RAMPV.FOR": ftnValueRoutines, "VALUEM.MAR": ftnValueCaller} {
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords, splitSource(text)); err != nil {
			t.Fatal(err)
		}
	}

	shareFindRun(t, d, "FORTRAN RAMPV")
	shareFindRun(t, d, "MACRO RAMPV")
	shareFindRun(t, d, "MACRO VALUEM")
	shareFindRun(t, d, "LINK VALUEM,RAMPV")

	if out := buf.String(); out != "" {
		t.Fatalf("FORTRAN, MACRO, and LINK said:\n%s", out)
	}

	if err := d.Dispatch("RUN VALUEM"); err != nil && !vmserrors.MessageInhibited(err) {
		fmt.Fprintf(buf, "%%%s\n", err)
	}

	// The descriptor: REAL*4 (DTYPE_F, 10) of class NCA (10), rank 2; 24 bytes;
	// A0 16 bytes before POINTER, the element (0, 0) counting from (2, 1) with
	// strides 4 and 8; bounds 2:3 and 1:3. Then the elements in array element
	// order, and LIB$FREE_VM's success. Then WORDS(3), a CHARACTER value of
	// length 3 the function gave a dynamic string in the program's class D
	// descriptor, and LIB$SFREE1_DD's success.
	want := " 168427524 33554432 24 -16  4  8  2  3  1  3\n" +
		" 21.0 31.0 22.0 32.0 23.0 33.0\n" +
		"freed 1\n" +
		"abc\n" +
		"freed 1\n"

	if got := buf.String(); got != want {
		t.Errorf("RUN VALUEM printed:\n%s\nwant:\n%s", got, want)
	}
}
