;
; testdata/asm/putmsg.asm -- docs/PHASE-26.md subtask 25's acceptance
; fixture: $GETMSG and $PUTMSG, the services VMS reports errors with.
;
;   1. $GETMSG for SS$_ABORT (^X2C) with flags 1 (the text only): "abort",
;      5 characters.
;   2. $PUTMSG of the manual's example vector -- SS$_ABORT, then RMS$_FNF
;      (^X18292) with a zero STV -- with an action routine, ACTRTN. The
;      routine sees each line before it's written: it counts the calls,
;      records the first line's length, and returns 1 (write it) the first
;      time and 0 (don't) the second. So only "%SYSTEM-F-ABORT, abort" is
;      written.
;   3. $PUTMSG of SS$_ACCVIO (^XC) with its four $FAO parameters, no
;      action routine.
;
; R0 is 1 at the end if every call behaved; the Go test checks the text
; written, the call count, and the recorded length.

	.microkernel
	.p1vector

	.entry	main, ^m<>

; ---- 1. $GETMSG(SS$_ABORT, flags 1) ----
	pushl	#0			; outadr
	pushl	#1			; flags: the text only
	pushal	@#msgdsc		; bufadr
	pushal	@#msglen		; msglen
	pushl	#^X2C			; msgid: SS$_ABORT
	calls	#5, @#sys$getmsg
	blbs	r0, ok1
	brw	fail
ok1:	cmpw	@#msglen, #5		; "abort"
	beql	ok2
	brw	fail
ok2:

; ---- 2. $PUTMSG(VECTOR, ACTRTN, , 7) ----
	pushl	#7			; actprm
	pushl	#0			; facnam
	pushal	@#actrtn		; actrtn
	pushal	@#vector		; msgvec
	calls	#4, @#sys$putmsg
	blbs	r0, ok3
	brw	fail
ok3:	cmpl	@#calls, #2		; the routine saw both lines
	beql	ok4
	brw	fail
ok4:

; ---- 3. $PUTMSG(ACCVEC) ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushal	@#accvec
	calls	#4, @#sys$putmsg
	blbs	r0, ok5
	brw	fail
ok5:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; The action routine: called with a line's descriptor and actprm (7).
; Returns 1 (write the line) the first time, 0 after that.
	.entry	actrtn, ^m<r2>
	cmpl	8(ap), #7		; actprm arrived
	bneq	no
	incl	@#calls
	movl	4(ap), r2		; the line's descriptor
	cmpl	@#calls, #1
	bneq	no
	movzwl	(r2), @#linlen		; the first line's length
	movl	#1, r0
	ret
no:	clrl	r0
	ret

calls:	.long	0
linlen:	.long	0
msglen:	.word	0

; Message vectors: the count of longwords that follow (flags 0: all
; parts), then the messages.
vector:	.long	3, ^X2C, ^X18292, 0
accvec:	.long	5, ^XC, 4, ^X200, ^X300, ^X1B

msgdsc:	.long	^D80
	.long	msgbuf
msgbuf:	.blkb	^D80

	.end	main
