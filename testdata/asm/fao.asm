;
; testdata/asm/fao.asm -- docs/PHASE-26.md subtask 24's acceptance
; fixture: $FAO and $FAOL building text that $QIOW then writes to the
; terminal, the way nearly every MACRO-32 program that prints does it.
;
;   1. $ASSIGN a channel to TTA0:.
;   2. $FAO with "!AS has !UL file!%S at !XL!/" and three parameters in
;      the call: the name (a descriptor), 3, and ^X1F4. $QIOW writes the
;      FAOLEN bytes it produced.
;   3. $FAOL with "!AC:!3(4UB)!/" and its parameters in PRMLST: a counted
;      string and three bytes. $QIOW writes that too.
;   4. $FAO with a directive it doesn't know (!QQ) returns SS$_BADPARAM
;      (^X14).
;
; R0 is 1 at the end if every call behaved; the Go test checks the text
; written to the terminal:
;
;	"SYSTEM has 3 files at 000001F4\r\nBYTES:   1  22 255\r\n"

	.microkernel
	.p1vector

	.entry	main, ^m<>

; ---- 1. $ASSIGN(TTA0:) ----
	pushl	#0			; mbxnam
	pushl	#0			; acmode
	pushal	@#chan
	pushal	@#devnam
	calls	#4, @#sys$assign
	blbs	r0, ok1
	brw	fail
ok1:

; ---- 2. $FAO_S CTRSTR=CTRL1, OUTLEN=FAOLEN, OUTBUF=FAODSC, P1..P3 ----
	pushl	#^X1F4			; p3: shown in hexadecimal
	pushl	#3			; p2: the file count
	pushal	@#name			; p1: a descriptor, for !AS
	pushal	@#faodsc		; outbuf
	pushal	@#faolen		; outlen
	pushal	@#ctrl1			; ctrstr
	calls	#6, @#sys$fao
	blbs	r0, ok2
	brw	fail
ok2:	bsbw	write

; ---- 3. $FAOL_S CTRSTR=CTRL2, OUTLEN=FAOLEN, OUTBUF=FAODSC, PRMLST ----
	pushal	@#prmlst
	pushal	@#faodsc
	pushal	@#faolen
	pushal	@#ctrl2
	calls	#4, @#sys$faol
	blbs	r0, ok3
	brw	fail
ok3:	bsbw	write

; ---- 4. an unknown directive ----
	pushl	#0
	pushal	@#faodsc
	pushal	@#faolen
	pushal	@#ctrl3
	calls	#4, @#sys$fao
	cmpl	r0, #^X14		; SS$_BADPARAM
	beql	ok4
	brw	fail
ok4:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

; write: $QIOW IO$_WRITEVBLK of the FAOLEN bytes at FAOBUF.
write:	pushl	#0			; p6
	pushl	#0			; p5
	pushl	#0			; p4: no carriage control
	pushl	#0			; p3
	movzwl	@#faolen, -(sp)		; p2: the length $FAO returned
	pushal	@#faobuf		; p1
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushl	#0			; iosb
	pushl	#^X30			; func: IO$_WRITEVBLK
	movzwl	@#chan, -(sp)
	pushl	#0			; efn
	calls	#^D12, @#sys$qiow
	blbs	r0, wrok
	brw	fail
wrok:	rsb

chan:	.word	0
faolen:	.word	0
devnam:	.ascid	"TTA0"
name:	.ascid	"SYSTEM"
ctrl1:	.ascid	"!AS has !UL file!%S at !XL!/"
ctrl2:	.ascid	"!AC:!3(4UB)!/"
ctrl3:	.ascid	"!QQ"

; A counted string (a length byte, then the text), for !AC.
bytes:	.byte	5
	.ascii	"BYTES"

prmlst:	.long	bytes, 1, ^D22, ^D255

; The output buffer and its descriptor (80 bytes).
faodsc:	.long	^D80
	.long	faobuf
faobuf:	.blkb	^D80

	.end	main
