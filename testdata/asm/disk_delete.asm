;
; testdata/asm/disk_delete.asm -- docs/PHASE-26.md subtask 44's
; acceptance fixture: deleting with the disk's $QIO. The volume holds
; KEEP.TXT and OLD.TXT in [000000] to begin with.
;
;   1. $ASSIGN a channel to DUA0: (CHAN).
;   2. A temporary file: IO$_CREATE!IO$M_CREATE!IO$M_ACCESS!IO$M_DELETE of
;      SCRATCH.TMP, one block, accessed for writing. Write TEXT to it and
;      read it back into BUF: a working file while accessed. IO$_DEACCESS
;      deletes it, directory entry and all.
;   3. A rename, the way the ACP does it: look KEEP.TXT up (IO$_ACCESS
;      alone, its file ID into FIB2), enter that file as RENAMED.TXT
;      (IO$_CREATE, no IO$M_CREATE; the name made in RESULT2), then
;      remove the KEEP.TXT entry (IO$_DELETE, no IO$M_DELETE; the entry
;      removed in RESULT3). The file itself is untouched.
;   4. IO$_DELETE!IO$M_DELETE of OLD.TXT: entry and file.
;   5. $DASSGN.
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

; ---- 2. a temporary file: create, write, read, deaccess ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushal	@#scrd			; p2: SCRATCH.TMP
	pushal	@#fib1d			; p1
	pushl	#^X1F3			; IO$_CREATE!IO$M_CREATE!IO$M_ACCESS!IO$M_DELETE
	bsbw	qio
	jsb	@#check
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#1			; p3: VBN
	pushl	#^D12			; p2: bytes
	pushal	@#text			; p1
	pushl	#^X30			; IO$_WRITEVBLK
	bsbw	qio
	jsb	@#check
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#1
	pushl	#^D12
	pushal	@#buf
	pushl	#^X31			; IO$_READVBLK
	bsbw	qio
	jsb	@#check
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#^X34			; IO$_DEACCESS: the file goes
	bsbw	qio
	jsb	@#check

; ---- 3. rename KEEP.TXT to RENAMED.TXT ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushal	@#keepd			; p2: KEEP.TXT
	pushal	@#fib2d			; p1
	pushl	#^X32			; IO$_ACCESS: look it up
	bsbw	qio
	jsb	@#check
	pushl	#0
	pushl	#0
	pushal	@#res2d			; p4
	pushal	@#reslen2		; p3
	pushal	@#renamd		; p2: RENAMED.TXT
	pushal	@#fib2d			; p1: KEEP.TXT's file ID
	pushl	#^X33			; IO$_CREATE: enter it
	bsbw	qio
	jsb	@#check
	pushl	#0
	pushl	#0
	pushal	@#res3d			; p4
	pushal	@#reslen3		; p3
	pushal	@#keepd			; p2: KEEP.TXT
	pushal	@#fib2d			; p1
	pushl	#^X35			; IO$_DELETE: remove the old entry
	bsbw	qio
	jsb	@#check

; ---- 4. delete OLD.TXT ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushal	@#oldd			; p2: OLD.TXT
	pushal	@#fib3d			; p1
	pushl	#^X135			; IO$_DELETE!IO$M_DELETE
	bsbw	qio
	jsb	@#check

; ---- 5. deassign ----
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
scrd:	.ascid	"SCRATCH.TMP"
keepd:	.ascid	"KEEP.TXT"
renamd:	.ascid	"RENAMED.TXT"
oldd:	.ascid	"OLD.TXT"
; FIB1, the temporary file's: FIB$M_WRITE, directory (4,4,0), and
; FIB$M_EXTEND for one block.
fib1:	.long	^X100			; FIB$L_ACCTL
	.word	0, 0, 0			; FIB$W_FID
	.word	4, 4, 0			; FIB$W_DID
	.long	0			; FIB$L_WCC
	.word	0			; FIB$W_NMCTL
	.word	^X80			; FIB$W_EXCTL: FIB$M_EXTEND
	.long	1			; FIB$L_EXSZ
	.blkb	^D36
fib1e:
fib1d:	.long	fib1e-fib1, fib1
; FIB2 (the rename's) and FIB3 (the delete's): directory (4,4,0).
fib2:	.long	0
	.word	0, 0, 0
	.word	4, 4, 0
fib2e:
fib2d:	.long	fib2e-fib2, fib2
fib3:	.long	0
	.word	0, 0, 0
	.word	4, 4, 0
fib3e:
fib3d:	.long	fib3e-fib3, fib3
res2d:	.long	^D20, result2
res3d:	.long	^D20, result3
reslen2: .word	0
reslen3: .word	0
result2: .blkb	^D20
result3: .blkb	^D20
text:	.ascii	"Scratch data"
buf:	.blkb	^D12
chan:	.word	0
iosb:	.long	0, 0
retpc:	.long	0

	.end	main
