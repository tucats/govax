;
; testdata/asm/mailbox.asm -- docs/PHASE-26.md subtask 29's acceptance
; fixture: a mailbox passing messages, including a read that has to wait
; for its message.
;
;   1. $CREMBX a temporary mailbox with the logical name MBXDEMO; its
;      channel is RCHAN. $ASSIGN a second channel, WCHAN, through the
;      logical name.
;   2. An IO$M_NOW write of "first" on WCHAN, then a $QIOW read on RCHAN:
;      the message is there already.
;   3. $SETIMR for 10ms, with an AST (WRTAST) that writes "second". Then
;      a $QIOW read on RCHAN: the mailbox is empty, so it waits, while
;      the timer runs and its AST writes, and then returns with "second".
;   4. A $QIOW read with IO$M_NOW of the now-empty mailbox: end of file
;      (^X870) in the IOSB.
;   5. $DASSGN both channels, which deletes the temporary mailbox.
;
; R0 is 1 at the end if every call behaved; the Go test checks the two
; messages read and that the mailbox is gone.

	.microkernel
	.p1vector

	.entry	main, ^m<>

	mtpr	#0, #^X12		; IPL 0: ASTs can be delivered

; ---- 1. the mailbox and two channels ----
	pushal	@#mbxnam		; lognam
	pushl	#0			; acmode
	pushl	#0			; promsk
	pushl	#0			; bufquo
	pushl	#0			; maxmsg
	pushal	@#rchan			; chan
	pushl	#0			; prmflg: temporary
	calls	#7, @#sys$crembx
	blbs	r0, ok1
	brw	fail
ok1:	pushl	#0
	pushl	#0
	pushal	@#wchan
	pushal	@#mbxnam
	calls	#4, @#sys$assign
	blbs	r0, ok2
	brw	fail
ok2:

; ---- 2. write "first" (NOW), then read it ----
	pushal	@#first
	pushl	#5
	bsbw	write
	pushal	@#msg1
	bsbw	read
	cmpw	@#iosb, #1
	beql	ok3
	brw	fail
ok3:

; ---- 3. a read that waits for the timer AST's write ----
	pushl	#0			; flags
	pushl	#0			; reqidt
	pushal	@#wrtast		; astadr
	pushal	@#delta10		; daytim
	pushl	#2			; efn
	calls	#5, @#sys$setimr
	pushal	@#msg2
	bsbw	read
	cmpw	@#iosb, #1
	beql	ok4
	brw	fail
ok4:	cmpw	@#iocnt, #6		; the length of "second"
	beql	ok5
	brw	fail
ok5:

; ---- 4. an empty mailbox, IO$M_NOW: end of file ----
	pushl	#0			; p6
	pushl	#0			; p5
	pushl	#0			; p4
	pushl	#0			; p3
	pushl	#^D16			; p2
	pushal	@#msg2			; p1
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushal	@#iosb
	pushl	#^X71			; IO$_READVBLK!IO$M_NOW
	movzwl	@#rchan, -(sp)
	pushl	#0
	calls	#^D12, @#sys$qiow
	cmpw	@#iosb, #^X870		; SS$_ENDOFFILE
	beql	ok6
	brw	fail
ok6:

; ---- 5. deassign both channels ----
	movzwl	@#wchan, -(sp)
	calls	#1, @#sys$dassgn
	movzwl	@#rchan, -(sp)
	calls	#1, @#sys$dassgn

	movl	#1, r0
	ret

fail:	clrl	r0
	ret

; read(buffer): $QIOW IO$_READVBLK of up to 16 bytes on RCHAN.
read:	movl	4(sp), r1		; the buffer
	pushl	#0			; p6
	pushl	#0			; p5
	pushl	#0			; p4
	pushl	#0			; p3
	pushl	#^D16			; p2
	pushl	r1			; p1
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushal	@#iosb
	pushl	#^X31			; IO$_READVBLK
	movzwl	@#rchan, -(sp)
	pushl	#0			; efn
	calls	#^D12, @#sys$qiow
	blbs	r0, rdok
	brw	fail
rdok:	movl	(sp)+, r1		; the return address
	tstl	(sp)+			; drop the argument
	jmp	(r1)

; write(size, data): $QIOW IO$_WRITEVBLK!IO$M_NOW on WCHAN.
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
	pushl	#0			; iosb
	pushl	#^X70			; IO$_WRITEVBLK!IO$M_NOW
	movzwl	@#wchan, -(sp)
	pushl	#0
	calls	#^D12, @#sys$qiow
	blbs	r0, wrok
	brw	fail
wrok:	movl	(sp)+, r1
	addl2	#8, sp			; drop the arguments
	jmp	(r1)

; WRTAST: the timer's AST writes "second" (IO$M_NOW, so it completes
; at once, handing the message to the waiting read).
	.entry	wrtast, ^m<>
	pushal	@#second
	pushl	#6
	bsbw	write
	ret

rchan:	.word	0
wchan:	.word	0
iosb:	.word	0			; the IOSB: status,
iocnt:	.word	0			; transfer count,
	.long	0			; and the other process's PID
mbxnam:	.ascid	"MBXDEMO"
first:	.ascii	"first"
second:	.ascii	"second"
delta10: .long	^XFFFE7960, ^XFFFFFFFF	; -100000: 10ms
msg1:	.blkb	^D16
msg2:	.blkb	^D16

	.end	main
