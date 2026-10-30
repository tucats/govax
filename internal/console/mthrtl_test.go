package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// MTHRTL, VMS's mathematics library (docs/PHASE-77 - MTHRTL.md): LINK
// finds its routines in the table captured from VMS 7.1's image, so a
// program that calls one links with no MTHRTL.EXE anywhere. Every entry
// has a stub of its kind: every CALL entry has its routine by now, and a JSB
// entry no manual gives signals NOTIMPL with its name.

const mthCaller = `	.TITLE	MTHCALL
	.PSECT	DATA,WRT,NOEXE,LONG
X:	.FLOAT	1.0
NEXT:	.ASCID	/RANDOM RETURNED/
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	PUSHAF	X
	CALLS	#1,G^MTH$RANDOM
	PUSHAQ	NEXT
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVF	X,R0
	JSB	G^MTH$SQRT_R2
	MOVL	#1,R0
	RET
	.END	START
`

func TestMTHRTLLinks(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]MTHCALL.MAR"}, rms.TextRecords, splitSource(mthCaller)); err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, "DEFINE SYS$SHARE DUA2:[000000]")
		shareFindRun(t, d, "MACRO MTHCALL")
		shareFindRun(t, d, "LINK MTHCALL")

		if out := buf.String(); out != "" {
			t.Errorf("MACRO and LINK said:\n%s", out)
		}

		buf.Reset()
		_ = d.Dispatch("RUN MTHCALL")

		// A CALL entry answers, and a JSB entry with no routine says so.
		out := buf.String()

		if !strings.Contains(out, "RANDOM RETURNED") {
			t.Errorf("MTH$RANDOM didn't return; RUN said:\n%s", out)
		}

		if !strings.Contains(out, "routine MTH$SQRT_R2 is not implemented") {
			t.Errorf("no NOTIMPL for MTH$SQRT_R2; RUN said:\n%s", out)
		}
	})
}
