;
; testdata/asm/disk_create.asm -- docs/PHASE-26.md subtask 43's
; acceptance fixture: creating a file with the disk's $QIO, and giving it
; a second name.
;
;   1. $ASSIGN a channel to DUA0: (CHAN).
;   2. IO$_CREATE!IO$M_CREATE!IO$M_ACCESS of NOTES.TXT in [000000]: a new
;      file, one block allocated at once (FIB$M_EXTEND, FIB$L_EXSZ 1),
;      accessed for writing, with an attribute list making it a
;      STREAM_LF text file (ATR$C_RECATTR). The new file ID comes back in
;      the FIB, the name made in RESULT1 (RESLEN1).
;   3. IO$_WRITEVBLK of TEXT (TEXTLEN bytes) to virtual block 1.
;   4. IO$_DEACCESS with an attribute list writing the record attributes
;      again, their end of file now block 1, byte TEXTLEN.
;   5. IO$_CREATE alone (no IO$M_CREATE): enter the file (by the file ID
;      still in the FIB) in [000000] as ALIAS.TXT too; the name made in
;      RESULT2 (RESLEN2).
;   6. $DASSGN.
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

; ---- 2. create NOTES.TXT, one block, accessed for writing ----
	pushl	#0			; p6
	pushal	@#atrlst		; p5: the attributes to give it
	pushal	@#res1d			; p4: the name made
	pushal	@#reslen1		; p3: its length
	pushal	@#notesd		; p2: the name
	pushal	@#fibd			; p1: the FIB
	pushl	#^XF3			; IO$_CREATE!IO$M_CREATE!IO$M_ACCESS
	bsbw	qio
	jsb	@#check

; ---- 3. write the text ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#1			; p3: VBN
	movzwl	@#textlen, -(sp)	; p2: bytes
	pushal	@#text			; p1
	pushl	#^X30			; IO$_WRITEVBLK
	bsbw	qio
	jsb	@#check

; ---- 4. deaccess, recording the end of file: block 1, byte TEXTLEN ----
	movab	fat, r2			; (FAT+8 would be a forward reference
					; with an operator: see DEVIATIONS.md)
	movl	#^X10000, 8(r2)		; FAT$L_EFBLK = 1, high word first
	movw	@#textlen, ^D12(r2)	; FAT$W_FFBYTE
	pushl	#0
	pushal	@#atrlst		; p5
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#^X34			; IO$_DEACCESS
	bsbw	qio
	jsb	@#check

; ---- 5. enter the same file as ALIAS.TXT ----
	pushl	#0
	pushl	#0
	pushal	@#res2d			; p4
	pushal	@#reslen2		; p3
	pushal	@#aliasd		; p2
	pushal	@#fibd			; p1: the file ID from step 2
	pushl	#^X33			; IO$_CREATE
	bsbw	qio
	jsb	@#check

; ---- 6. deassign ----
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
notes:	.ascii	"NOTES.TXT"
notese:
notesd:	.long	notese-notes, notes
alias:	.ascii	"ALIAS.TXT"
aliase:
aliasd:	.long	aliase-alias, alias
; The FIB: access control FIB$M_WRITE; file ID 0 (filled in by the
; create); directory ID (4,4,0), the master file directory; name control
; 0; extend control FIB$M_EXTEND, for FIB$L_EXSZ 1 block.
fib:	.long	^X100			; FIB$L_ACCTL
	.word	0, 0, 0			; FIB$W_FID
	.word	4, 4, 0			; FIB$W_DID
	.long	0			; FIB$L_WCC
	.word	0			; FIB$W_NMCTL
	.word	^X80			; FIB$W_EXCTL: FIB$M_EXTEND
	.long	1			; FIB$L_EXSZ
	.long	0			; FIB$L_EXVBN
	.blkb	^D32			; the rest
fibe:
fibd:	.long	fibe-fib, fib
; The attribute list: ATR$C_RECATTR (4), 32 bytes, at FAT.
atrlst:	.word	^D32, 4
	.long	fat
	.long	0
; The record attribute area ($FATDEF): FAT$B_RTYPE 5 (STREAM_LF),
; FAT$B_RATT 2 (FAT$M_IMPLIEDCC); the end of file is set before deaccess.
fat:	.byte	5, 2
	.blkb	^D30
text:	.ascii	"Created by $QIO."<^D10>	; ending in a line feed
texte:
textlen: .word	texte-text
res1d:	.long	^D20, result1
res2d:	.long	^D20, result2
reslen1: .word	0
reslen2: .word	0
result1: .blkb	^D20
result2: .blkb	^D20
chan:	.word	0
iosb:	.long	0, 0
retpc:	.long	0

	.end	main
