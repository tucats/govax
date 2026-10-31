package mthrtl_test

import (
	"strings"
	"testing"
)

// manualExample is the program in the manual's Appendix B.
const manualExample = `	.TITLE	ROOT
	.PSECT	DATA,WRT,NOEXE,LONG
TWO:	.FLOAT	2.0
BAD:	.FLOAT	-1.0
SEED:	.LONG	1
CTL:	.ASCID	/square root !XL, random !XL/
BUF:	.BLKB	80
OUT:	.LONG	80
	.ADDRESS BUF
LEN:	.BLKL	1
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<R2>
	PUSHAF	TWO
	CALLS	#1,G^MTH$SQRT		; the argument by reference, the result in R0
	MOVL	R0,R2
	PUSHAL	SEED
	CALLS	#1,G^MTH$RANDOM
	$FAO_S	CTRSTR=CTL,OUTLEN=LEN,OUTBUF=OUT,P1=R2,P2=R0
	MOVL	LEN,OUT
	PUSHAQ	OUT
	CALLS	#1,G^LIB$PUT_OUTPUT
	PUSHAF	BAD
	CALLS	#1,G^MTH$SQRT		; no handler: a severe condition ends the image
	RET
	.END	START
`

// TestManualExample holds the manual's example to the output it shows.
func TestManualExample(t *testing.T) {
	_, d, out := boot(t)

	build(t, d, out, "ROOT", manualExample)

	_ = d.Dispatch("RUN ROOT") // the image's end is what's looked at

	t.Logf("RUN said:\n%s", out)

	for _, want := range []string{
		"square root 04F340B5, random 80003886",
		"%MTH-F-SQUROONEG, square root of negative value",
		"user PC ",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("no %q; RUN said:\n%s", want, out)
		}
	}
}
