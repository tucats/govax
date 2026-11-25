package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/testproc"
)

// srchandlerProgram signals CONDITION the way VMS 7.1's own LIB$SIGNAL does
// (Phase 113, subtask 10): with no call of govax's, it lays out on its stack
// the signal array, a longword VMS leaves between the arrays, the mechanism
// array, and the argument list naming them (VAX/VMS Internals and Data
// Structures, Figure 4-3), and jumps to SYS$SRCHANDLER. Its handler continues
// a warning, with R0 changed to ^X41 in the mechanism array, and resignals
// anything else.
const srchandlerProgram = `	.TITLE	SRCH
	.PSECT	CODE,EXE,NOWRT
	.ENTRY	START,^M<R2,R3>
	MOVAB	HANDLER,(FP)
	MOVPSL	-(SP)			; the signal array: PSL,
	MOVAB	10$,-(SP)		; PC,
	PUSHL	#CONDITION		; the condition,
	PUSHL	#3			; and its count
	MOVL	SP,R2
	PUSHL	#2			; VMS's longword between the arrays
	CLRQ	-(SP)			; the mechanism array: R1, R0,
	MNEGL	#3,-(SP)		; depth,
	PUSHL	FP			; frame,
	PUSHL	#4			; and its count
	MOVL	SP,R3
	PUSHL	R3			; the argument list
	PUSHL	R2
	PUSHL	#2
	JMP	G^SYS$SRCHANDLER
10$:	RET				; continued: R0 from the mechanism array

	.ENTRY	HANDLER,^M<R2>
	MOVL	4(AP),R2		; the signal array
	CMPL	4(R2),#^X1230
	BNEQ	20$
	MOVL	8(AP),R2		; the mechanism array
	MOVL	#^X41,12(R2)		; its R0
	MOVL	#1,R0			; SS$_CONTINUE
	RET
20$:	MOVL	#^X918,R0		; SS$_RESIGNAL
	RET
	.END	START
`

// TestSrchandlerFromProgram is Phase 113's subtask 10: SYS$SRCHANDLER takes
// up a condition a program laid out itself, as VMS's LIBRTL.EXE does when it
// signals; govax's own dispatches were all it knew before. A warning the
// handler continues resumes at the signal array's PC with the arrays gone; a
// severe condition every handler resignals ends the image.
func TestSrchandlerFromProgram(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	d, _, buf := dungeonConsole(t, false)
	dir := t.TempDir()

	for _, tc := range []struct{ condition, want string }{
		{"^X1230", `$STATUS == "%X00000041"`},
		{"^X1234", `$STATUS == "%X10001234"`},
	} {
		src := filepath.Join(dir, "srch.mar")
		text := "CONDITION = " + tc.condition + "\n" + srchandlerProgram

		if err := os.WriteFile(src, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, `COPY/QUIET "`+src+`"/HOST SRCH.MAR`)
		shareFindRun(t, d, "MACRO SRCH")
		shareFindRun(t, d, "LINK/NOTRACEBACK SRCH")

		buf.Reset()
		_ = d.Dispatch("RUN SRCH")
		shareFindRun(t, d, "SHOW SYMBOL $STATUS")

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if got := strings.TrimSpace(lines[len(lines)-1]); got != tc.want {
			t.Errorf("CONDITION = %s: %q, want %q\n%s", tc.condition, got, tc.want, buf.String())
		}
	}
}
