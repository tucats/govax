;
; testdata/asm/jpi_ast_state.asm -- docs/PHASE-26.md subtask 21's
; acceptance fixture: $GETJPI's AST and state items, read at moments
; when they differ. Run in kernel mode, so the kernel bit (bit 0) is the
; one that changes.
;
;   1. Normally: ASTEN, ASTACT, ASTCNT, and STATE into the NORMAL block.
;   2. With ASTs disabled ($SETAST(0)): ASTEN into ASTENOFF.
;   3. Inside an AST routine (queued by $DCLAST): ASTACT into ASTACTIN.
;   4. With a timer AST pending (an hour away, then cancelled): ASTCNT
;      into ASTCNTTMR.
;
; R0 is 1 at the end if every call succeeded; the Go test checks the
; values. Item codes: JPI$_ASTACT ^X300, JPI$_ASTEN ^X301, JPI$_STATE
; ^X306, JPI$_ASTCNT ^X30E.

	.microkernel
	.p1vector

	.entry	main, ^m<>

	mtpr	#0, #^X12		; IPL 0: ASTs can be delivered

; ---- 1. the normal state ----
	pushal	@#normlst
	calls	#1, @#getjpi
	blbs	r0, ok1
	brw	fail
ok1:

; ---- 2. ASTs disabled ----
	pushl	#0
	calls	#1, @#sys$setast
	pushal	@#offlst
	calls	#1, @#getjpi
	blbs	r0, ok2
	brw	fail
ok2:
	pushl	#1
	calls	#1, @#sys$setast

; ---- 3. inside an AST ----
	pushl	#0			; acmode
	pushl	#0			; astprm
	pushal	@#astrtn
	calls	#3, @#sys$dclast	; delivered before the next instruction
	blbs	r0, ok3
	brw	fail
ok3:
	cmpl	@#astran, #1
	beql	ok4
	brw	fail
ok4:

; ---- 4. a timer AST outstanding ----
	pushl	#0			; flags
	pushl	#^X55			; reqidt
	pushal	@#astrtn
	pushal	@#hour
	pushl	#0
	calls	#5, @#sys$setimr
	blbs	r0, ok5
	brw	fail
ok5:
	pushal	@#tmrlst
	calls	#1, @#getjpi
	blbs	r0, ok6
	brw	fail
ok6:
	pushl	#0
	pushl	#^X55
	calls	#2, @#sys$cantim

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; GETJPI(itmlst): $GETJPIW for this process with the given item list.
	.entry	getjpi, ^m<>
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushl	#0			; iosb
	pushl	b^4(ap)			; itmlst
	pushl	#0			; prcnam
	pushl	#0			; pidadr
	pushl	#0			; efn
	calls	#7, @#sys$getjpiw
	ret

; ASTRTN: reads ASTACT while this AST is active.
	.entry	astrtn, ^m<>
	pushal	@#inlst
	calls	#1, @#getjpi
	movl	#1, @#astran
	ret

hour:	.long	^X9E3B9800, ^XFFFFFFF7	; -36000000000: one hour

astran:	   .long 0
asten:	   .long 0
astact:	   .long 0
astcnt:	   .long 0
state:	   .long 0
astenoff:  .long 0
astactin:  .long 0
astcnttmr: .long 0

; Item lists: buffer length, item code, buffer, return length address.
normlst: .word	4, ^X301
	.long	asten, 0
	.word	4, ^X300
	.long	astact, 0
	.word	4, ^X30E
	.long	astcnt, 0
	.word	4, ^X306
	.long	state, 0
	.long	0
offlst:	.word	4, ^X301
	.long	astenoff, 0
	.long	0
inlst:	.word	4, ^X300
	.long	astactin, 0
	.long	0
tmrlst:	.word	4, ^X30E
	.long	astcnttmr, 0
	.long	0

	.end	main
