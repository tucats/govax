package console

import "testing"

// TestBPTAndCustomerOpcodeSignal: BPT and an XFC whose selector govax
// doesn't define fault through the SCB vectors VMS's exception dispatcher
// turns into SS$_BREAK and SS$_OPCCUS (*VAX/VMS Internals and Data
// Structures*, Table 4-2), which the program's handler sees. Both are
// faults, so the handler moves the signal array's PC past the instruction
// before it continues.
func TestBPTAndCustomerOpcodeSignal(t *testing.T) {
	r0, out := runMacroProgram(t, `	$SSDEF
	.PSECT	DATA,WRT,NOEXE
BREAK:	.ASCID	/SS$_BREAK/
CUSTOM:	.ASCID	/SS$_OPCCUS/
DONE:	.ASCID	/done/
	.PSECT	CODE,NOWRT,EXE
	.ENTRY	HANDLER,^M<R2,R3>
	MOVL	4(AP),R2		; the signal array
	MOVL	(R2),R3			; its count: the PC is at -4(R2)[R3]
	CMPL	4(R2),#SS$_BREAK
	BNEQ	10$
	INCL	-4(R2)[R3]		; past the BPT, 1 byte
	PUSHAQ	BREAK
	BRB	30$
10$:	CMPL	4(R2),#SS$_OPCCUS
	BNEQ	20$
	ADDL2	#2,-4(R2)[R3]		; past the XFC, 2 bytes
	PUSHAQ	CUSTOM
	BRB	30$
20$:	MOVL	#SS$_RESIGNAL,R0
	RET
30$:	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#SS$_CONTINUE,R0
	RET
	.ENTRY	START,^M<>
	MOVAB	HANDLER,(FP)
	BPT
	.BYTE	^XFC,^X55		; XFC #^X55, no such selector
	PUSHAQ	DONE
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#1,R0
	RET
	.END	START
`)

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1", r0)
	}

	if want := "SS$_BREAK\nSS$_OPCCUS\ndone\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

// TestServiceArgumentListOutOfReach: a system service called with an
// argument list the program can't read returns SS$_ACCVIO, as VMS's change
// mode dispatcher returns it, rather than faulting; the program goes on.
func TestServiceArgumentListOutOfReach(t *testing.T) {
	r0, out := runMacroProgram(t, `	.PSECT	CODE,NOWRT,EXE
	.ENTRY	START,^M<>
	CALLG	@#4,G^SYS$GETTIM	; page 0 has no access
	RET
	.END	START
`)

	if r0 != 12 { // SS$_ACCVIO
		t.Errorf("R0 = %#x, want 0xC (SS$_ACCVIO)\n%s", r0, out)
	}
}
