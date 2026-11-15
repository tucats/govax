package console

import (
	"strings"
	"testing"
)

// TestInnerModeConditionSearch: a condition raised in kernel mode, in a
// routine $CMKRNL called, is searched for on the kernel stack's frames
// alone; the program's user-mode handler, in a frame on the user stack,
// isn't called for it. Here the kernel routine calls a service govax
// lacks, whose error condition the catch-all reports and continues; the
// service returns SS$_UNSUPPORTED, and a second $CMKRNL then works,
// where a user-mode handler's unwind from kernel mode used to leave the
// kernel stack 4 bytes off, so every later $CMKRNL faulted
// (docs/planning/PHASE-94 - privileges.md, subtask 2's rehearsal).
func TestInnerModeConditionSearch(t *testing.T) {
	r0, out := runMacroProgram(t, `	.PSECT	DATA,WRT,NOEXE,LONG
CALLED:	.LONG	0
	.PSECT	CODE,NOWRT,EXE
	.ENTRY	START,^M<>
	CALLS	#0,SUB
	PUSHL	#0
	PUSHAB	GOOD
	CALLS	#2,G^SYS$CMKRNL
	TSTL	CALLED
	BEQL	10$
	MOVL	#2,R0
10$:	RET
	.ENTRY	SUB,^M<>
	MOVAB	HANDLER,(FP)
	PUSHL	#0
	PUSHAB	BAD
	CALLS	#2,G^SYS$CMKRNL
	RET
	.ENTRY	HANDLER,^M<>
	INCL	CALLED
	PUSHL	#0
	PUSHL	#0
	CALLS	#2,G^SYS$UNWIND
	MOVL	#1,R0
	RET
	.ENTRY	BAD,^M<>
	CALLS	#0,G^SYS$GETCHN
	RET
	.ENTRY	GOOD,^M<>
	MOVL	#1,R0
	RET
	.END	START
`)

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (the second $CMKRNL's, with the handler never called)\n%s", r0, out)
	}

	if !strings.Contains(out, "-GOVAX-E-NOTIMPL, routine SYS$GETCHN is not implemented in govax") {
		t.Errorf("no NOTIMPL message from the catch-all:\n%s", out)
	}
}
