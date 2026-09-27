;
; testdata/asm/timer_services.asm -- docs/PHASE-26.md subtask 11's
; acceptance fixture for $SETIMR and $CANTIM. Unlike wait_timer.asm, it
; never touches the interval clock's ICCS register, installs no interrupt
; handler, and leaves IPL alone: the RTL's timers run on the engine's
; system time directly, not on the guest's timer interrupt.
;
;   1. sets a 20ms timer on event flag 5 with request ID 7 ($SETIMR),
;   2. cancels it ($CANTIM 7),
;   3. sets a 50ms timer on event flag 4 ($SETIMR), and waits for it
;      ($WAITFR), which lasts until 50ms of emulated time have passed,
;   4. checks that flag 5 is still clear ($READEF): the cancelled timer,
;      though its 20ms passed during the wait, never fired.
;
; R0 is 1 at the end if every step did what it should, 0 otherwise.
; SS$_WASCLR is 1. Delta times are negative quadwords in 100ns units.

	.microkernel
	.p1vector

	.entry	main, ^m<>

; ---- 1. $SETIMR(efn=5, 20ms, astadr=0, reqidt=7) ----
	pushl	#0
	pushl	#7
	pushl	#0
	pushal	@#delta20
	pushl	#5
	calls	#5, @#sys$setimr
	blbs	r0, ok1
	brw	fail
ok1:

; ---- 2. $CANTIM(reqidt=7, acmode=caller's) ----
	pushl	#0
	pushl	#7
	calls	#2, @#sys$cantim
	blbs	r0, ok2
	brw	fail
ok2:

; ---- 3. $SETIMR(efn=4, 50ms), then $WAITFR(4) ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushal	@#delta50
	pushl	#4
	calls	#5, @#sys$setimr
	blbs	r0, ok3
	brw	fail
ok3:
	pushl	#4
	calls	#1, @#sys$waitfr
	blbs	r0, ok4
	brw	fail
ok4:

; ---- 4. flag 5 never set ----
	pushl	#0
	pushl	#5
	calls	#2, @#sys$readef
	cmpl	r0, #1
	beql	ok5
	brw	fail
ok5:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

delta20: .long	^XFFFCF2C0, ^XFFFFFFFF	; -200000: 20ms
delta50: .long	^XFFF85EE0, ^XFFFFFFFF	; -500000: 50ms

	.end	main
