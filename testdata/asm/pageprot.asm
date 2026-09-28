;
; testdata/asm/pageprot.asm -- docs/PHASE-26.md subtask 36's acceptance
; fixture: page protection and page locking.
;
;   1. $SETPRT makes the page at ^X60000 user read-only (PRT$C_UR, ^XF),
;      returning its old protection (PRT$C_UW, 4) in PRVPRT. Writing it is
;      then an access violation, which MAIN's handler records (FAULTS) and
;      continues past, at AFTER.
;   2. $SETPRT makes it PRT$C_UW again; the write works (VALUE).
;   3. $LKWSET locks the page in the working set twice: SS$_WASCLR, then
;      SS$_WASSET (LOCK1, LOCK2). $ULWSET unlocks it: SS$_WASSET (UNLOCK).
;
; The Go test checks every value recorded below.

	.microkernel
	.p1vector
	.scb	exc$accvio, console$handler

	.entry	main, ^m<>
	movab	handler, (fp)
	pushal	@#prvprt
	pushl	#^X0F			; PRT$C_UR
	pushl	#0
	pushl	#0
	pushal	@#range
	calls	#5, @#sys$setprt
	blbc	r0, fail
	movl	#^X11, @#^X60008	; access violation
after:	pushl	#0
	pushl	#4			; PRT$C_UW
	pushl	#0
	pushl	#0
	pushal	@#range
	calls	#5, @#sys$setprt
	blbc	r0, fail
	movl	#^X22, @#^X60008
	movl	@#^X60008, @#value
	pushl	#0
	pushl	#0
	pushal	@#range
	calls	#3, @#sys$lkwset
	movl	r0, @#lock1
	pushl	#0
	pushl	#0
	pushal	@#range
	calls	#3, @#sys$lkwset
	movl	r0, @#lock2
	pushl	#0
	pushl	#0
	pushal	@#range
	calls	#3, @#sys$ulwset
	movl	r0, @#unlock
	movl	#1, r0
	ret

fail:	clrl	r0
	ret

; HANDLER: count an access violation and continue at AFTER.
	.entry	handler, ^m<r2>
	movl	4(ap), r0		; signal array
	cmpl	4(r0), #^X0C		; SS$_ACCVIO?
	bneq	resig
	incl	@#faults
	movl	(r0), r2
	ashl	#2, r2, r2
	addl2	r2, r0
	subl2	#4, r0			; -> the signal array's PC
	movab	after, (r0)
	movl	#1, r0			; SS$_CONTINUE
	ret
resig:	movl	#^X918, r0		; SS$_RESIGNAL
	ret

range:	.long	^X60000, ^X60000
prvprt:	.long	^XFF			; a byte, in a longword
faults:	.long	0
value:	.long	0
lock1:	.long	0
lock2:	.long	0
unlock:	.long	0

	.end	main
