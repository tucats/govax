;
; testdata/asm/mode_switch_ast.asm -- docs/PHASE-26.md subtask 22's
; acceptance fixture: a kernel-mode AST interrupting user-mode code, by
; switching the CPU into kernel mode and back.
;
;   1. In kernel mode, $SETIMR with an AST: the timer is set from kernel
;      mode, so its AST (KAST) is a kernel-mode AST.
;   2. REI down to user mode (the user stack is the one VMINIT set up).
;   3. In user mode, $HIBER. When the timer expires, its kernel AST is
;      delivered by switching to kernel mode: KAST records the mode it
;      runs in and the previous mode (from its PSL), and its parameter,
;      then calls $WAKE. The AST exit switches back to user mode, where
;      $HIBER runs again and returns.
;   4. Back in user mode, record the mode again and return.
;
; The Go test checks KAST ran in kernel mode with user as the previous
; mode, got parameter 7, and that the code after $HIBER ran in user mode.
; The assembler's radix is hex, so decimal numbers above 9 are ^D.

	.microkernel
	.p1vector

	.entry	main, ^m<>

	mtpr	#0, #^X12		; IPL 0: ASTs can be delivered

; ---- 1. a kernel-mode timer AST, 10ms away ----
	pushl	#0			; flags
	pushl	#7			; reqidt: the AST's parameter
	pushal	@#kast
	pushal	@#delta10
	pushl	#2			; efn
	calls	#5, @#sys$setimr

; ---- 2. drop to user mode ----
	pushl	#^X03C00000		; PSL: current and previous mode user, IPL 0
	pushal	@#usercode		; PC
	rei

; ---- 3. hibernate in user mode ----
usercode:
	calls	#0, @#sys$hiber		; ended by the kernel AST's $WAKE

; ---- 4. back in user mode ----
	movpsl	r0
	extzv	#^D24, #2, r0, @#umode	; PSL<CUR_MOD>
	movl	#1, r0
	ret

; KAST(reqidt): runs in kernel mode, interrupting user mode.
	.entry	kast, ^m<r2>
	movpsl	r2
	extzv	#^D24, #2, r2, @#kmode	; PSL<CUR_MOD>: kernel, 0
	extzv	#^D22, #2, r2, @#kprv	; PSL<PRV_MOD>: user, 3
	movl	b^4(ap), @#kparam
	pushl	#0
	pushl	#0
	calls	#2, @#sys$wake
	ret

delta10: .long	^XFFFE7960, ^XFFFFFFFF	; -100000: 10ms

kmode:	.long	^XFF
kprv:	.long	^XFF
kparam:	.long	0
umode:	.long	^XFF

	.end	main
