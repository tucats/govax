;
; testdata/asm/rms_rename.asm -- SYS$RENAME's end-to-end fixture: a real,
; assembled and executed VAX program that creates DUA0:OLD.DAT, renames it
; to DUA0:NEW.DAT with SYS$RENAME, and checks the rename took: NEW.DAT
; opens, OLD.DAT no longer does (RMS$_FNF), and renaming a second file
; onto NEW.DAT;1 is refused with RMS$_ENT (a rename never replaces a
; file), leaving that second file where it was.
;
; Like rms_roundtrip.asm, the FABs are built with .FAB, the SYS$ symbols
; come from .P1VECTOR, and every check branches with "BLBS/BLSS okN; BRW
; fail" because the program is longer than a byte displacement reaches.
; SYS$RENAME takes four arguments: the old FAB, an error and a success
; completion routine (none here), and the new FAB -- pushed in reverse.

	.microkernel
	.p1vector
	.rmsdef

oldfab:	.fab	fac=FAB$M_PUT, org=FAB$C_SEQ, rfm=FAB$C_FIX, mrs=4, fna=oldspec, fns=^D12
newfab:	.fab	fac=FAB$M_GET, org=FAB$C_SEQ, rfm=FAB$C_FIX, mrs=4, fna=newspec, fns=^D14
twofab:	.fab	fac=FAB$M_PUT, org=FAB$C_SEQ, rfm=FAB$C_FIX, mrs=4, fna=twospec, fns=^D12

	.entry	main, ^m<>

; ---- create OLD.DAT (empty) ----
	pushal	@#oldfab
	calls	#1, @#sys$create
	blbs	r0, ok1
	brw	fail
ok1:	pushal	@#oldfab
	calls	#1, @#sys$close
	blbs	r0, ok2
	brw	fail
ok2:

; ---- rename OLD.DAT to NEW.DAT;1 ----
	pushal	@#newfab
	clrl	-(sp)
	clrl	-(sp)
	pushal	@#oldfab
	calls	#4, @#sys$rename
	blbs	r0, ok3
	brw	fail
ok3:	cmpl	@#oldfab+FAB$L_STS, #RMS$_NORMAL
	beql	ok4
	brw	fail
ok4:

; ---- NEW.DAT opens ----
	pushal	@#newfab
	calls	#1, @#sys$open
	blbs	r0, ok5
	brw	fail
ok5:	pushal	@#newfab
	calls	#1, @#sys$close
	blbs	r0, ok6
	brw	fail
ok6:

; ---- OLD.DAT doesn't ----
	movb	#FAB$M_GET, @#oldfab+FAB$B_FAC
	pushal	@#oldfab
	calls	#1, @#sys$open
	cmpl	r0, #RMS$_FNF
	beql	ok7
	brw	fail
ok7:

; ---- TWO.DAT onto NEW.DAT;1 is refused; TWO.DAT stays ----
	pushal	@#twofab
	calls	#1, @#sys$create
	blbs	r0, ok8
	brw	fail
ok8:	pushal	@#twofab
	calls	#1, @#sys$close
	blbs	r0, ok9
	brw	fail
ok9:	pushal	@#newfab
	clrl	-(sp)
	clrl	-(sp)
	pushal	@#twofab
	calls	#4, @#sys$rename
	cmpl	r0, #RMS$_ENT
	beql	ok10
	brw	fail
ok10:	movb	#FAB$M_GET, @#twofab+FAB$B_FAC
	pushal	@#twofab
	calls	#1, @#sys$open
	blbs	r0, ok11
	brw	fail
ok11:	pushal	@#twofab
	calls	#1, @#sys$close

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; ---- data ----
oldspec: .ascii	"DUA0:OLD.DAT"
newspec: .ascii	"DUA0:NEW.DAT;1"
twospec: .ascii	"DUA0:TWO.DAT"

	.end	main
