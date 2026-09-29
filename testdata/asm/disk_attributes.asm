;
; testdata/asm/disk_attributes.asm -- docs/PHASE-26.md subtask 42's
; acceptance fixture: reading and writing a file's attributes through the
; disk's $QIO attribute lists, the way RMS sets a file's end of file when
; it closes it.
;
;   1. $ASSIGN a channel to DUA0: (CHAN).
;   2. IO$_ACCESS!IO$M_ACCESS of DATA.TXT in [000000], for writing, with
;      an attribute list reading its record attribute area (ATR$C_RECATTR,
;      32 bytes, $FATDEF) into FAT1.
;   3. IO$_WRITEVBLK of "Short now." (10 bytes) over virtual block 1.
;   4. The end of file in FAT1 set to block 1, byte 10: FAT$L_EFBLK (at
;      8, a "swapped" longword: high word first) and FAT$W_FFBYTE (at 12).
;   5. IO$_DEACCESS with an attribute list writing FAT1 back.
;   6. IO$_ACCESS alone (the FIB still holding the file ID step 2 found):
;      no access, just the attributes: the record attribute area into
;      FAT2 and the name (ATR$C_ASCNAME) into NAME2.
;   7. $DASSGN.
;
; R0 is 1 at the end if every call and every IOSB succeeded.

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

; ---- 2. access DATA.TXT for writing, reading its record attributes ----
	pushl	#0			; p6
	pushal	@#rdfat1		; p5: the attribute list
	pushl	#0			; p4
	pushl	#0			; p3
	pushal	@#named			; p2: the name
	pushal	@#fibd			; p1: the FIB
	pushl	#^X72			; IO$_ACCESS!IO$M_ACCESS
	bsbw	qio
	jsb	@#check

; ---- 3. overwrite block 1 with shorter text ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#1			; p3: VBN
	pushl	#^D10			; p2: bytes
	pushal	@#short			; p1
	pushl	#^X30			; IO$_WRITEVBLK
	bsbw	qio
	jsb	@#check

; ---- 4-5. the end of file is block 1, byte 10: write it on deaccess ----
	movl	#^X10000, @#fat1+8	; FAT$L_EFBLK = 1, high word first
	movw	#^D10, @#fat1+^D12	; FAT$W_FFBYTE = 10
	pushl	#0
	pushal	@#wrfat1		; p5: the attribute list
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#^X34			; IO$_DEACCESS
	bsbw	qio
	jsb	@#check

; ---- 6. read the attributes again, without accessing the file ----
	pushl	#0
	pushal	@#rdfat2		; p5
	pushl	#0
	pushl	#0
	pushl	#0			; p2: no name: by the FIB's file ID
	pushal	@#fibd			; p1
	pushl	#^X32			; IO$_ACCESS
	bsbw	qio
	jsb	@#check

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
name:	.ascii	"DATA.TXT"
namee:
named:	.long	namee-name, name
; The FIB: access control FIB$M_WRITE, file ID 0 (to be looked up),
; directory ID (4,4,0): the master file directory.
fib:	.long	^X100			; FIB$L_ACCTL
	.word	0, 0, 0			; FIB$W_FID
	.word	4, 4, 0			; FIB$W_DID
	.blkb	^D48			; the rest
fibe:
fibd:	.long	fibe-fib, fib
; The attribute lists: entries of a size word, a code word, and a buffer
; address, ended by a zero longword. ATR$C_RECATTR is 4, ATR$C_ASCNAME 16.
rdfat1:	.word	^D32, 4
	.long	fat1
	.long	0
wrfat1:	.word	^D32, 4
	.long	fat1
	.long	0
rdfat2:	.word	^D32, 4
	.long	fat2
	.word	^D20, ^D16
	.long	name2
	.long	0
short:	.ascii	"Short now."
chan:	.word	0
iosb:	.long	0, 0
retpc:	.long	0
fat1:	.blkb	^D32
fat2:	.blkb	^D32
name2:	.blkb	^D20

	.end	main
