;
; testdata/asm/operator.asm -- docs/PHASE-26.md subtask 39's acceptance
; fixture: operator requests and broadcasts.
;
;   1. $CREMBX a mailbox for the operator's reply (CHAN).
;   2. $SNDOPR an OPC$_RQ_RQST to the CENTRAL operator, "Please mount
;      tape 17", with the request code ^X42 and the mailbox: OPCOM shows it
;      as request 1 on the console (the operator terminal).
;   3. Acting as the operator, $SNDOPR an OPC$_RQ_REPLY to request 1 with
;      status ^X0001 and the text "Mounted". A $QIOW read of the mailbox
;      gets the reply (REPLY), whose longword at +4 is the request code
;      ^X42.
;   4. $BRKTHRUW "Shutting down" to every terminal (BRK$C_ALLTERMS, 4),
;      with an IOSB (BIOSB).
;
; R0 is 1 at the end if every call behaved; the Go test checks the
; console output and the recorded values.

	.microkernel
	.p1vector

	.entry	main, ^m<>

; ---- 1. the reply mailbox ----
	pushl	#0			; lognam
	pushl	#0			; acmode
	pushl	#0			; promsk
	pushl	#0			; bufquo
	pushl	#0			; maxmsg
	pushal	@#chan
	pushl	#0
	calls	#7, @#sys$crembx
	jsb	@#check

; ---- 2. the request ----
	movzwl	@#chan, -(sp)
	pushal	@#rqstd
	calls	#2, @#sys$sndopr
	jsb	@#check

; ---- 3. the operator's reply, read from the mailbox ----
	pushl	#0
	pushal	@#replyd
	calls	#2, @#sys$sndopr
	jsb	@#check
	pushl	#0			; p6
	pushl	#0			; p5
	pushl	#0			; p4
	pushl	#0			; p3
	pushl	#^D40			; p2
	pushal	@#reply			; p1
	pushl	#0
	pushl	#0
	pushal	@#iosb
	pushl	#^X31			; IO$_READVBLK
	movzwl	@#chan, -(sp)
	pushl	#0
	calls	#^D12, @#sys$qiow
	jsb	@#check

; ---- 4. a broadcast ----
	pushl	#0			; astprm
	pushl	#0			; astadr
	pushl	#0			; timout
	pushl	#0			; reqid
	pushl	#0			; flags
	pushl	#0			; carcon
	pushal	@#biosb			; iosb
	pushl	#4			; sndtyp: BRK$C_ALLTERMS
	pushl	#0			; sendto
	pushal	@#bcast			; msgbuf
	pushl	#0			; efn
	calls	#^D11, @#sys$brkthruw
	jsb	@#check

	movl	#1, r0
	ret

; CHECK (by JSB): return from MAIN with R0 0 unless R0 is a success.
; RET works from here because FP is still MAIN's frame.
check:	blbs	r0, checkok
	clrl	r0
	ret
checkok: rsb

; OPC$_RQ_RQST: code 3, target CENTRAL (1), request code ^X42, text.
rqst:	.byte	3, 1, 0, 0
	.long	^X42
	.ascii	"Please mount tape 17"
rqste:
rqstd:	.long	rqste-rqst, rqst	; its descriptor (after it: the
					; assembler can't subtract forward
					; references)

; OPC$_RQ_REPLY: code 4, reserved, status ^X0001, request number 1,
; unit 0, the counted terminal name, the text.
replyb:	.byte	4, 0, 1, 0
	.long	1
	.word	0
	.byte	4
	.ascii	"OPA0"
	.ascii	"Mounted"
replye:
replyd:	.long	replye-replyb, replyb

bcast:	.ascid	"Shutting down"
chan:	.word	0
iosb:	.long	0, 0
biosb:	.long	0, 0
reply:	.blkb	^D40

	.end	main
