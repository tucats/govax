;
; testdata/asm/process_control.asm -- docs/PHASE-26.md subtask 30's
; acceptance fixture: $SETPRN, $SETPRI, and $FORCEX on the program's own
; process.
;
;   1. $SETPRN "WORKER".
;   2. $SETPRI to priority 6, returning the previous base priority in
;      OLDPRI.
;   3. REI down to user mode, declare an exit handler (EXH), and
;      $FORCEX(code ^X2C) itself. The forced exit arrives as a user-mode
;      AST right after the call: $EXIT runs, calls EXH with the status,
;      and ends the image, so the code after $FORCEX never runs.
;
; The Go test checks the process name and priority, OLDPRI, that EXH
; saw ^X2C, that REACHED is still 0, and that R0 is the exit status.

	.microkernel
	.p1vector

	.entry	main, ^m<>

	mtpr	#0, #^X12		; IPL 0: ASTs can be delivered

; ---- 1. $SETPRN("WORKER") ----
	pushal	@#prcnam
	calls	#1, @#sys$setprn
	blbs	r0, ok1
	brw	fail
ok1:

; ---- 2. $SETPRI(pri 6, prvpri OLDPRI) ----
	pushal	@#oldpri		; prvpri
	pushl	#6			; pri
	pushl	#0			; prcnam
	pushl	#0			; pidadr: this process
	calls	#4, @#sys$setpri
	blbs	r0, ok2
	brw	fail
ok2:

; ---- 3. user mode, an exit handler, and $FORCEX ----
	pushl	#^X03C00000		; PSL: current and previous mode user, IPL 0
	pushal	@#usercode
	rei

usercode:
	pushal	@#desblk
	calls	#1, @#sys$dclexh
	blbs	r0, ok3
	brw	fail
ok3:	pushl	#^X2C			; code
	pushl	#0			; prcnam
	pushl	#0			; pidadr
	calls	#3, @#sys$forcex
	movl	#1, @#reached		; never: the forced exit comes first
	movl	#1, r0
	ret

fail:	clrl	r0
	ret

; The exit handler records the status it's given.
	.entry	exh, ^m<>
	movl	@4(ap), @#seen
	ret

; The exit control block: link, handler, argument count, status address.
desblk:	.long	0
	.long	exh
	.long	1
	.long	status

status:	.long	0
seen:	.long	0
reached: .long	0
oldpri:	.long	^XFF
prcnam:	.ascid	"WORKER"

	.end	main
