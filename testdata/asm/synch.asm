;
; testdata/asm/synch.asm -- docs/PHASE-26.md subtask 18's acceptance
; fixture: $SYNCH telling a request's completion apart from another use
; of the same event flag.
;
;   1. $GETJPI (not $GETJPIW) with event flag 3 and an IOSB, then
;      $SYNCH(3, IOSB1). The request already completed, so $SYNCH returns
;      at once, leaving flag 3 set.
;   2. A stand-in for an asynchronous request that shares event flag 4
;      with something else:
;        - timer A, 10ms, sets flag 4 (the "something else": a false
;          alarm, with IOSB2 still 0);
;        - timer B, 30ms, sets flag 4 and calls COMPLETE as an AST, which
;          stores SS$_NORMAL in IOSB2 (the "request" completing).
;      $SYNCH(4, IOSB2) wakes at 10ms, finds IOSB2 still 0, clears the
;      flag, and waits again; at 30ms it finds IOSB2 set and returns. The
;      Go test checks at least 30ms passed, and flag 4 is left set.
;
; R0 is 1 at the end if every step did what it should, 0 otherwise.
; Delta times are negative quadwords in 100ns units.

	.microkernel
	.p1vector

	.entry	main, ^m<>

	mtpr	#0, #^X12		; IPL 0: ASTs can be delivered

; ---- 1. $GETJPI(efn 3, JPI$_PID, IOSB1); $SYNCH(3, IOSB1) ----
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushal	@#iosb1
	pushal	@#itmlst
	pushl	#0			; prcnam
	pushl	#0			; pidadr
	pushl	#3			; efn
	calls	#7, @#sys$getjpi
	blbs	r0, ok1
	brw	fail
ok1:	pushal	@#iosb1
	pushl	#3
	calls	#2, @#sys$synch
	blbs	r0, ok2
	brw	fail
ok2:	cmpl	@#iosb1, #1		; the request's status
	beql	ok3
	brw	fail
ok3:	pushl	#0
	pushl	#3
	calls	#2, @#sys$readef
	cmpl	r0, #9			; SS$_WASSET: $SYNCH left flag 3 set
	beql	ok4
	brw	fail
ok4:

; ---- 2. a false alarm on flag 4, then the real completion ----
	pushl	#0			; flags
	pushl	#0			; reqidt
	pushl	#0			; no AST: just sets flag 4
	pushal	@#delta10
	pushl	#4
	calls	#5, @#sys$setimr	; timer A
	pushl	#0
	pushl	#0
	pushal	@#complete		; the "request" completing
	pushal	@#delta30
	pushl	#4
	calls	#5, @#sys$setimr	; timer B
	pushal	@#iosb2
	pushl	#4
	calls	#2, @#sys$synch
	blbs	r0, ok5
	brw	fail
ok5:	cmpl	@#iosb2, #1
	beql	ok6
	brw	fail
ok6:	pushl	#0
	pushl	#4
	calls	#2, @#sys$readef
	cmpl	r0, #9			; SS$_WASSET
	beql	ok7
	brw	fail
ok7:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; COMPLETE: the stand-in request's completion, writing its IOSB.
	.entry	complete, ^m<>
	movl	#1, @#iosb2
	ret

delta10: .long	^XFFFE7960, ^XFFFFFFFF	; -100000: 10ms
delta30: .long	^XFFFB6C20, ^XFFFFFFFF	; -300000: 30ms

iosb1:	.long	0, 0
iosb2:	.long	0, 0
pid:	.long	0

; A $GETJPI item list: JPI$_PID (^X319) into PID, then the terminator.
itmlst:	.word	4, ^X319
	.long	pid, 0
	.long	0

	.end	main
