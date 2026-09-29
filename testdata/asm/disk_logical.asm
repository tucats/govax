;
; testdata/asm/disk_logical.asm -- docs/PHASE-26.md subtask 45's
; acceptance fixture: logical and physical block I/O on a mounted volume,
; with the privileges it needs.
;
;   1. $ASSIGN a channel to DUA0: (CHAN).
;   2. IO$_READLBLK of logical block 1, the home block, into HOME.
;   3. IO$_WRITELBLK of BOOT (BOOTLEN bytes) to logical block 0, the boot
;      block, which the file system doesn't use.
;   4. IO$_READPBLK of physical block 0 (the same block, on these disks)
;      into BACK.
;   5. $SETPRV disabling LOG_IO and PHY_IO for the image; IO$_READLBLK
;      again: $QIOW's R0, kept in NOPRIV, is SS$_NOPRIV. $SETPRV enabling
;      them again.
;   6. IO$_READLBLK of a block far past the end of the volume: the IOSB's
;      status, kept in ILLBLK, is SS$_ILLBLKNUM.
;   7. $DASSGN.
;
; R0 is 1 at the end if every call and IOSB meant to succeed did.

	.microkernel
	.p1vector

	.entry	main, ^m<>

; ---- 1. a channel to the disk ----
	pushl	#0			; mbxnam
	pushl	#0			; acmode
	pushal	@#chan
	pushal	@#devnam
	calls	#4, @#sys$assign
	jsb	@#check

; ---- 2. read the home block ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#1			; p3: LBN 1
	pushl	#^D512			; p2: bytes
	pushal	@#home			; p1
	pushl	#^X21			; IO$_READLBLK
	bsbw	qio
	jsb	@#check

; ---- 3. write the boot block ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0			; p3: LBN 0
	movzwl	@#bootlen, -(sp)	; p2
	pushal	@#boot			; p1
	pushl	#^X20			; IO$_WRITELBLK
	bsbw	qio
	jsb	@#check

; ---- 4. read it back, physically ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0			; p3: block 0
	pushl	#^D512
	pushal	@#back
	pushl	#^X0C			; IO$_READPBLK
	bsbw	qio
	jsb	@#check

; ---- 5. without LOG_IO and PHY_IO: SS$_NOPRIV ----
	pushl	#0			; prvprv
	pushl	#0			; prmflg: for this image
	pushal	@#blkprv		; prvadr
	pushl	#0			; enbflg: disable
	calls	#4, @#sys$setprv
	jsb	@#check
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#1
	pushl	#^D512
	pushal	@#home
	pushl	#^X21			; IO$_READLBLK
	bsbw	qio
	movl	r0, @#noprv		; R0 of $QIOW
	pushl	#0
	pushl	#0
	pushal	@#blkprv
	pushl	#1			; enable again
	calls	#4, @#sys$setprv
	jsb	@#check

; ---- 6. past the end of the volume: SS$_ILLBLKNUM ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#^X7FFFFFFF		; p3
	pushl	#^D512
	pushal	@#back
	pushl	#^X21			; IO$_READLBLK
	bsbw	qio
	movl	r0, @#illblk		; the IOSB's status

; ---- 7. deassign ----
	movzwl	@#chan, -(sp)
	calls	#1, @#sys$dassgn
	jsb	@#check

	movl	#1, r0
	ret

; QIO (by BSBW, with the function and p1-p6 pushed, function last):
; $QIOW on CHAN with IOSB; returns with R0 the IOSB's status if $QIOW
; itself succeeded. The arguments are popped.
qio:	movl	(sp)+, retpc		; the return address
	movl	(sp)+, r1		; the function
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushal	@#iosb
	pushl	r1
	movzwl	@#chan, -(sp)
	pushl	#0			; efn
	calls	#^D12, @#sys$qiow	; takes efn..p6 off the stack
	blbc	r0, 10$
	movzwl	iosb, r0
10$:	jmp	@retpc

; CHECK (by JSB): return from MAIN with R0 0 unless R0 is a success.
check:	blbs	r0, 10$
	clrl	r0
	ret
10$:	rsb

devnam:	.ascid	"DUA0:"
; The privilege mask ($PRVDEF): LOG_IO is bit 7, PHY_IO bit 22.
blkprv:	.long	^X00400080, 0
boot:	.ascii	"govax boot block"
boote:
bootlen: .word	boote-boot
noprv:	.long	0
illblk:	.long	0
chan:	.word	0
iosb:	.long	0, 0
retpc:	.long	0
home:	.blkb	^D512
back:	.blkb	^D512

	.end	main
