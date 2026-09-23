;
; testdata/asm/rms_roundtrip.asm -- docs/PHASE-22.md subtask 14's end-to-end
; RMS acceptance fixture: a real, assembled/executed VAX program that drives
; SYS$CREATE -> SYS$CONNECT -> SYS$PUT (x3) -> SYS$CLOSE -> SYS$OPEN ->
; SYS$CONNECT -> SYS$GET (x3) -> SYS$CLOSE against a mounted ODS-2 disk
; device (DUA0:), verifying each record read back matches what was written.
;
; FAB/RAB below are built via .FAB/.RAB (docs/PHASE-24.md), this project's
; own equivalent of the real $FAB/$RAB macros -- .RMSDEF (this file's own
; equivalent of $FABDEF/$RABDEF/$RMSDEF) defines every real FAB$/RAB$/RMS$
; symbol first, so both the .FAB/.RAB keyword values below (FAB$M_PUT,
; FAB$C_SEQ, ...) and the runtime field pokes further down
; (@#fab+FAB$B_FAC, real-MACRO-32-style <label>+offset-symbol addressing --
; .FAB/.RAB don't generate their own per-field sub-labels, see
; docs/PHASE-24.md's own design notes on why) resolve to real, checked
; values instead of the bare, only-comment-documented numeric literals this
; fixture used before this phase. SYS$xxx symbols below are the real
; P1-vector addresses .P1VECTOR defines (internal/vmsdef.P1VectorTable),
; called through "@#", matching every other SYS$/LIB$ call in this
; project's own test fixtures (e.g. hello.asm's "calls #1,@#lib$put_output").
;
; .MICROKERNEL/.P1VECTOR below build the real VMS P1 system-service vector
; (internal/asm/pseudo.go's pseudoP1Vector) -- every SYS$xxx symbol this
; program calls, and the CALLS-reachable trampoline each one's address
; actually holds, come from there rather than from hand-set symbols/a
; test-side byte deposit.

	.microkernel
	.p1vector
	.rmsdef

; FAB/RAB built here, before the code below, rather than down with the rest
; of this file's data (after "fail:") the way earlier fixtures in this
; project keep data: the code below addresses fields at runtime the real-
; MACRO-32 way ("@#rab+RAB$L_RBF"), and this assembler's forward-reference
; fixup mechanism -- like the reference tool's own -- can't defer a symbol
; combined with an operator (VAX_FWDOPERATOR), only a bare symbol. Placing
; "fab"/"rab" ahead of every place that adds an offset to them keeps both
; already resolved by the time the code needs to. FNA=fspec below is a bare
; forward reference (fspec is still defined down in the data section) --
; that's fine; only "label+offset-symbol" needs the label already resolved.
;
; Initial field values that never change across the whole program (FAC's
; *initial* PUT-access value -- it's changed at runtime for the reopen,
; below -- ORG, RFM, MRS, FNA/FNS, and RAB$L_FAB/RAB$B_RAC, which the pre-
; Phase-24 version of this fixture re-set identically via runtime MOVB/
; MOVAL/MOVL right before each CONNECT) are supplied directly as keyword
; values here; only fields that genuinely vary at runtime (RAB$L_RBF/
; RAB$W_RSZ, a different record each PUT/GET) are still poked via real-
; MACRO-32-style "@#rab+RAB$L_RBF" addressing in the code below. FNS uses
; "^D13" (decimal), not a bare "13" -- this assembler's default radix is
; hex, so a plain "13" here would mean 0x13 (19), not decimal 13
; (strlen("DUA0:TEST.DAT")).
fab:	.fab	fac=FAB$M_PUT, org=FAB$C_SEQ, rfm=FAB$C_FIX, mrs=4, fna=fspec, fns=^D13
rab:	.rab	fab=fab, rac=RAB$C_SEQ

	.entry	main, ^m<>

; Every RMS call below is checked via "BLBS r0,okN / BRW fail" rather than
; a plain "BLBC r0,fail": real VAX conditional branches (BLBC/BLBS/BNEQ/...)
; only ever encode an 8-bit displacement, and this program's full CREATE->
; ...->CLOSE sequence is far longer than 127 bytes end to end, so a direct
; conditional branch all the way down to the shared "fail:" label doesn't
; fit -- the standard MACRO-32 idiom for a conditional branch outside byte
; range is to invert the condition and fall through to an unconditional
; BRW (word displacement, +/-32K) over the real target.
	pushal	@#fab
	calls	#1, @#sys$create
	blbs	r0, ok1
	brw	fail
ok1:

; ---- CONNECT a RAB to the just-created file and PUT three records ----
	pushal	@#rab
	calls	#1, @#sys$connect
	blbs	r0, ok2
	brw	fail
ok2:

	moval	@#rec1, r1
	movl	r1, @#rab+RAB$L_RBF
	movw	#4, @#rab+RAB$W_RSZ
	pushal	@#rab
	calls	#1, @#sys$put
	blbs	r0, ok3
	brw	fail
ok3:

	moval	@#rec2, r1
	movl	r1, @#rab+RAB$L_RBF
	movw	#4, @#rab+RAB$W_RSZ
	pushal	@#rab
	calls	#1, @#sys$put
	blbs	r0, ok4
	brw	fail
ok4:

	moval	@#rec3, r1
	movl	r1, @#rab+RAB$L_RBF
	movw	#4, @#rab+RAB$W_RSZ
	pushal	@#rab
	calls	#1, @#sys$put
	blbs	r0, ok5
	brw	fail
ok5:

	pushal	@#fab
	calls	#1, @#sys$close
	blbs	r0, ok6
	brw	fail
ok6:

; ---- reopen the same file for reading, reusing the same FAB/RAB ----
	movb	#FAB$M_GET, @#fab+FAB$B_FAC
	pushal	@#fab
	calls	#1, @#sys$open
	blbs	r0, ok7
	brw	fail
ok7:

	pushal	@#rab
	calls	#1, @#sys$connect
	blbs	r0, ok8
	brw	fail
ok8:

	moval	@#readbuf, r1
	movl	r1, @#rab+RAB$L_RBF
	pushal	@#rab
	calls	#1, @#sys$get
	blbs	r0, ok9
	brw	fail
ok9:
	movl	@#readbuf, r2
	cmpl	r2, @#rec1
	beql	ok10
	brw	fail
ok10:

	moval	@#readbuf, r1
	movl	r1, @#rab+RAB$L_RBF
	pushal	@#rab
	calls	#1, @#sys$get
	blbs	r0, ok11
	brw	fail
ok11:
	movl	@#readbuf, r2
	cmpl	r2, @#rec2
	beql	ok12
	brw	fail
ok12:

	moval	@#readbuf, r1
	movl	r1, @#rab+RAB$L_RBF
	pushal	@#rab
	calls	#1, @#sys$get
	blbs	r0, ok13
	brw	fail
ok13:
	movl	@#readbuf, r2
	cmpl	r2, @#rec3
	beql	ok14
	brw	fail
ok14:

	pushal	@#fab
	calls	#1, @#sys$close
	blbs	r0, ok15
	brw	fail
ok15:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; ---- data ----
fspec:	.ascii	"DUA0:TEST.DAT"

rec1:	.long	^X11111111
rec2:	.long	^X22222222
rec3:	.long	^X33333333
readbuf: .long	0

	.end	main
