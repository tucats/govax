package console

import (
	"strings"
	"testing"
)

// TestChangeModeToUser: a CHMU with no change mode handler is the condition
// SS$_CMODUSER, the code its detail longword, which the program's handler
// continues (a trap: execution goes on after the CHMU); with a handler
// declared by $DCLCMH, the CHMU goes to it in user mode, the code on top of
// the stack, which the handler pops before its REI.
func TestChangeModeToUser(t *testing.T) {
	r0, out := runMacroProgram(t, `	$SSDEF
	.PSECT	DATA,WRT,NOEXE
CODE:	.LONG	0
PRVHND:	.LONG	-1
SIGNAL:	.ASCID	/SS$_CMODUSER, code 5/
CMH:	.ASCID	/change mode handler, code 7/
DONE:	.ASCID	/done/
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	HANDLER,^M<R2>
	MOVL	4(AP),R2		; the signal array
	CMPL	4(R2),#SS$_CMODUSER
	BNEQ	10$
	CMPL	8(R2),#5
	BNEQ	10$
	PUSHAQ	SIGNAL
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#SS$_CONTINUE,R0
	RET
10$:	MOVL	#SS$_RESIGNAL,R0
	RET
	.ALIGN	LONG
	.BYTE	1			; a NOP: the handler starts at no boundary
CMHAND:	MOVL	(SP)+,CODE		; the change mode code
	REI
	.ENTRY	START,^M<>
	MOVAB	HANDLER,(FP)
	CHMU	#5			; no handler declared: a condition
	$DCLCMH_S ADDRES=CMHAND,PRVHND=PRVHND
	BLBC	R0,90$
	TSTL	PRVHND			; there was none before
	BNEQ	90$
	CHMU	#7			; to CMHAND
	CMPL	CODE,#7
	BNEQ	90$
	PUSHAQ	CMH
	CALLS	#1,G^LIB$PUT_OUTPUT
	PUSHAQ	DONE
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#1,R0
	RET
90$:	MOVL	#44,R0			; SS$_ABORT
	RET
	.END	START
`)

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1\n%s", r0, out)
	}

	if want := "SS$_CMODUSER, code 5\nchange mode handler, code 7\ndone\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

// TestSystemServiceFailureExceptions: with $SETSFM enabled, a service that
// fails with an error or severe status signals SS$_SSFAIL (a warning, such
// as $ASSIGN's SS$_NOSUCHDEV, doesn't), the status its detail longword; a handler that
// continues it has the service return with R0 the status. $SETSFM returns
// SS$_WASCLR, then SS$_WASSET. A service run in the caller's mode ($FAO)
// signals nothing.
func TestSystemServiceFailureExceptions(t *testing.T) {
	r0, out := runMacroProgram(t, `	$SSDEF
	.PSECT	DATA,WRT,NOEXE
STATUS:	.LONG	0
CTRL:	.ASCID	/!XL/
OUTBUF:	.BLKB	8
OUTDSC:	.LONG	8
	.ADDRESS OUTBUF
SSFAIL:	.ASCID	/SS$_SSFAIL with the service's status/
DONE:	.ASCID	/done/
	.PSECT	CODE,NOWRT,EXE
	.ENTRY	HANDLER,^M<R2>
	MOVL	4(AP),R2		; the signal array
	CMPL	4(R2),#SS$_SSFAIL
	BNEQ	10$
	MOVL	8(R2),STATUS
	PUSHAQ	SSFAIL
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#SS$_CONTINUE,R0
	RET
10$:	MOVL	#SS$_RESIGNAL,R0
	RET
	.ENTRY	START,^M<R2>
	MOVAB	HANDLER,(FP)
	$SETSFM_S ENBFLG=#1
	CMPL	R0,#SS$_WASCLR
	BNEQ	90$
	$SETSFM_S ENBFLG=#1
	CMPL	R0,#SS$_WASSET
	BNEQ	90$
	$GETTIM_S TIMADR=0		; SS$_ACCVIO, an error
	MOVL	R0,R2
	BLBS	R2,90$
	CMPL	R2,STATUS		; the service returned its status
	BNEQ	90$
	CLRL	STATUS
	$FAO_S	CTRSTR=CTRL,OUTBUF=OUTDSC	; too few arguments: fails, no signal
	TSTL	STATUS
	BNEQ	90$
	PUSHAQ	DONE
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#1,R0
	RET
90$:	MOVL	#44,R0			; SS$_ABORT
	RET
	.END	START
`)

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1\n%s", r0, out)
	}

	if want := "SS$_SSFAIL with the service's status\ndone\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

// TestSystemServiceFailureUnhandled: SS$_SSFAIL is severe, so with no
// handler to take it the catch-all shows its message and ends the image.
func TestSystemServiceFailureUnhandled(t *testing.T) {
	r0, out := runMacroProgram(t, `	.PSECT	DATA,WRT,NOEXE
NOTREACHED: .ASCID /not reached/
	.PSECT	CODE,NOWRT,EXE
	.ENTRY	START,^M<>
	$SETSFM_S ENBFLG=#1
	$GETTIM_S TIMADR=0		; SS$_ACCVIO, an error
	PUSHAQ	NOTREACHED
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#1,R0
	RET
	.END	START
`)

	if r0&0x0FFFFFFF != 0x45C { // SS$_SSFAIL, marked STS$M_INHIB_MSG
		t.Errorf("R0 = %#x, want SS$_SSFAIL\n%s", r0, out)
	}

	if !strings.Contains(out, "%SYSTEM-F-SSFAIL") || strings.Contains(out, "not reached") {
		t.Errorf("output %q, want the SSFAIL message and nothing after", out)
	}
}
