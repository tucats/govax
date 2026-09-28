;
; testdata/asm/vaspace.asm -- docs/PHASE-26.md subtask 35's acceptance
; fixture: creating and deleting virtual address space.
;
;   1. $CRETVA creates the two pages at ^X60000-^X603FF (RETADR1). The
;      program writes ^X55 to ^X60208 and reads it back (READ1).
;   2. $DELTVA deletes them (RETADR2). Reading ^X60208 now is an access
;      violation: MAIN's handler records the address from the signal
;      array (BADVA) and continues at AFTER.
;   3. $CRETVA creates ^X60200's page again: a fresh demand-zero page, so
;      ^X60208 reads 0 (READ3, which starts as ^XFF).
;
; The Go test checks every value recorded below.

	.microkernel
	.p1vector
	.scb	exc$accvio, console$handler

	.entry	main, ^m<>
	movab	handler, (fp)
	pushl	#0			; acmode: the caller's
	pushal	@#retadr1
	pushal	@#range
	calls	#3, @#sys$cretva
	blbc	r0, fail
	movl	#^X55, @#^X60208
	movl	@#^X60208, @#read1
	pushl	#0
	pushal	@#retadr2
	pushal	@#range
	calls	#3, @#sys$deltva
	blbc	r0, fail
	movl	@#^X60208, @#read2	; access violation
after:	pushl	#0
	pushl	#0
	pushal	@#page2
	calls	#3, @#sys$cretva
	blbc	r0, fail
	movl	@#^X60208, @#read3
	movl	#1, r0
	ret

fail:	clrl	r0
	ret

; HANDLER: for an access violation, record the address and continue at
; AFTER.
	.entry	handler, ^m<r2>
	movl	4(ap), r0		; signal array
	cmpl	4(r0), #^X0C		; SS$_ACCVIO?
	bneq	resig
	movl	^X0C(r0), @#badva	; its virtual address
	movl	(r0), r2
	ashl	#2, r2, r2
	addl2	r2, r0
	subl2	#4, r0			; -> the signal array's PC
	movab	after, (r0)
	movl	#1, r0			; SS$_CONTINUE
	ret
resig:	movl	#^X918, r0		; SS$_RESIGNAL
	ret

range:	.long	^X60000, ^X603FF
page2:	.long	^X60200, ^X60200
retadr1: .long	0, 0
retadr2: .long	0, 0
read1:	.long	0
read2:	.long	^XFF
read3:	.long	^XFF
badva:	.long	0

	.end	main
