;
; testdata/asm/callback.asm -- docs/PHASE-58 subtask 2's fixture: a
; LIB$-style routine in Go that calls back into the program.
;
; TEST$CALLBACK routine, count, value is the test's own routine
; (internal/console's TestCallback_assembledProgram registers it, Go
; code behind a stub, as a LIB$ routine is): it calls routine count
; times, as CALLG (i, value) for i from 1 to count, and returns the sum
; of the R0s it got back.
;
;   1. A plain loop: CB1 returns i*value, so (CB1, 3, 10) is
;      10+20+30 = 60 (SUM1), and CB1 runs three times (CALLS1).
;   2. A callback that calls the routine again: CB2(i) returns
;      TEST$CALLBACK(CB1, 2, i), 1*i+2*i = 3i, so (CB2, 2, 0) is 3+6 = 9
;      (SUM2).
;   3. A callback that never returns: SUB establishes LIB$SIG_TO_RET and
;      calls (CB3, 2, 0); CB3 signals, and LIB$SIG_TO_RET unwinds to
;      MAIN, past the routine's stub frame, with the condition as R0
;      (RET3). The routine is abandoned (the test checks).
;   4. Then the routine is called again, (CB1, 1, 5): 5 (SUM4); and SP is
;      where it was before the calls (SPOK).
;
; The .SHIM lines give the program the stubs; the LIB$ ones are skipped
; when the kernel, assembled first, has already defined them.

	.microkernel
	.p1vector
	.iif not_defined lib$signal, .shim lib$signal, ^d33, LIBRTL, ^X04F0
	.iif not_defined lib$establish, .shim lib$establish, ^d35, LIBRTL, ^X03C0
	.iif not_defined lib$sig_to_ret, .shim lib$sig_to_ret, ^d37, LIBRTL, ^X0500
	.shim	test$callback, ^d999, TEST, ^X0000

	.entry	main, ^m<>
	movl	sp, @#spbefore
	pushl	#10
	pushl	#3
	pushal	@#cb1
	calls	#3, @#test$callback
	movl	r0, @#sum1

	pushl	#0
	pushl	#2
	pushal	@#cb2
	calls	#3, @#test$callback
	movl	r0, @#sum2

	calls	#0, @#sub
	movl	r0, @#ret3

	pushl	#5
	pushl	#1
	pushal	@#cb1
	calls	#3, @#test$callback
	movl	r0, @#sum4

	cmpl	sp, @#spbefore
	bneq	spbad
	movl	#1, @#spok
spbad:	movl	#1, r0
	ret

; CB1 (i, value): i*value.
	.entry	cb1, ^m<>
	incl	@#calls1
	mull3	4(ap), 8(ap), r0
	ret

; CB2 (i, value): TEST$CALLBACK(CB1, 2, i).
	.entry	cb2, ^m<>
	pushl	4(ap)
	pushl	#2
	pushal	@#cb1
	calls	#3, @#test$callback
	ret

; CB3: signal a condition, which SUB's handler turns into SUB's return.
	.entry	cb3, ^m<>
	pushl	#^X08018002
	calls	#1, @#lib$signal
	movl	#^X99, r0		; never: the unwind removes this frame
	ret

	.entry	sub, ^m<>
	pushal	@#lib$sig_to_ret
	calls	#1, @#lib$establish
	pushl	#0
	pushl	#2
	pushal	@#cb3
	calls	#3, @#test$callback
	movl	#^X77, r0		; never
	ret

sum1:	.long	0
sum2:	.long	0
ret3:	.long	0
sum4:	.long	0
calls1:	.long	0
spbefore: .long	0
spok:	.long	0

	.end	main
