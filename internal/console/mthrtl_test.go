package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// MTHRTL, VMS's mathematics library (docs/PHASE-77 - MTHRTL.md): LINK
// finds its routines in the table captured from VMS 7.1's image, so a
// program that calls one links with no MTHRTL.EXE anywhere. Every entry
// has a stub of its kind, and a routine that isn't written yet signals
// NOTIMPL with its name.

const mthCaller = `	.TITLE	MTHCALL
	.PSECT	DATA,WRT,NOEXE,LONG
X:	.FLOAT	1.0
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	PUSHAF	X
	CALLS	#1,G^MTH$RANDOM
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

		// The error isn't severe, so the program goes on to the next call: a
		// CALL entry, then a JSB entry.
		out := buf.String()

		for _, routine := range []string{"MTH$RANDOM", "MTH$SQRT_R2"} {
			if !strings.Contains(out, "routine "+routine+" is not implemented") {
				t.Errorf("no NOTIMPL for %s; RUN said:\n%s", routine, out)
			}
		}
	})
}
