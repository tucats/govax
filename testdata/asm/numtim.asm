;
; testdata/asm/numtim.asm -- docs/PHASE-26.md subtask 20's acceptance
; fixture: $BINTIM converts two text times to binary, and $NUMTIM
; breaks each into its seven numeric fields.
;
;   1. "29-FEB-2000 23:59:59.99" (absolute) into ABSFLD.
;   2. "5 03:18:32.07" (a delta) into DELFLD: year and month 0, day 5.
;
; R0 is 1 at the end if every call succeeded; the Go test checks the
; fields.

	.microkernel
	.p1vector

	.entry	main, ^m<>

	pushal	@#abstim
	pushal	@#abstxt
	calls	#2, @#sys$bintim
	blbc	r0, fail
	pushal	@#abstim
	pushal	@#absfld
	calls	#2, @#sys$numtim
	blbc	r0, fail

	pushal	@#deltim
	pushal	@#deltxt
	calls	#2, @#sys$bintim
	blbc	r0, fail
	pushal	@#deltim
	pushal	@#delfld
	calls	#2, @#sys$numtim
	blbc	r0, fail

	movl	#1, r0
	ret

fail:	movl	#0, r0
	ret

abstxt:	.ascid	"29-FEB-2000 23:59:59.99"
deltxt:	.ascid	"5 03:18:32.07"

abstim:	.long	0, 0
deltim:	.long	0, 0

; Seven words each: year, month, day, hour, minute, second, hundredths.
absfld:	.blkb	^D14
delfld:	.blkb	^D14

	.end	main
