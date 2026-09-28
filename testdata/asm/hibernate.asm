;
; testdata/asm/hibernate.asm -- docs/PHASE-26.md subtask 14's acceptance
; fixture for $HIBER, $WAKE, $SCHDWK, and $CANWAK. Like
; timer_services.asm, it does no interrupt setup: scheduled wakeups run on
; the RTL's timer queue and the engine's system time.
;
;   1. $WAKE, then $HIBER: the wakeup is already pending, so $HIBER
;      returns at once,
;   2. $SCHDWK: first wakeup in 30ms, then every 10ms,
;   3. $HIBER three times: woken at 30, 40, and 50ms,
;   4. $CANWAK: the repeating wakeup is cancelled.
;
; R0 is 1 at the end if every call succeeded, 0 otherwise. The Go test
; checks that at least 50ms of emulated time passed and nothing is left
; queued. Delta times are negative quadwords in 100ns units.

	.microkernel
	.p1vector

	.entry	main, ^m<r2>

; ---- 1. $WAKE(), then $HIBER() returns at once ----
	pushl	#0
	pushl	#0
	calls	#2, @#sys$wake
	blbs	r0, ok1
	brw	fail
ok1:
	calls	#0, @#sys$hiber
	blbs	r0, ok2
	brw	fail
ok2:

; ---- 2. $SCHDWK(daytim=30ms, reptim=10ms) ----
	pushal	@#delta10
	pushal	@#delta30
	pushl	#0
	pushl	#0
	calls	#4, @#sys$schdwk
	blbs	r0, ok3
	brw	fail
ok3:

; ---- 3. hibernate three times ----
	movl	#3, r2
again:	calls	#0, @#sys$hiber
	blbs	r0, ok4
	brw	fail
ok4:	sobgtr	r2, again

; ---- 4. $CANWAK() ----
	pushl	#0
	pushl	#0
	calls	#2, @#sys$canwak
	blbs	r0, ok5
	brw	fail
ok5:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

delta10: .long	^XFFFE7960, ^XFFFFFFFF	; -100000: 10ms
delta30: .long	^XFFFB6C20, ^XFFFFFFFF	; -300000: 30ms

	.end	main
