;
; testdata/asm/signals.asm -- docs/PHASE-26.md subtask 33's acceptance
; fixture: software conditions.
;
;   1. MAIN establishes HANDLER with LIB$ESTABLISH (OLDH: the previous
;      handler, 0) and calls SUB1, which signals the customer condition
;      ^X08018003 (informational) with one FAO argument, 42. The search
;      starts at SUB1's frame (depth 0, no handler); HANDLER, one frame
;      up, records the signal array's count and argument and its depth,
;      sets the mechanism array's R0 to ^X77, and continues: LIB$SIGNAL
;      returns ^X77 (RET1).
;   2. MAIN signals SS$_ENDOFFILE, a warning HANDLER resignals. The
;      catch-all prints %SYSTEM-W-ENDOFFILE and, the condition not being
;      severe, LIB$SIGNAL returns (RET2: the mechanism array's R0, 0).
;   3. LIB$REVERT removes HANDLER, returning it (REVERTED).
;   4. LIB$MATCH_COND finds C0's match among C1-C3 (MATCHED: 2, C2, which
;      differs from C0 only in severity and control bits).
;   5. SUB2 establishes STOPH and calls LIB$STOP. STOPH tries to
;      continue, so the image exits with "%LIB-F-ATTCONSTO, attempt to
;      continue from stop" and status ^X18018004 (the condition, forced
;      SEVERE, message inhibited). REACHED stays 0.
;
; The .SHIM lines give the program the LIB$ routines' stubs, as the
; booted kernel.asm's own .SHIM table does.

	.microkernel
	.p1vector
	.shim	lib$signal,	^d33, LIBRTL, 04F0
	.shim	lib$stop,	^d34, LIBRTL, 04F8
	.shim	lib$establish,	^d35, LIBRTL, 03C0
	.shim	lib$revert,	^d36, LIBRTL, 0490
	.shim	lib$match_cond,	^d38, LIBRTL, 0460

	.entry	main, ^m<>
	pushal	@#handler
	calls	#1, @#lib$establish
	movl	r0, @#oldh
	calls	#0, @#sub1
	movl	r0, @#ret1
	pushl	#^X870			; SS$_ENDOFFILE
	calls	#1, @#lib$signal
	movl	r0, @#ret2
	calls	#0, @#lib$revert
	movl	r0, @#reverted
	pushal	@#c3
	pushal	@#c2
	pushal	@#c1
	pushal	@#c0
	calls	#4, @#lib$match_cond
	movl	r0, @#matched
	calls	#0, @#sub2		; never returns
	movl	#1, @#reached
	ret

	.entry	sub1, ^m<>
	pushl	#^X2A			; FAO argument: 42
	pushl	#1			; one FAO argument
	pushl	#^X08018003		; the condition
	calls	#3, @#lib$signal
	ret				; with LIB$SIGNAL's R0

	.entry	sub2, ^m<>
	pushal	@#stoph
	calls	#1, @#lib$establish
	pushl	#^X08018003
	calls	#1, @#lib$stop
	movl	#1, @#reached		; never
	ret

	.entry	handler, ^m<r2,r3>
	movl	4(ap), r2		; signal array
	movl	8(ap), r3		; mechanism array
	cmpl	4(r2), #^X08018003
	bneq	resig
	movl	(r2), @#sigcount
	movl	^X0C(r2), @#sigarg	; after the condition and the FAO count
	movl	8(r3), @#depth
	movl	#^X77, ^X0C(r3)		; LIB$SIGNAL's return value
	movl	#1, r0			; SS$_CONTINUE
	ret
resig:	movl	#^X918, r0		; SS$_RESIGNAL
	ret

	.entry	stoph, ^m<>
	incl	@#stopcalls
	movl	#1, r0			; try to continue
	ret

oldh:	.long	^XFF
ret1:	.long	0
ret2:	.long	^XFF
reverted: .long	0
matched: .long	0
sigcount: .long	0
sigarg:	.long	0
depth:	.long	^XFF
stopcalls: .long 0
reached: .long	0
c0:	.long	^X0000000C
c1:	.long	^X00000870
c2:	.long	^X1000000A
c3:	.long	^X0000000C

	.end	main
