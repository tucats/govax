;
; testdata/asm/timer_ast.asm -- docs/PHASE-26.md subtask 16's acceptance
; fixture: ASTs from $SETIMR and $GETJPI, in the patterns VMS programs
; use them.
;
;   1. $SETIMR with an AST, then $HIBER: the classic "sleep until the
;      timer". The AST routine (TMRAST) records its parameter (the
;      timer's reqidt, 77) and calls $WAKE, which ends the $HIBER it
;      interrupted. The timer's event flag is set too.
;   2. A $SETIMR with an AST cancelled by $CANTIM never delivers it:
;      the recorded parameter is still 77 after waiting past its time.
;   3. A timer AST interrupts a loop that calls no services at all: the
;      loop spins until the AST routine (SETDONE) sets DONE.
;   4. $GETJPI with an AST: it completes at once, so its AST routine
;      (JPIAST) has run, with astprm 5, by the time the call returns.
;
; No interrupt setup is needed: timers run on the engine's system time,
; and ASTs are delivered at IPL 0. R0 is 1 at the end if every step did
; what it should, 0 otherwise. Delta times are negative quadwords in
; 100ns units.

	.microkernel
	.p1vector

	.entry	main, ^m<>

	mtpr	#0, #^X12		; IPL 0: ASTs can be delivered

; ---- 1. $SETIMR(efn=4, 30ms, TMRAST, reqidt=77); $HIBER ----
	pushl	#0			; flags
	pushl	#^X77			; reqidt: the AST's parameter
	pushal	@#tmrast		; astadr
	pushal	@#delta30
	pushl	#4
	calls	#5, @#sys$setimr
	blbs	r0, ok1
	brw	fail
ok1:	calls	#0, @#sys$hiber		; woken by TMRAST's $WAKE
	cmpl	@#astparam, #^X77
	beql	ok2
	brw	fail
ok2:	pushal	@#efstate		; the state longword (VMS requires it)
	pushl	#4
	calls	#2, @#sys$readef
	cmpl	r0, #9			; SS$_WASSET: the timer set flag 4 too
	beql	ok3
	brw	fail
ok3:

; ---- 2. a cancelled timer's AST never runs ----
	pushl	#0
	pushl	#^X88
	pushal	@#tmrast
	pushal	@#delta10
	pushl	#5
	calls	#5, @#sys$setimr	; would record 88
	pushl	#0
	pushl	#^X88
	calls	#2, @#sys$cantim
	pushl	#0
	pushl	#0
	pushl	#0
	pushal	@#delta30
	pushl	#6
	calls	#5, @#sys$setimr	; no AST: just a flag to wait on
	pushl	#6
	calls	#1, @#sys$waitfr	; 30ms: well past the cancelled 10ms
	cmpl	@#astparam, #^X77
	beql	ok4
	brw	fail
ok4:

; ---- 3. an AST interrupts a loop with no service calls ----
	pushl	#0
	pushl	#0
	pushal	@#setdone
	pushal	@#delta10
	pushl	#7
	calls	#5, @#sys$setimr
spin:	tstl	@#done
	beql	spin

; ---- 4. $GETJPIW(itmlst=PID, astadr=JPIAST, astprm=5) ----
	pushl	#5			; astprm
	pushal	@#jpiast		; astadr
	pushl	#0			; iosb
	pushal	@#itmlst
	pushl	#0			; prcnam
	pushl	#0			; pidadr
	pushl	#0			; efn
	calls	#7, @#sys$getjpiw
	blbs	r0, ok5
	brw	fail
ok5:	cmpl	@#jpiparam, #5		; the AST already ran
	beql	ok6
	brw	fail
ok6:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; TMRAST(reqidt): records its parameter and wakes the process.
	.entry	tmrast, ^m<>
	movl	b^4(ap), @#astparam
	pushl	#0
	pushl	#0
	calls	#2, @#sys$wake
	ret

; SETDONE: ends step 3's loop.
	.entry	setdone, ^m<>
	movl	#1, @#done
	ret

; JPIAST(astprm): records its parameter.
	.entry	jpiast, ^m<>
	movl	b^4(ap), @#jpiparam
	ret

delta10: .quad	-^D100000		; 10ms
delta30: .quad	-^D300000		; 30ms

astparam:	.long	0
jpiparam:	.long	0
done:		.long	0
pid:		.long	0
efstate: .long	0

; A $GETJPI item list: JPI$_PID (^X319) into PID, then the terminator.
itmlst:	.word	4, ^X319
	.long	pid, 0
	.long	0

	.end	main
