;
; testdata/asm/getdvi.asm -- docs/PHASE-26.md subtask 28's acceptance
; fixture: $GETDVI and $GETDVIW, asking about a terminal.
;
;   1. $ASSIGN a channel to TTA0:.
;   2. $GETDVIW by channel for DVI$_DEVNAM (^X20, into NAME, its length in
;      NAMLEN), DVI$_DEVCLASS (4), DVI$_UNIT (^XC), and DVI$_REFCNT (^X1E).
;   3. $GETDVI (the asynchronous form) by name -- SYS$OUTPUT, a logical
;      name for the terminal -- for DVI$_FULLDEVNAM (^XE8), with event
;      flag 3 and an IOSB, then $SYNCH on them.
;
; R0 is 1 at the end if every call behaved; the Go test checks the values.

	.microkernel
	.p1vector

	.entry	main, ^m<>

; ---- 1. $ASSIGN(TTA0:) ----
	pushl	#0
	pushl	#0
	pushal	@#chan
	pushal	@#devnam
	calls	#4, @#sys$assign
	blbs	r0, ok1
	brw	fail
ok1:

; ---- 2. $GETDVIW by channel ----
	pushl	#0			; nullarg
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushl	#0			; iosb
	pushal	@#itmlst
	pushl	#0			; devnam
	movzwl	@#chan, -(sp)		; chan
	pushl	#0			; efn
	calls	#8, @#sys$getdviw
	blbs	r0, ok2
	brw	fail
ok2:

; ---- 3. $GETDVI by name, then $SYNCH ----
	pushl	#0
	pushl	#0
	pushl	#0
	pushal	@#iosb
	pushal	@#fullst
	pushal	@#outnam		; devnam
	pushl	#0			; chan
	pushl	#3			; efn
	calls	#8, @#sys$getdvi
	blbs	r0, ok3
	brw	fail
ok3:	pushal	@#iosb
	pushl	#3
	calls	#2, @#sys$synch
	blbs	r0, ok4
	brw	fail
ok4:	cmpl	@#iosb, #1
	beql	ok5
	brw	fail
ok5:

	movl	#1, r0
	ret

fail:	clrl	r0
	ret

chan:	.word	0
devnam:	.ascid	"TTA0"
outnam:	.ascid	"SYS$OUTPUT"
iosb:	.long	0, 0

namlen:	.word	0
fullen:	.word	0
class:	.long	0
unit:	.long	^XFF
refcnt:	.long	0
name:	.blkb	^D64
full:	.blkb	^D64

; Item lists: buffer length, item code, buffer, return length address.
itmlst:	.word	^D64, ^X20
	.long	name, namlen
	.word	4, 4
	.long	class, 0
	.word	4, ^XC
	.long	unit, 0
	.word	4, ^X1E
	.long	refcnt, 0
	.long	0
fullst:	.word	^D64, ^XE8
	.long	full, fullen
	.long	0

	.end	main
