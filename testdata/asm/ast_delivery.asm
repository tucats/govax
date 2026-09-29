;
; testdata/asm/ast_delivery.asm -- docs/PHASE-26.md subtask 15's
; acceptance fixture for AST delivery, $DCLAST, and $SETAST.
;
; ASTs are only delivered below IPL 2. A console after a normal boot
; (kernel.asm) or VMINIT is already at IPL 0; the program sets it anyway,
; so it doesn't depend on how it was started. (Changing that MTPR to IPL 2
; makes the program fail: no AST is delivered.)
;
;   1. $DCLAST(bump, 5): the AST runs as soon as the service returns.
;      The AST routine adds its parameter to COUNTER and deliberately
;      clobbers R0 and R1; the program checks both come back intact
;      (R0 is $DCLAST's SS$_NORMAL, R1 what it set before the call),
;      along with R2 (saved by the routine's entry mask).
;   2. $SETAST(0) returns SS$_WASSET (9); a second $DCLAST(bump, 10)
;      stays queued: COUNTER is still 5.
;   3. $SETAST(1) returns SS$_WASCLR (1), and the queued AST runs at
;      once: COUNTER becomes 15.
;   4. The routine records its argument count (5: astprm, R0, R1, PC,
;      PSL), for the Go test to check.
;
; R0 is 1 at the end if everything held, 0 otherwise.

	.microkernel
	.p1vector

	.entry	main, ^m<r2>

	mtpr	#0, #^X12		; IPL 0: ASTs can be delivered

; ---- 1. $DCLAST(bump, 5): delivered at once ----
	movl	#^X1234, r2
	movl	#^X5678, r1
	pushl	#0			; acmode: the caller's
	pushl	#5			; astprm
	pushal	@#bump			; astadr
	calls	#3, @#sys$dclast
	cmpl	r0, #1			; $DCLAST's status, restored after the AST
	beql	ok1
	brw	fail
ok1:	cmpl	@#counter, #5
	beql	ok2
	brw	fail
ok2:	cmpl	r2, #^X1234		; preserved by the routine's entry mask
	beql	ok2a
	brw	fail
ok2a:	cmpl	r1, #^X5678		; restored from the AST frame
	beql	ok3
	brw	fail
ok3:

; ---- 2. $SETAST(0): the next AST waits ----
	pushl	#0
	calls	#1, @#sys$setast
	cmpl	r0, #9			; SS$_WASSET: they were enabled
	beql	ok4
	brw	fail
ok4:	pushl	#0
	pushl	#^X10
	pushal	@#bump
	calls	#3, @#sys$dclast
	cmpl	@#counter, #5		; not delivered yet
	beql	ok5
	brw	fail
ok5:

; ---- 3. $SETAST(1): delivered now ----
	pushl	#1
	calls	#1, @#sys$setast
	cmpl	r0, #1			; SS$_WASCLR: they were disabled
	beql	ok6
	brw	fail
ok6:	cmpl	@#counter, #^X15
	beql	ok7
	brw	fail
ok7:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; The AST routine: COUNTER += astprm, and wreck R0/R1, which AST
; delivery must restore. Also notes its argument count.
	.entry	bump, ^m<r2>
	addl2	b^4(ap), @#counter
	movl	(ap), @#argcount
	movl	#^XDEAD, r2
	movl	#^XBAD0, r0
	movl	#^XBAD1, r1
	ret

counter:	.long	0
argcount:	.long	0

	.end	main
