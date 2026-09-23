;
; testdata/asm/rms_roundtrip.asm -- docs/PHASE-22.md subtask 14's end-to-end
; RMS acceptance fixture: a real, assembled/executed VAX program that drives
; SYS$CREATE -> SYS$CONNECT -> SYS$PUT (x3) -> SYS$CLOSE -> SYS$OPEN ->
; SYS$CONNECT -> SYS$GET (x3) -> SYS$CLOSE against a mounted ODS-2 disk
; device (DUA0:), verifying each record read back matches what was written.
;
; FAB/RAB fields below are hand-laid-out via .BLKB/.BYTE/.WORD/.LONG
; directives at their real $FABDEF/$RABDEF byte offsets (see
; internal/rms/fab.go, rab.go) rather than expanded from a macro -- this
; project has no $FABDEF/$RABDEF .INCLUDE facility yet (explicitly out of
; scope for docs/PHASE-22.md's subtask 14). SYS$xxx symbols below are
; literal P1-vector addresses (internal/rtl/p1vector.go) called through
; "@#", matching every other SYS$/LIB$ call in this project's own test
; fixtures (e.g. hello.asm's "calls #1,@#lib$put_output"): a bare .SET
; symbol pointing straight at the target address, no linker/image-activation
; step involved.
;
; The Go test driving this fixture (internal/console/rms_e2e_test.go) is
; responsible for depositing an "XFC #XFC$P1VECTOR" trampoline at each of
; the six SYS$xxx addresses below before running this program -- nothing in
; internal/asm's own .P1VECTOR pseudo-op does that yet (see that pseudo-op's
; own doc comment: building it for real would need internal/asm to import
; internal/rtl's service table, deferred).
;

	.set	/perm	sys$create	^X7FFEE1C8
	.set	/perm	sys$connect	^X7FFEE1C0
	.set	/perm	sys$put		^X7FFEE188
	.set	/perm	sys$close	^X7FFEE1B8
	.set	/perm	sys$open	^X7FFEE208
	.set	/perm	sys$get		^X7FFEE180

	.entry	main, ^m<>

; ---- build the FAB for a fixed-format, 4-byte-record sequential file ----
	movb	#1, @#fab_fac		; FAB$V_PUT
	movb	#0, @#fab_org		; FAB$C_SEQ
	movb	#1, @#fab_rfm		; FAB$C_FIX
	movw	#4, @#fab_mrs		; 4-byte fixed records

	moval	@#fspec, r1
	movl	r1, @#fab_fna
	movb	#^D13, @#fab_fns	; strlen("DUA0:TEST.DAT") -- ^D: this
					; assembler's default radix is hex,
					; so a plain "13" here would mean
					; 0x13 (19), not decimal 13

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
	moval	@#fab, r1
	movl	r1, @#rab_fab
	pushal	@#rab
	calls	#1, @#sys$connect
	blbs	r0, ok2
	brw	fail
ok2:

	movb	#0, @#rab_rac		; RAB$C_SEQ

	moval	@#rec1, r1
	movl	r1, @#rab_rbf
	movw	#4, @#rab_rsz
	pushal	@#rab
	calls	#1, @#sys$put
	blbs	r0, ok3
	brw	fail
ok3:

	moval	@#rec2, r1
	movl	r1, @#rab_rbf
	movw	#4, @#rab_rsz
	pushal	@#rab
	calls	#1, @#sys$put
	blbs	r0, ok4
	brw	fail
ok4:

	moval	@#rec3, r1
	movl	r1, @#rab_rbf
	movw	#4, @#rab_rsz
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
	movb	#2, @#fab_fac		; FAB$V_GET
	pushal	@#fab
	calls	#1, @#sys$open
	blbs	r0, ok7
	brw	fail
ok7:

	moval	@#fab, r1
	movl	r1, @#rab_fab
	pushal	@#rab
	calls	#1, @#sys$connect
	blbs	r0, ok8
	brw	fail
ok8:

	movb	#0, @#rab_rac		; RAB$C_SEQ

	moval	@#readbuf, r1
	movl	r1, @#rab_rbf
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
	movl	r1, @#rab_rbf
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
	movl	r1, @#rab_rbf
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

; ---- FAB: 80 bytes total, offsets per internal/rms/fab.go ----
; The ".blkb" counts below are byte counts in DECIMAL (matching the
; comments' own decimal offsets) -- this assembler's default radix is hex,
; so any two-digit count that could be misread as a hex value is spelled
; with an explicit "^D" (decimal) prefix. A single digit (0-9) reads the
; same in either radix and needs no prefix.
fab:		.blkb	2	; 0-1  reserved
fab_ifi:	.word	0	; 2-3  FAB$W_IFI
		.blkb	4	; 4-7  reserved
fab_sts:	.long	0	; 8-11 FAB$L_STS
fab_stv:	.long	0	; 12-15 FAB$L_STV
		.blkb	6	; 16-21 reserved
fab_fac:	.byte	0	; 22   FAB$B_FAC
		.blkb	6	; 23-28 reserved
fab_org:	.byte	0	; 29   FAB$B_ORG
fab_rat:	.byte	0	; 30   FAB$B_RAT
fab_rfm:	.byte	0	; 31   FAB$B_RFM
		.blkb	^D12	; 32-43 reserved
fab_fna:	.long	0	; 44-47 FAB$L_FNA
		.blkb	4	; 48-51 reserved (FAB$L_DNA)
fab_fns:	.byte	0	; 52   FAB$B_FNS
		.blkb	1	; 53   reserved
fab_mrs:	.word	0	; 54-55 FAB$W_MRS
		.blkb	^D24	; 56-79 reserved

; ---- RAB: 68 bytes total, offsets per internal/rms/rab.go (same decimal-
; ".blkb"-count convention as the FAB above) ----
rab:		.blkb	2	; 0-1  reserved
rab_isi:	.word	0	; 2-3  RAB$W_ISI
		.blkb	4	; 4-7  reserved
rab_sts:	.long	0	; 8-11 RAB$L_STS
rab_stv:	.long	0	; 12-15 RAB$L_STV
		.blkb	^D14	; 16-29 reserved
rab_rac:	.byte	0	; 30   RAB$B_RAC
		.blkb	1	; 31   reserved
rab_usz:	.word	0	; 32-33 RAB$W_USZ
rab_rsz:	.word	0	; 34-35 RAB$W_RSZ
rab_ubf:	.long	0	; 36-39 RAB$L_UBF
rab_rbf:	.long	0	; 40-43 RAB$L_RBF
		.blkb	^D16	; 44-59 reserved
rab_fab:	.long	0	; 60-63 RAB$L_FAB
		.blkb	4	; 64-67 reserved

	.end	main
