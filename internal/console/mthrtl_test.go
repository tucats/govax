package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// MTHRTL, VMS's mathematics library (docs/PHASE-77 - MTHRTL.md): LINK
// finds its routines in the table captured from VMS 7.1's image, so a
// program that calls one links with no MTHRTL.EXE anywhere. Subtask 1 has
// no routines, so the call signals NOTIMPL.

const mthCaller = `	.TITLE	MTHCALL
	.PSECT	DATA,WRT,NOEXE,LONG
X:	.FLOAT	1.0
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	PUSHAF	X
	CALLS	#1,G^MTH$SIN
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

		if out := buf.String(); !strings.Contains(out, "NOTIMPL") || !strings.Contains(out, "MTHRTL") {
			t.Errorf("RUN said:\n%s", out)
		}
	})
}
