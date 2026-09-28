;
; testdata/asm/cmkrnl.asm -- docs/PHASE-26.md subtask 26's acceptance
; fixture: a user-mode program running routines of its own in kernel and
; executive mode with $CMKRNL and $CMEXEC.
;
;   1. REI down to user mode. (Executive mode's stack is the one VMINIT
;      sets up: see docs/MODE-STACKS.md.)
;   2. $CMKRNL(KRNL, ARGS): KRNL runs in kernel mode. It records its mode
;      and previous mode (from the PSL), executes MFPR -- a privileged
;      instruction that would fault in user mode -- to read IPL, adds its
;      argument (5) to ^X1230, and returns that as its status. $CMKRNL
;      returns it in R0: ^X1235.
;   3. $CMEXEC(EXEC): EXEC records the mode it runs in and returns 1.
;   4. Back in user mode, record the mode.
;
; The Go test checks the modes recorded, and R0 is 1 at the end if both
; services returned their routine's status.

	.microkernel
	.p1vector

	.entry	main, ^m<>

	mtpr	#^X1F, #^X12		; IPL 31 while in kernel mode
; ---- 1. drop to user mode (IPL 0) ----
	pushl	#^X03C00000		; PSL: current and previous mode user, IPL 0
	pushal	@#usercode		; PC
	rei

usercode:
; ---- 2. $CMKRNL ----
	pushal	@#args			; arglst
	pushal	@#krnl			; routin
	calls	#2, @#sys$cmkrnl
	cmpl	r0, #^X1235		; the routine's status
	beql	ok1
	brw	fail
ok1:

; ---- 3. $CMEXEC ----
	pushl	#0
	pushal	@#exec
	calls	#2, @#sys$cmexec
	cmpl	r0, #1
	beql	ok2
	brw	fail
ok2:

; ---- 4. back in user mode ----
	movpsl	r0
	extzv	#^D24, #2, r0, @#umode
	movl	#1, r0
	ret

fail:	clrl	r0
	ret

; KRNL(n): runs in kernel mode; returns ^X1230 + n.
	.entry	krnl, ^m<r2>
	movpsl	r2
	extzv	#^D24, #2, r2, @#kmode	; PSL<CUR_MOD>: kernel, 0
	extzv	#^D22, #2, r2, @#kprv	; PSL<PRV_MOD>: user, 3
	mfpr	#^X12, @#kipl		; privileged: IPL
	addl3	#^X1230, b^4(ap), r0
	ret

; EXEC(): runs in executive mode.
	.entry	exec, ^m<r2>
	movpsl	r2
	extzv	#^D24, #2, r2, @#emode	; executive, 1
	movl	#1, r0
	ret

args:	.long	1, 5			; one argument: 5

kmode:	.long	^XFF
kprv:	.long	^XFF
kipl:	.long	^XFF
emode:	.long	^XFF
umode:	.long	^XFF

	.end	main
