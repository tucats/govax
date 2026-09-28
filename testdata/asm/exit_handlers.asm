;
; testdata/asm/exit_handlers.asm -- docs/PHASE-26.md subtask 19's
; acceptance fixture: exit handlers and $EXIT. The Go test runs it in
; user mode, since kernel mode can't declare exit handlers.
;
;   1. $DCLEXH declares three exit control blocks, EXH1, EXH3, then EXH2.
;   2. $CANEXH cancels EXH3, so its handler must never run.
;   3. $EXIT(^X2C) calls the remaining handlers, newest first: HANDLER2,
;      then HANDLER1. Each records the order it ran in and the exit
;      status $EXIT stored through its first argument.
;   4. $EXIT never returns: the image ends, returning to the console
;      with R0 = ^X2C. The instructions after the call must not run.
;
; The Go test checks R0, the order (HANDLER2 first), both recorded
; statuses, that HANDLER3 and the code after $EXIT never ran, and that
; no handlers are left declared.

	.microkernel
	.p1vector

	.entry	main, ^m<>

; ---- 1. declare three handlers ----
	pushal	@#exh1
	calls	#1, @#sys$dclexh
	blbc	r0, fail
	pushal	@#exh3
	calls	#1, @#sys$dclexh
	blbc	r0, fail
	pushal	@#exh2
	calls	#1, @#sys$dclexh
	blbc	r0, fail

; ---- 2. cancel the middle one ----
	pushal	@#exh3
	calls	#1, @#sys$canexh
	blbc	r0, fail

; ---- 3. exit ----
	pushl	#^X2C
	calls	#1, @#sys$exit

; ---- 4. never reached ----
	movl	#1, @#returned
	ret

fail:	movl	#0, r0
	ret

; HANDLER1(status address): records when it ran and the status.
	.entry	handler1, ^m<r2>
	incl	@#count
	movl	@#count, @#order1
	movl	b^4(ap), r2		; the address of the exit status
	movl	(r2), @#status1
	ret

; HANDLER2(status address): the same, into its own cells.
	.entry	handler2, ^m<r2>
	incl	@#count
	movl	@#count, @#order2
	movl	b^4(ap), r2
	movl	(r2), @#status2
	ret

; HANDLER3: cancelled, so it must never run.
	.entry	handler3, ^m<>
	movl	#1, @#ran3
	ret

; Exit control blocks: forward link (VMS's), handler, argument count,
; then the first argument, the address of a status longword.
exh1:	.long	0, handler1, 1, exst1
exh2:	.long	0, handler2, 1, exst2
exh3:	.long	0, handler3, 1, exst3
exst1:	.long	0
exst2:	.long	0
exst3:	.long	0

count:	 .long	0
order1:	 .long	0
order2:	 .long	0
status1: .long	0
status2: .long	0
ran3:	 .long	0
returned: .long	0

	.end	main
