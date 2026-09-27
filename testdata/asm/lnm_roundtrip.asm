;
; testdata/asm/lnm_roundtrip.asm -- docs/PHASE-25.md subtask 8's logical-
; name acceptance fixture: a real, assembled/executed VAX program that
;
;   1. creates MYOUT = "SYS$OUTPUT" in the process table ($CRELNM),
;   2. translates it back through LNM$FILE_DEV and checks the string and
;      its length ($TRNLNM with an LNM$_STRING item),
;   3. writes a record to the file spec "MYOUT", which RMS translates
;      iteratively (MYOUT -> SYS$OUTPUT -> _TTA0:, the console),
;   4. deletes MYOUT ($DELLNM), and
;   5. checks that translating it again fails with SS$_NOLOGNAM.
;
; R0 is 1 at the end if every step did what it should, 0 otherwise. As in
; rms_roundtrip.asm, each check is "Bcc okN / BRW fail", since a plain
; conditional branch can't reach "fail" from this far away.
;
; Item codes and status values are written as numbers (this assembler
; has no .LNMDEF): LNM$_STRING is 2, SS$_NOLOGNAM is ^X1BC. The default
; radix is hex, hence the ^D on decimal lengths.

	.microkernel
	.p1vector
	.rmsdef

fab:	.fab	fac=FAB$M_PUT, org=FAB$C_SEQ, rfm=FAB$C_VAR, mrs=^D80, fna=fspec, fns=5
rab:	.rab	fab=fab, rac=RAB$C_SEQ

	.entry	main, ^m<>

; ---- 1. $CRELNM(attr=0, LNM$PROCESS, MYOUT, acmode=caller's, crelst) ----
	pushal	@#crelst
	pushl	#0
	pushal	@#lognam
	pushal	@#tabnam
	pushl	#0
	calls	#5, @#sys$crelnm
	blbs	r0, ok1
	brw	fail
ok1:

; ---- 2. $TRNLNM(attr=0, LNM$FILE_DEV, MYOUT, acmode=none, trnlst) ----
	pushal	@#trnlst
	pushl	#0
	pushal	@#lognam
	pushal	@#filedev
	pushl	#0
	calls	#5, @#sys$trnlnm
	blbs	r0, ok2
	brw	fail
ok2:
	cmpw	@#retlen, #^D10
	beql	ok3
	brw	fail
ok3:
	cmpc3	#^D10, @#eqvbuf, @#eqvstr
	beql	ok4
	brw	fail
ok4:

; ---- 3. write one record to "MYOUT" ----
	pushal	@#fab
	calls	#1, @#sys$create
	blbs	r0, ok5
	brw	fail
ok5:
	pushal	@#rab
	calls	#1, @#sys$connect
	blbs	r0, ok6
	brw	fail
ok6:
	moval	@#msg, r1
	movl	r1, @#rab+RAB$L_RBF
	movw	#^D17, @#rab+RAB$W_RSZ
	pushal	@#rab
	calls	#1, @#sys$put
	blbs	r0, ok7
	brw	fail
ok7:
	pushal	@#fab
	calls	#1, @#sys$close
	blbs	r0, ok8
	brw	fail
ok8:

; ---- 4. $DELLNM(LNM$PROCESS, MYOUT, acmode=caller's) ----
	pushl	#0
	pushal	@#lognam
	pushal	@#tabnam
	calls	#3, @#sys$dellnm
	blbs	r0, ok9
	brw	fail
ok9:

; ---- 5. translating MYOUT now fails with SS$_NOLOGNAM ----
	pushal	@#trnlst
	pushl	#0
	pushal	@#lognam
	pushal	@#filedev
	pushl	#0
	calls	#5, @#sys$trnlnm
	cmpl	r0, #^X1BC
	beql	ok10
	brw	fail
ok10:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; ---- data ----
tabnam:	.ascid	"LNM$PROCESS"
filedev: .ascid	"LNM$FILE_DEV"
lognam:	.ascid	"MYOUT"
fspec:	.ascii	"MYOUT"
eqvstr:	.ascii	"SYS$OUTPUT"
msg:	.ascii	"LNM round trip OK"

; $CRELNM item list: one LNM$_STRING, "SYS$OUTPUT".
crelst:	.word	^D10
	.word	2
	.long	eqvstr
	.long	0
	.long	0

; $TRNLNM item list: LNM$_STRING into eqvbuf, its length into retlen.
trnlst:	.word	^D32
	.word	2
	.long	eqvbuf
	.long	retlen
	.long	0

eqvbuf:	.blkb	^D32
retlen:	.word	0

	.end	main
