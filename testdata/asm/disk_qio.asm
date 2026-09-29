;
; testdata/asm/disk_qio.asm -- docs/PHASE-26.md subtask 41's acceptance
; fixture: virtual block I/O on a file of a mounted ODS-2 volume, through
; the disk's $QIO functions (the ACP), without RMS.
;
;   1. $ASSIGN a channel to DUA0: (CHAN).
;   2. IO$_ACCESS!IO$M_ACCESS with a FIB whose directory ID is the master
;      file directory's, (4,4,0), and whose access control asks for write
;      access (FIB$M_WRITE), looking up and opening DATA.TXT. The result
;      name comes back in RESULT (RESLEN), and the file ID in the FIB.
;   3. IO$_READVBLK of virtual block 1 into BUF1.
;   4. IO$_WRITEVBLK of "Hello, disk!" to virtual block 1.
;   5. IO$_READVBLK of virtual block 1 again, into BUF2.
;   6. IO$_DEACCESS, and $DASSGN.
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

; ---- 2. look DATA.TXT up and access it for writing ----
	pushl	#0			; p6
	pushl	#0			; p5: attributes
	pushal	@#resd			; p4: result name
	pushal	@#reslen		; p3: its length
	pushal	@#named			; p2: the name
	pushal	@#fibd			; p1: the FIB
	pushl	#^X72			; IO$_ACCESS!IO$M_ACCESS
	bsbw	qio
	jsb	@#check

; ---- 3-5. read, write, read block 1 ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#1			; p3: VBN
	pushl	#^D512			; p2: bytes
	pushal	@#buf1			; p1
	pushl	#^X31			; IO$_READVBLK
	bsbw	qio
	jsb	@#check
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#1
	pushl	#^D12
	pushal	@#hello
	pushl	#^X30			; IO$_WRITEVBLK
	bsbw	qio
	jsb	@#check
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#1
	pushl	#^D512
	pushal	@#buf2
	pushl	#^X31			; IO$_READVBLK
	bsbw	qio
	jsb	@#check

; ---- 6. deaccess and deassign ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#^X34			; IO$_DEACCESS
	bsbw	qio
	jsb	@#check
	movzwl	@#chan, -(sp)
	calls	#1, @#sys$dassgn
	jsb	@#check

	movl	#1, r0
	ret

; QIO (by BSBW, with the function and p1-p6 pushed, function last):
; $QIOW on CHAN with IOSB; returns with R0 the IOSB's status if $QIOW
; itself succeeded. The arguments are popped.
qio:	movl	(sp)+, @#retpc		; the return address
	movl	(sp)+, r1		; the function
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushal	@#iosb
	pushl	r1
	movzwl	@#chan, -(sp)
	pushl	#0			; efn
	calls	#^D12, @#sys$qiow	; takes efn..p6 off the stack
	blbc	r0, qiodone
	movzwl	@#iosb, r0
qiodone: movl	@#retpc, r1
	jmp	(r1)

; CHECK (by JSB): return from MAIN with R0 0 unless R0 is a success.
check:	blbs	r0, checkok
	clrl	r0
	ret
checkok: rsb

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
result:	.blkb	^D20
resd:	.long	^D20, result
reslen:	.word	0
hello:	.ascii	"Hello, disk!"
chan:	.word	0
iosb:	.long	0, 0
retpc:	.long	0
buf1:	.blkb	^D512
buf2:	.blkb	^D512

	.end	main
