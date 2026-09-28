;
; testdata/asm/mailbox_wait.asm -- docs/PHASE-26.md subtask 37's
; acceptance fixture: a mailbox's attention ASTs and resource wait mode.
;
;   1. $CREMBX a mailbox holding at most 16 bytes of messages (CHAN).
;   2. IO$_SETMODE!IO$M_READATTN enables a read attention AST (RATTN,
;      parameter ^X11). An IO$M_NOW write of "hello" then delivers it:
;      RATTN records its parameter (RPARAM) and reads the message (MSG1).
;   3. A 16-byte write fills the mailbox. $SETIMR arms a 10ms timer whose
;      AST (RDAST) reads one message (MSG2). Then a $QIOW write of "wxyz"
;      finds the mailbox full: with resource wait mode on (VMS's
;      default) the program waits until the timer's AST has made room,
;      then the write goes in (WSTAT, its IOSB status), and a read gets
;      it (MSG3).
;   4. $SETRWM(1) turns resource wait mode off (RWM1: SS$_WASCLR, it was
;      on). The mailbox filled again, a write completes at once with
;      SS$_MBFULL (FULLST). $SETRWM(0) turns it back on (RWM2:
;      SS$_WASSET).
;
; R0 is 1 at the end if every call behaved; the Go test checks the
; recorded values.

	.microkernel
	.p1vector

	.entry	main, ^m<>
	mtpr	#0, #^X12		; IPL 0: ASTs can be delivered

; ---- 1. the mailbox ----
	pushl	#0			; lognam
	pushl	#0			; acmode
	pushl	#0			; promsk
	pushl	#^D16			; bufquo
	pushl	#^D16			; maxmsg
	pushal	@#chan
	pushl	#0			; prmflg: temporary
	calls	#7, @#sys$crembx
	blbs	r0, ok1
	brw	fail
ok1:

; ---- 2. a read attention AST ----
	pushl	#0			; p6
	pushl	#0			; p5
	pushl	#0			; p4
	pushl	#0			; p3: access mode
	pushl	#^X11			; p2: AST parameter
	pushal	@#rattn			; p1: AST routine
	pushl	#0
	pushl	#0
	pushal	@#iosb
	pushl	#^XA3			; IO$_SETMODE!IO$M_READATTN
	movzwl	@#chan, -(sp)
	pushl	#0
	calls	#^D12, @#sys$qiow
	blbs	r0, ok2
	brw	fail
ok2:	pushal	@#hello
	pushl	#5
	bsbw	write			; RATTN runs right after

; ---- 3. a writer waiting for room ----
	pushal	@#sixteen
	pushl	#^D16
	bsbw	write			; the mailbox is full now
	pushl	#0			; flags
	pushl	#0			; reqidt
	pushal	@#rdast			; astadr
	pushal	@#delta10		; daytim
	pushl	#2			; efn
	calls	#5, @#sys$setimr
	pushal	@#wxyz
	pushl	#4
	bsbw	write			; waits until RDAST reads
	movzwl	@#iosb, @#wstat
	pushal	@#msg3
	bsbw	read

; ---- 4. resource wait mode off: SS$_MBFULL ----
	pushl	#1
	calls	#1, @#sys$setrwm
	movl	r0, @#rwm1
	pushal	@#sixteen
	pushl	#^D16
	bsbw	write
	pushal	@#wxyz
	pushl	#1
	bsbw	write			; full: completes with SS$_MBFULL
	movzwl	@#iosb, @#fullst
	pushl	#0
	calls	#1, @#sys$setrwm
	movl	r0, @#rwm2

	movl	#1, r0
	ret

fail:	clrl	r0
	ret

; read(buffer): $QIOW IO$_READVBLK of up to 16 bytes.
read:	movl	4(sp), r1		; the buffer
	pushl	#0			; p6
	pushl	#0			; p5
	pushl	#0			; p4
	pushl	#0			; p3
	pushl	#^D16			; p2
	pushl	r1			; p1
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushl	#0			; iosb
	pushl	#^X31			; IO$_READVBLK
	movzwl	@#chan, -(sp)
	pushl	#3			; efn
	calls	#^D12, @#sys$qiow
	blbs	r0, rdok
	brw	fail
rdok:	movl	(sp)+, r1		; the return address
	tstl	(sp)+			; drop the argument
	jmp	(r1)

; write(size, data): $QIOW IO$_WRITEVBLK!IO$M_NOW, status in IOSB.
write:	movl	4(sp), r1		; the size
	movl	8(sp), r0		; the data
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	r1			; p2
	pushl	r0			; p1
	pushl	#0
	pushl	#0
	pushal	@#iosb
	pushl	#^X70			; IO$_WRITEVBLK!IO$M_NOW
	movzwl	@#chan, -(sp)
	pushl	#0
	calls	#^D12, @#sys$qiow
	blbs	r0, wrok
	brw	fail
wrok:	movl	(sp)+, r1
	addl2	#8, sp			; drop the arguments
	jmp	(r1)

; RATTN: the read attention AST records its parameter and reads.
	.entry	rattn, ^m<>
	movl	4(ap), @#rparam
	pushal	@#msg1
	bsbw	read
	ret

; RDAST: the timer's AST reads one message, making room.
	.entry	rdast, ^m<>
	pushal	@#msg2
	bsbw	read
	ret

chan:	.word	0
iosb:	.long	0, 0
rparam:	.long	0
wstat:	.long	0
fullst:	.long	0
rwm1:	.long	0
rwm2:	.long	0
hello:	.ascii	"hello"
sixteen: .ascii	"0123456789ABCDEF"
wxyz:	.ascii	"wxyz"
delta10: .long	^XFFFE7960, ^XFFFFFFFF	; -100000: 10ms
msg1:	.blkb	^D16
msg2:	.blkb	^D16
msg3:	.blkb	^D16

	.end	main
