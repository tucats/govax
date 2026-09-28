;
; testdata/asm/condition_exit.asm -- docs/PHASE-26.md subtask 31's
; acceptance fixture for a condition nobody handles.
;
; In user mode, the program declares an exit handler, then calls SUB,
; which reads address 0 (the no-access guard page). No frame has a
; condition handler, so the catch-all reports
;
;	%SYSTEM-F-ACCVIO, access violation, reason mask=..., ...
;
; and, the condition being severe, ends the image through $EXIT with
; status SS$_ACCVIO + STS$M_INHIB_MSG (^X1000000C). The exit handler
; records the status; the code after the call never runs.

	.microkernel
	.p1vector

; Send the exceptions to console$handler, as kernel.asm's SCB does in a
; booted system: the RTL's condition dispatcher takes them from there.
	.scb	exc$accvio, console$handler

	.entry	main, ^m<>
	pushl	#^X03C00000		; PSL: current and previous mode user
	pushal	@#usercode
	rei

usercode:
	pushal	@#desblk
	calls	#1, @#sys$dclexh
	calls	#0, @#sub
	movl	#1, @#reached		; never
	movl	#1, r0
	ret

	.entry	sub, ^m<>
	movl	@#0, r1			; access violation
	ret

	.entry	exh, ^m<>
	movl	@4(ap), @#seen
	ret

desblk:	.long	0
	.long	exh
	.long	1
	.long	status

status:	.long	0
seen:	.long	0
reached: .long	0

	.end	main
