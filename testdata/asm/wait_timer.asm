;
; testdata/asm/wait_timer.asm -- docs/PHASE-26.md subtask 9's acceptance
; fixture for the event-flag wait services. It waits for an event flag
; that only the interval timer's interrupt handler sets -- the timer being
; govax's one truly asynchronous event source -- so it shows a waiting
; process being woken by an asynchronous event:
;
;   1. installs "tick" as the interval-timer interrupt handler (.SCB),
;   2. starts the interval clock with interrupts enabled, and lowers IPL
;      to 0 so the timer's IPL 22 interrupt can be taken,
;   3. waits for event flag 3 ($WAITFR); the process sits in the wait
;      until the handler runs,
;   4. on its first tick, the handler sets flag 3 ($SETEF), counts the
;      tick, and stops the clock,
;   5. back in the main program, $WAITFR completes. $WFLAND and $WFLOR on
;      flags that are already set complete at once.
;
; R0 is 1 at the end if every step did what it should, 0 otherwise.

	.microkernel
	.p1vector

	.scb	exc$interval, tick

	.entry	main, ^m<>

; ---- 1-2. start the clock: every 10 ticks, interrupts enabled ----
	mcoml	#^D10, r0
	mtpr	r0, #VAX$PR_NICR
	mtpr	#^X51, #VAX$PR_ICCS	; RUN, XFR (reload ICR), IE
	mtpr	#0, #^X12		; IPL 0

; ---- 3. wait for flag 3 ----
	pushl	#3
	calls	#1, @#sys$waitfr
	cmpl	r0, #1
	beql	ok1
	brw	fail
ok1:
	tstl	@#ticks			; the wait ended because a tick ran
	bneq	ok2
	brw	fail
ok2:

; ---- 5. waits already satisfied: flags 3 and 5 set ----
	pushl	#5
	calls	#1, @#sys$setef
	pushl	#^X28			; flags 3 and 5
	pushl	#0
	calls	#2, @#sys$wfland
	cmpl	r0, #1
	beql	ok3
	brw	fail
ok3:
	pushl	#^X30			; flags 4 and 5: 5 is set
	pushl	#0
	calls	#2, @#sys$wflor
	cmpl	r0, #1
	beql	ok4
	brw	fail
ok4:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; ---- 4. interval timer interrupt handler ----
	.align	8
tick:	pushr	#^M<r0,r1>
	incl	@#ticks
	pushl	#3
	calls	#1, @#sys$setef
	mtpr	#^X80, #VAX$PR_ICCS	; clear INT, stop the clock
	popr	#^M<r0,r1>
	rei

ticks:	.long	0

	.end	main
