;
; testdata/asm/unwind.asm -- docs/PHASE-26.md subtask 34's acceptance
; fixture: unwinding the call stack from a condition handler.
;
;   1. SUB1 establishes LIB$SIG_TO_RET as its handler and calls SUB2,
;      which reads address 0: an access violation. LIB$SIG_TO_RET (depth
;      1) unwinds to SUB1's caller with the condition value as R0, so SUB1
;      "returns" SS$_ACCVIO to MAIN (RET1).
;   2. MAIN sets R2 to ^X1234 and calls SUB3 with one argument. SUB3 saves
;      R2 (its entry mask), sets it to ^X99, establishes H3, and calls
;      SUB4, which establishes H4 and calls LIB$STOP. H4 resignals; H3
;      (depth 1) sets the mechanism array's R0 to ^X42 and calls $UNWIND
;      with depth 2 (SUB4 and SUB3) and a new PC, RECOVER, in MAIN. As the
;      frames go, H4 and H3 are each called with SS$_UNWIND (UNW4, UNW3).
;      MAIN resumes at RECOVER with R0 = ^X42 (RET3), R2 = ^X1234 again
;      (SUB3's RET restored it: SAVEDR2), and SP where it was before the
;      CALLS pushed SUB3's argument (SPOK).
;
; The .SHIM lines give the program the LIB$ routines' stubs, as the
; booted kernel.asm's own .SHIM table does.

	.microkernel
	.p1vector
	.scb	exc$accvio, console$handler
	.shim	lib$stop,	^d34, LIBRTL, 04F8
	.shim	lib$establish,	^d35, LIBRTL, 03C0
	.shim	lib$sig_to_ret,	^d37, LIBRTL, 0500

	.entry	main, ^m<r2>
	calls	#0, @#sub1
	movl	r0, @#ret1
	movl	#^X1234, r2
	movl	sp, @#spbefore
	pushl	#7
	calls	#1, @#sub3
	movl	#1, @#reached		; skipped: the unwind resumes at RECOVER
recover:
	movl	r0, @#ret3
	movl	r2, @#savedr2
	cmpl	sp, @#spbefore
	bneq	spbad
	movl	#1, @#spok
spbad:	movl	#1, r0
	ret

	.entry	sub1, ^m<>
	pushal	@#lib$sig_to_ret
	calls	#1, @#lib$establish
	calls	#0, @#sub2
	movl	#^X99, r0		; never: SUB2 doesn't return
	ret

	.entry	sub2, ^m<>
	movl	@#0, r1			; access violation
	ret

	.entry	sub3, ^m<r2>
	movl	#^X99, r2
	movab	h3, (fp)
	calls	#0, @#sub4
	ret

	.entry	sub4, ^m<>
	movab	h4, (fp)
	pushl	#^X08018004
	calls	#1, @#lib$stop

; H4: count SS$_UNWIND calls, resignal anything else.
	.entry	h4, ^m<>
	movl	4(ap), r0
	cmpl	4(r0), #^X920		; SS$_UNWIND?
	bneq	h4res
	incl	@#unw4
h4res:	movl	#^X918, r0		; SS$_RESIGNAL
	ret

; H3: count SS$_UNWIND calls; for the stop, unwind two frames to RECOVER.
	.entry	h3, ^m<>
	movl	4(ap), r0
	cmpl	4(r0), #^X920		; SS$_UNWIND?
	bneq	h3stop
	incl	@#unw3
	ret
h3stop:	movl	8(ap), r0
	movl	#^X42, ^X0C(r0)		; the R0 MAIN resumes with
	pushal	@#recover		; newpc
	pushal	@#depth			; depadr
	calls	#2, @#sys$unwind
	ret

depth:	.long	2
ret1:	.long	0
ret3:	.long	0
savedr2: .long	0
spbefore: .long	0
spok:	.long	0
unw3:	.long	0
unw4:	.long	0
reached: .long	0

	.end	main
