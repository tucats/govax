package console

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// Assumed-shape dummies called from MACRO (docs/planning/PHASE-104 - Fortran 90
// dynamic arrays.md, finding 9, subtask 5b): the routine's entry copies whatever
// array descriptor it is given, a class NCA one with strides in bytes or a class
// A one with multipliers in elements, and under /CHECK=BOUNDS refuses anything
// else, or a descriptor of another rank, with FOR's BADARRDESC.

// ftnShapedRoutine is the FORTRAN side: a matrix of any shape, printed whole.
const ftnShapedRoutine = `      OPTIONS /F90/CHECK=BOUNDS
      SUBROUTINE SHOWV(V)
      REAL V(:, :)
      PRINT '(1X, 2I3, 6F5.1)', SIZE(V, 1), SIZE(V, 2), V
      END
`

// ftnShapedCaller is a MACRO program passing SHOWV a class A descriptor of a 2
// by 3 array, a class NCA one of its first row's first and third elements, and
// then the descriptor the program's foreign command names as its first
// character: S a scalar's (class S), R a class NCA one of rank 1.
const ftnShapedCaller = `	.TITLE	SHAPEM
	.PSECT	DATA,NOEXE,WRT,LONG
ARR:	.F_FLOATING 1.0,2.0,3.0,4.0,5.0,6.0
DSCA:	.WORD	4			; class A, 2 by 3: multipliers 2 and 3
	.BYTE	10,4
	.ADDRESS ARR
	.BYTE	0,0,^XC0,2
	.LONG	24
	.ADDRESS ARR-12
	.LONG	2,3
	.LONG	1,2,1,3
DSCN:	.WORD	4			; class NCA, 1 by 2: strides 4 and 16
	.BYTE	10,10
	.ADDRESS ARR
	.BYTE	0,0,0,2
	.LONG	8
	.ADDRESS ARR-20
	.LONG	4,16
	.LONG	1,1,1,2
DSCS:	.WORD	4			; class S
	.BYTE	10,1
	.ADDRESS ARR
	.LONG	0,0
DSCR:	.WORD	4			; class NCA of rank 1
	.BYTE	10,10
	.ADDRESS ARR
	.BYTE	0,0,0,1
	.LONG	24
	.ADDRESS ARR-4
	.LONG	4
	.LONG	1,6
LINE:	.LONG	8
	.ADDRESS TEXT
TEXT:	.BLKB	8
	.PSECT	CODE,EXE,NOWRT
	.ENTRY	SHAPEM,^M<>
	PUSHAB	LINE
	CALLS	#1,G^LIB$GET_FOREIGN
	PUSHAB	DSCA
	CALLS	#1,G^SHOWV
	PUSHAB	DSCN
	CALLS	#1,G^SHOWV
	MOVAB	DSCS,R0
	CMPB	TEXT,#^A/R/
	BNEQ	10$
	MOVAB	DSCR,R0
10$:	PUSHL	R0
	CALLS	#1,G^SHOWV
	RET
	.END	SHAPEM
`

func TestFortranShapedFromMACRO(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	for name, text := range map[string]string{"SHOWV.FOR": ftnShapedRoutine, "SHAPEM.MAR": ftnShapedCaller} {
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords, splitSource(text)); err != nil {
			t.Fatal(err)
		}
	}

	shareFindRun(t, d, "FORTRAN SHOWV")
	shareFindRun(t, d, "MACRO SHOWV")
	shareFindRun(t, d, "MACRO SHAPEM")
	shareFindRun(t, d, "LINK SHAPEM,SHOWV")
	shareFindRun(t, d, `SHAPEM :== $DUA1:[000000]SHAPEM`)

	if out := buf.String(); out != "" {
		t.Fatalf("FORTRAN, MACRO, and LINK said:\n%s", out)
	}

	for _, arg := range []string{"S", "R"} {
		buf.Reset()

		if err := d.Dispatch("SHAPEM " + arg); err != nil && !vmserrors.MessageInhibited(err) {
			fmt.Fprintf(buf, "%%%s\n", err)
		}

		out := buf.String()

		for _, want := range []string{
			" 2  3  1.0  2.0  3.0  4.0  5.0  6.0\n",
			" 1  2  1.0  5.0\n",
			"%FOR-F-BADARRDESC, argument is not an array descriptor of the dummy argument's rank\n",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("SHAPEM %s printed:\n%s\nwhich hasn't %q", arg, out, want)
			}
		}
	}
}
