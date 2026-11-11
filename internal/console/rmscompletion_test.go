package console

import "testing"

// TestRMSCompletionRoutines: an RMS service's error and success completion
// routines run as ASTs before the service returns to its caller (the
// OpenVMS RMS Reference Manual, 2.4.3), with the FAB's or RAB's address as
// the parameter, and the service's R0 kept: a $OPEN that fails calls its
// error routine and not its success one, and a $CREATE,
// $CONNECT, and $PUT to the terminal call their success routines, the
// $PUT's after the record it wrote.
func TestRMSCompletionRoutines(t *testing.T) {
	r0, out := runMacroProgram(t, `	$FABDEF
	.PSECT	DATA,WRT,NOEXE
NOFAB:	$FAB	FNM=<NOSUCH_FILE_HERE.DAT>
TTFAB:	$FAB	FNM=<TT:>,RAT=CR
TTRAB:	$RAB	FAB=TTFAB,RBF=REC,RSZ=6
REC:	.ASCII	/record/
ERRMSG:	.ASCID	/error routine, parameter the FAB/
SUCMSG:	.ASCID	/success routine/
AFTER:	.ASCID	/after the $OPEN, R0 kept/
DONE:	.ASCID	/done/
BAD:	.ASCID	/wrong/
	.PSECT	CODE,NOWRT,EXE
	.ENTRY	ERRAST,^M<>
	MOVAB	BAD,R0
	CMPL	4(AP),#NOFAB
	BNEQ	10$
	MOVAB	ERRMSG,R0
10$:	PUSHL	R0
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
	.ENTRY	SUCAST,^M<>
	PUSHAB	SUCMSG
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
	.ENTRY	START,^M<R2>
	$OPEN	FAB=NOFAB,ERR=ERRAST,SUC=SUCAST
	MOVL	R0,R2
	BLBS	R2,90$
	CMPL	R2,NOFAB+FAB$L_STS
	BNEQ	90$
	PUSHAB	AFTER
	CALLS	#1,G^LIB$PUT_OUTPUT
	$CREATE	FAB=TTFAB,ERR=ERRAST,SUC=SUCAST
	$CONNECT RAB=TTRAB,ERR=ERRAST,SUC=SUCAST
	$PUT	RAB=TTRAB,ERR=ERRAST,SUC=SUCAST
	$CLOSE	FAB=TTFAB
	PUSHAB	DONE
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#1,R0
	RET
90$:	MOVL	R2,R0
	RET
	.END	START
`)

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (the failed $OPEN's R0 was its FAB$L_STS after its AST)", r0)
	}

	want := "error routine, parameter the FAB\n" +
		"after the $OPEN, R0 kept\n" +
		"success routine\n" +
		"success routine\n" +
		"record\n" +
		"success routine\n" +
		"done\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}
