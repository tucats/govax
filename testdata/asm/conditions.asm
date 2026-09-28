;
; testdata/asm/conditions.asm -- docs/PHASE-26.md subtask 31's acceptance
; fixture: hardware exceptions signaled to condition handlers in call
; frames.
;
;   1. MAIN establishes MAINH as its frame's condition handler (the first
;      longword of a call frame holds its handler's address), then calls
;      SUB1. SUB1 establishes SUBH and reads address 0, which is in the
;      no-access guard page: an access violation. The search calls SUBH
;      first (depth 0: SUB1's own frame), which resignals, then MAINH
;      (depth 1: SUB1's caller). MAINH records the signal array's reason
;      mask and address, changes the saved R0 in the mechanism array to
;      ^X55 and the signal array's PC to RESUME1, and continues. SUB1
;      resumes at RESUME1 with R0 = ^X55 and returns 2.
;   2. MAIN calls SUB2, whose INDEX instruction has a subscript out of
;      range: an arithmetic exception, signaled as SS$_SUBRNG. MAINH
;      continues it at RESUME2, and SUB2 returns 3.
;
; The Go test checks every value recorded below. (The assembler's default
; radix is hexadecimal, so decimal offsets are written ^X.)

	.microkernel
	.p1vector

; Send the exceptions to console$handler, as kernel.asm's SCB does in a
; booted system: the RTL's condition dispatcher takes them from there.
	.scb	exc$accvio, console$handler
	.scb	exc$arith, console$handler

	.entry	main, ^m<>
	movab	mainh, (fp)		; MAINH handles conditions in MAIN and below
	calls	#0, @#sub1
	movl	r0, @#ret1
	calls	#0, @#sub2
	movl	r0, @#ret2
	movl	#1, r0
	ret

	.entry	sub1, ^m<>
	movab	subh, (fp)
	movl	@#0, r1			; access violation
	movl	#99, r0			; skipped: MAINH resumes at RESUME1
	ret
resume1:
	movl	r0, @#r0seen		; the mechanism array's R0
	movl	#2, r0
	ret

	.entry	sub2, ^m<>
	index	#^X0A, #0, #5, #1, #0, r0	; subscript 10 isn't in 0..5
	movl	#99, r0			; skipped
	ret
resume2:
	movl	#3, r0
	ret

; SUBH: count the call, record the depth, and resignal.
	.entry	subh, ^m<>
	incl	@#subcalls
	movl	8(ap), r0		; mechanism array
	movl	8(r0), @#subdepth
	movl	#^X918, r0		; SS$_RESIGNAL
	ret

; MAINH: continue an access violation at RESUME1 and a subscript range
; trap at RESUME2; resignal anything else.
	.entry	mainh, ^m<r2,r3,r4>
	movl	4(ap), r2		; signal array: n, condition, ..., PC, PSL
	movl	8(ap), r3		; mechanism array: 4, frame, depth, R0, R1
	movl	(r2), r4
	ashl	#2, r4, r4
	addl2	r2, r4
	subl2	#4, r4			; R4 -> the signal array's PC
	cmpl	4(r2), #^X0C		; SS$_ACCVIO?
	bneq	notacc
	movl	8(r2), @#mask
	movl	^X0C(r2), @#va
	movl	8(r3), @#depth1
	movl	#^X55, ^X0C(r3)		; the R0 SUB1 will resume with
	movab	resume1, (r4)
	brb	cont
notacc:	cmpl	4(r2), #^X4AC		; SS$_SUBRNG?
	bneq	resig
	movl	8(r3), @#depth2
	movab	resume2, (r4)
cont:	movl	#1, r0			; SS$_CONTINUE
	ret
resig:	movl	#^X918, r0		; SS$_RESIGNAL
	ret

ret1:	.long	0
ret2:	.long	0
r0seen:	.long	0
mask:	.long	^XFF
va:	.long	^XFF
depth1:	.long	^XFF
depth2:	.long	^XFF
subcalls: .long	0
subdepth: .long	^XFF

	.end	main
