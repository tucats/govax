;
; testdata/asm/privileges.asm -- docs/PHASE-26.md subtask 38's
; acceptance fixture: privileges.
;
;   1. $GETJPIW reads JPI$_CURPRIV, the enabled privileges: all 39 of
;      them (CURPRIV: ^XFFFFFFFF, ^X0000007F).
;   2. In user mode, $SETPRV disables CMKRNL (bit 0) for the image,
;      returning the previous mask (PRVPRV). $CMKRNL then fails with
;      SS$_NOPRIV (CMK1).
;   3. $SETPRV enables it again (SETPRV2: SS$_NORMAL, as the process is
;      authorized for it), and $CMKRNL runs KRNL in kernel mode, which
;      returns ^X77 (CMK2).
;   4. With TMPMBX disabled, $CREMBX fails with SS$_NOPRIV (MBX).
;
; The Go test checks every value recorded below, and that image rundown
; left the privileges as they were.

	.microkernel
	.p1vector

	.entry	main, ^m<>

; ---- 1. the current privileges ----
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushl	#0			; iosb
	pushal	@#items			; itmlst
	pushl	#0			; prcnam
	pushl	#0			; pidadr
	pushl	#0			; efn
	calls	#7, @#sys$getjpiw

; ---- 2. user mode, CMKRNL disabled ----
	pushl	#^X03C00000		; PSL: current and previous mode user
	pushal	@#usercode
	rei

usercode:
	pushal	@#prvprv		; prvprv
	pushl	#0			; prmflg: for this image
	pushal	@#cmkrnl		; prvadr
	pushl	#0			; enbflg: disable
	calls	#4, @#sys$setprv
	pushl	#0
	pushal	@#krnl
	calls	#2, @#sys$cmkrnl
	movl	r0, @#cmk1

; ---- 3. enabled again ----
	pushl	#0
	pushl	#0
	pushal	@#cmkrnl
	pushl	#1			; enable
	calls	#4, @#sys$setprv
	movl	r0, @#setprv2
	pushl	#0
	pushal	@#krnl
	calls	#2, @#sys$cmkrnl
	movl	r0, @#cmk2

; ---- 4. $CREMBX without TMPMBX ----
	pushl	#0
	pushl	#0
	pushal	@#tmpmbx
	pushl	#0
	calls	#4, @#sys$setprv
	pushl	#0			; lognam
	pushl	#0			; acmode
	pushl	#0			; promsk
	pushl	#0			; bufquo
	pushl	#0			; maxmsg
	pushal	@#chan
	pushl	#0			; temporary
	calls	#7, @#sys$crembx
	movl	r0, @#mbx

	movl	#1, r0
	ret

; KRNL runs in kernel mode through $CMKRNL.
	.entry	krnl, ^m<>
	movl	#^X77, r0
	ret

items:	.word	8			; buffer length
	.word	^X400			; JPI$_CURPRIV
	.long	curpriv			; buffer
	.long	0			; return length: not wanted
	.long	0			; end of list

curpriv: .long	0, 0
prvprv:	.long	0, 0
cmkrnl:	.long	1, 0			; PRV$M_CMKRNL
tmpmbx:	.long	^X8000, 0		; PRV$M_TMPMBX
cmk1:	.long	0
setprv2: .long	0
cmk2:	.long	0
mbx:	.long	0
chan:	.word	0

	.end	main
