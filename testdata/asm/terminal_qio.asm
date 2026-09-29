;
; testdata/asm/terminal_qio.asm -- docs/PHASE-26.md subtask 17's
; acceptance fixture: terminal I/O through $QIO/$QIOW, the way VMS
; programs talk to their terminal.
;
;   1. $ASSIGN a channel to TTA0:.
;   2. $QIOW IO$_READPROMPT: writes "Name? " and reads a line into
;      BUFFER. The IOSB says how many characters came back, and that
;      RETURN (13) ended the line.
;   3. $QIO IO$_WRITEVBLK, asynchronously: "Hello, " with FORTRAN
;      carriage control " " (start a new line, return the carriage
;      after), an AST routine (WRTAST) and event flag 5. $WAITFR waits
;      for the flag; by then the AST has run and recorded its parameter.
;   4. $QIOW IO$_WRITEVBLK of the name that was read, with no carriage
;      control.
;   5. $QIOW with a function a terminal doesn't have (63) is rejected
;      in R0 with SS$_ILLIOFUNC (244).
;   6. $CANCEL finds nothing outstanding; $DASSGN releases the channel.
;
; R0 is 1 at the end if every step did what it should, 0
; otherwise. The Go test supplies "Tom" as the typed line and checks the
; output.

	.microkernel
	.p1vector

	.entry	main, ^m<>

	mtpr	#0, #^X12		; IPL 0: ASTs can be delivered

; ---- 1. $ASSIGN(TTA0:) ----
	pushl	#0			; mbxnam
	pushl	#0			; acmode
	pushal	@#chan
	pushal	@#devnam
	calls	#4, @#sys$assign
	blbs	r0, ok1
	brw	fail
ok1:

; ---- 2. $QIOW(IO$_READPROMPT, BUFFER, 20, prompt "Name? ") ----
	pushl	#6			; p6: prompt size
	pushal	@#prompt		; p5: prompt
	pushl	#0			; p4: default terminators
	pushl	#0			; p3: no time limit
	pushl	#^D20			; p2: buffer size
	pushal	@#buffer		; p1: buffer
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushal	@#iosb			; iosb
	pushl	#^X37			; func: IO$_READPROMPT
	movzwl	@#chan, -(sp)		; chan
	pushl	#0			; efn
	calls	#^D12, @#sys$qiow
	blbs	r0, ok2
	brw	fail
ok2:	cmpw	@#iosb, #1		; the I/O's own status: SS$_NORMAL
	beql	ok3
	brw	fail
ok3:	movzwl	@#iocnt, @#namlen	; the transfer count
	cmpw	@#ioterm, #^D13		; ended by RETURN (13)
	beql	ok4
	brw	fail
ok4:

; ---- 3. $QIO(efn 5, IO$_WRITEVBLK "Hello, ", CC " ", AST WRTAST(9)) ----
	pushl	#0			; p6
	pushl	#0			; p5
	pushl	#^X20			; p4: FORTRAN carriage control " "
	pushl	#0			; p3
	pushl	#7			; p2: size
	pushal	@#hello			; p1
	pushl	#9			; astprm
	pushal	@#wrtast		; astadr
	pushal	@#iosb2			; iosb
	pushl	#^X30			; func: IO$_WRITEVBLK
	movzwl	@#chan, -(sp)
	pushl	#5			; efn
	calls	#^D12, @#sys$qio
	blbs	r0, ok5
	brw	fail
ok5:	pushl	#5
	calls	#1, @#sys$waitfr
	cmpl	@#astparam, #9		; the AST ran
	beql	ok6
	brw	fail
ok6:	cmpw	@#iocnt2, #7		; seven bytes written
	beql	ok7
	brw	fail
ok7:

; ---- 4. $QIOW(IO$_WRITEVBLK the name, no carriage control) ----
	pushl	#0
	pushl	#0
	pushl	#0			; p4: no carriage control
	pushl	#0
	pushl	@#namlen		; p2
	pushal	@#buffer		; p1
	pushl	#0
	pushl	#0
	pushl	#0			; no IOSB
	pushl	#^X30
	movzwl	@#chan, -(sp)
	pushl	#0
	calls	#^D12, @#sys$qiow
	blbs	r0, ok8
	brw	fail
ok8:

; ---- 5. an illegal function ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#0
	pushl	#^D63			; no terminal function 63
	movzwl	@#chan, -(sp)
	pushl	#0
	calls	#^D12, @#sys$qiow
	cmpl	r0, #^XF4		; SS$_ILLIOFUNC
	beql	ok9
	brw	fail
ok9:

; ---- 6. $CANCEL, $DASSGN ----
	movzwl	@#chan, -(sp)
	calls	#1, @#sys$cancel
	blbs	r0, ok10
	brw	fail
ok10:	movzwl	@#chan, -(sp)
	calls	#1, @#sys$dassgn
	blbs	r0, ok11
	brw	fail
ok11:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; WRTAST(astprm): records its parameter.
	.entry	wrtast, ^m<>
	movl	b^4(ap), @#astparam
	ret

devnam:	.ascid	"TTA0"
prompt:	.ascii	"Name? "
hello:	.ascii	"Hello, "

chan:	.word	0
	.word	0
; The I/O status blocks, a label on each word: status, transfer count,
; and (for a read) terminator and terminator size.
iosb:	.word	0
iocnt:	.word	0
ioterm:	.word	0
	.word	0
iosb2:	.word	0
iocnt2:	.word	0
	.long	0
namlen:	.long	0
astparam: .long	0
buffer:	.blkb	^D20

	.end	main
