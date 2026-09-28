;
; testdata/asm/exit_on_return.asm -- docs/PHASE-26.md subtask 19: an
; image whose main routine declares an exit handler and then simply
; returns, with status 7, instead of calling $EXIT. As on VMS, the image
; driver that called main calls $EXIT with that status, so the handler
; still runs, and sees 7. Run in user mode by the Go test, through RUN's
; image driver.

	.microkernel
	.p1vector

	.entry	main, ^m<>
	pushal	@#exh
	calls	#1, @#sys$dclexh
	movl	#7, r0
	ret

; HANDLER(status address): records the exit status.
	.entry	handler, ^m<r2>
	movl	b^4(ap), r2
	movl	(r2), @#seen
	ret

exh:	.long	0, handler, 1, exst
exst:	.long	0
seen:	.long	0

	.end	main
