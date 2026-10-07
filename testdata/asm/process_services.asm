;
; testdata/asm/process_services.asm -- docs/PHASE-26.md's acceptance
; fixture for its first batch of system services: a real, assembled and
; executed VAX program that, running in kernel mode,
;
;   1. sets user mode's stack pointer to ^X5000 - 8 ($ADJSTK), and checks
;      the value written back to newadr,
;   2. grows the working-set limit by 10 pages ($ADJWSL), and checks the
;      new limit returned (the default 512, plus 10),
;   3. allocates TTA0 ($ALLOC), and checks the physical name returned,
;   4. associates common event flag cluster "CLUSTER" with cluster 2
;      ($ASCEFC), sets flag 65 in it ($SETEF, which was clear), and reads
;      it back ($READEF, which finds it set and returns the cluster's
;      flags).
;
; R0 is 1 at the end if every step did what it should, 0 otherwise. Each
; check is "Bcc okN / BRW fail", since a plain conditional branch can't
; reach "fail" from this far away. SS$_WASCLR is 1, SS$_WASSET is 9.

	.microkernel
	.p1vector

	.entry	main, ^m<>

; ---- 1. $ADJSTK(acmode=user, adjust=-8, newadr) ----
	pushal	@#spval
	pushl	#^XFFF8
	pushl	#3
	calls	#3, @#sys$adjstk
	blbs	r0, ok1
	brw	fail
ok1:
	cmpl	@#spval, #^X4FF8
	beql	ok2
	brw	fail
ok2:

; ---- 2. $ADJWSL(pagcnt=10, wsetlm) ----
	pushal	@#wslimit
	pushl	#^D10
	calls	#2, @#sys$adjwsl
	blbs	r0, ok3
	brw	fail
ok3:
	cmpl	@#wslimit, #^D522
	beql	ok4
	brw	fail
ok4:

; ---- 3. $ALLOC(TTA0, phylen, phybuf, acmode=caller's, flags=0) ----
	pushl	#0
	pushl	#0
	pushal	@#phybuf
	pushal	@#phylen
	pushal	@#devnam
	calls	#5, @#sys$alloc
	cmpl	r0, #1
	beql	ok5
	brw	fail
ok5:
	cmpw	@#phylen, #6
	beql	ok6
	brw	fail
ok6:
	cmpc3	#6, @#phystr, @#physexp
	beql	ok7
	brw	fail
ok7:

; ---- 4. $ASCEFC(efn=64, CLUSTER), then $SETEF/$READEF flag 65 ----
	pushl	#0
	pushl	#0
	pushal	@#cefnam
	pushl	#^D64
	calls	#4, @#sys$ascefc
	blbs	r0, ok8
	brw	fail
ok8:
	pushl	#^D65
	calls	#1, @#sys$setef
	cmpl	r0, #1
	beql	ok9
	brw	fail
ok9:
	pushal	@#efstate
	pushl	#^D65
	calls	#2, @#sys$readef
	cmpl	r0, #9
	beql	ok10
	brw	fail
ok10:
	cmpl	@#efstate, #2
	beql	ok11
	brw	fail
ok11:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; ---- data ----
spval:	.long	^X5000
wslimit: .long	0
devnam:	.ascid	"TTA0"
physexp: .ascii	"_TTA0:"
phylen:	.word	0
phybuf:	.word	^D16
	.word	0
	.long	phystr
phystr:	.blkb	^D16
cefnam:	.ascid	"CLUSTER"
efstate: .long	0

	.end	main
