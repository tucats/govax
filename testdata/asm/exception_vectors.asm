;
; testdata/asm/exception_vectors.asm -- docs/PHASE-26.md subtask 32's
; acceptance fixture: $SETEXV's exception vectors.
;
;   1. $SETEXV sets the primary vector to PRIM (which counts its calls,
;      records its depth, and resignals) and the last-chance vector to
;      LAST (which records its depth and continues at RESUME1). MAIN has
;      no frame handler, so an access violation goes PRIM, the frames
;      (none), then LAST.
;   2. $SETEXV clears the primary vector, getting PRIM back in PRVHND,
;      and sets the secondary vector to SEC, which continues at RESUME2.
;      A second access violation reaches SEC without PRIM being called.
;
; The Go test checks the counts, depths, and PRVHND.

	.microkernel
	.p1vector
	.scb	exc$accvio, console$handler

	.entry	main, ^m<>
	pushl	#0			; prvhnd
	pushl	#0			; acmode: the caller's
	pushal	@#prim			; addres
	pushl	#0			; vector: primary
	calls	#4, @#sys$setexv
	blbc	r0, fail
	pushl	#0
	pushl	#0
	pushal	@#last
	pushl	#2			; last chance
	calls	#4, @#sys$setexv
	blbc	r0, fail
	movl	@#0, r1			; access violation
resume1:
	pushal	@#prvhnd
	pushl	#0
	pushl	#0			; no address: clear
	pushl	#0			; primary
	calls	#4, @#sys$setexv
	blbc	r0, fail
	pushl	#0
	pushl	#0
	pushal	@#sec
	pushl	#1			; secondary
	calls	#4, @#sys$setexv
	blbc	r0, fail
	movl	@#0, r1			; access violation
resume2:
	movl	#1, r0
	ret

fail:	clrl	r0
	ret

; PRIM counts its calls, records its depth, and resignals.
	.entry	prim, ^m<>
	incl	@#primcalls
	movl	8(ap), r0
	movl	8(r0), @#primdepth
	movl	#^X918, r0		; SS$_RESIGNAL
	ret

; LAST and SEC record their depths and continue at RESUME1 and RESUME2.
	.entry	last, ^m<r2>
	movl	8(ap), r0
	movl	8(r0), @#lastdepth
	movab	resume1, r1
	brb	setpc

	.entry	sec, ^m<r2>
	movl	8(ap), r0
	movl	8(r0), @#secdepth
	movab	resume2, r1

; Store R1 as the PC in the signal array at 4(AP) and continue.
setpc:	movl	4(ap), r0
	movl	(r0), r2
	ashl	#2, r2, r2
	addl2	r2, r0
	subl2	#4, r0			; -> the signal array's PC
	movl	r1, (r0)
	movl	#1, r0			; SS$_CONTINUE
	ret

primcalls: .long 0
primdepth: .long 0
lastdepth: .long 0
secdepth:  .long 0
prvhnd:	   .long 0

	.end	main
