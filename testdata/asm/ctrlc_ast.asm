;
; testdata/asm/ctrlc_ast.asm -- docs/PHASE-26.md subtask 27's acceptance
; fixture: a program catching CTRL/C itself with terminal ASTs, instead
; of being interrupted.
;
;   1. $ASSIGN a channel to TTA0:.
;   2. $QIOW IO$_SETMODE!IO$M_CTRLCAST (^X123): CAST, parameter 5, runs
;      when CTRL/C is typed. Then spin at SPIN1 until it has run. (The Go
;      test types CTRL/C when the program reaches SPIN1.)
;   3. $QIOW IO$_SETMODE!IO$M_CTRLYAST (^XA3): YAST, parameter 9. With no
;      CTRL/C AST enabled (the first was one-shot), a CTRL/C typed at
;      SPIN2 is taken as CTRL/Y, so YAST runs.
;
; R0 is 1 at the end; the Go test checks the parameters each AST
; recorded.

	.microkernel
	.p1vector

	.entry	main, ^m<>

	mtpr	#0, #^X12		; IPL 0: ASTs can be delivered

; ---- 1. $ASSIGN(TTA0:) ----
	pushl	#0
	pushl	#0
	pushal	@#chan
	pushal	@#devnam
	calls	#4, @#sys$assign
	blbs	r0, ok1
	brw	fail
ok1:

; ---- 2. a CTRL/C AST ----
	pushl	#0			; p6
	pushl	#0			; p5
	pushl	#0			; p4
	pushl	#0			; p3: access mode (user, maximized)
	pushl	#5			; p2: the AST parameter
	pushal	@#cast			; p1: the AST routine
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushl	#0			; iosb
	pushl	#^X123			; IO$_SETMODE!IO$M_CTRLCAST
	movzwl	@#chan, -(sp)
	pushl	#0			; efn
	calls	#^D12, @#sys$qiow
	blbs	r0, spin1
	brw	fail
spin1:	tstl	@#seen			; wait for CTRL/C
	beql	spin1

; ---- 3. a CTRL/Y AST, which CTRL/C falls back to ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#9
	pushal	@#yast
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#^XA3			; IO$_SETMODE!IO$M_CTRLYAST
	movzwl	@#chan, -(sp)
	pushl	#0
	calls	#^D12, @#sys$qiow
	blbs	r0, spin2
	brw	fail
spin2:	tstl	@#yseen			; wait for CTRL/C, taken as CTRL/Y
	beql	spin2

	movl	#1, r0
	ret

fail:	clrl	r0
	ret

; The AST routines record their parameter.
	.entry	cast, ^m<>
	movl	b^4(ap), @#seen
	ret

	.entry	yast, ^m<>
	movl	b^4(ap), @#yseen
	ret

chan:	.word	0
devnam:	.ascid	"TTA0"
seen:	.long	0
yseen:	.long	0

	.end	main
