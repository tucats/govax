;
; testdata/asm/getsyi.asm -- docs/PHASE-26.md subtask 23's acceptance
; fixture: $GETSYIW, as the VMS manual's own example uses it, and the
; wildcard loop programs use to visit every node of a cluster.
;
;   1. $GETSYIW for SYI$_VERSION (^X1000, 8 bytes) and SYI$_NODENAME
;      (^X10D9, up to 15 bytes, its length returned in NAMLEN), with an
;      IOSB.
;   2. A wildcard scan: CSID starts at -1, and $GETSYIW is called until it
;      returns SS$_NOMORENODE (^XA00), counting the nodes it visits in
;      NODES. A single system is one node.
;
; R0 is 1 at the end if every call behaved; the Go test checks the
; strings and the count.

	.microkernel
	.p1vector

	.entry	main, ^m<>

; ---- 1. version and node name ----
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushal	@#iosb
	pushal	@#itmlst
	pushl	#0			; nodename
	pushl	#0			; csidadr
	pushl	#0			; efn
	calls	#7, @#sys$getsyiw
	blbs	r0, ok1
	brw	fail
ok1:	cmpl	@#iosb, #1
	beql	ok2
	brw	fail
ok2:

; ---- 2. every node in the cluster ----
	movl	#-1, @#csid
loop:	pushl	#0
	pushl	#0
	pushl	#0			; no IOSB
	pushal	@#wildlst
	pushl	#0
	pushal	@#csid
	pushl	#0
	calls	#7, @#sys$getsyiw
	cmpl	r0, #^XA00		; SS$_NOMORENODE: done
	beql	done
	blbs	r0, ok3
	brw	fail
ok3:	incl	@#nodes
	brb	loop
done:

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

iosb:	.long	0, 0
csid:	.long	0
nodes:	.long	0
namlen:	.word	0
version: .blkb	8
node:	.blkb	^D15
wnode:	.blkb	^D15

; Item lists: buffer length, item code, buffer, return length address.
itmlst:	.word	8, ^X1000
	.long	version, 0
	.word	^D15, ^X10D9
	.long	node, namlen
	.long	0
wildlst: .word	^D15, ^X10D9
	.long	wnode, 0
	.long	0

	.end	main
