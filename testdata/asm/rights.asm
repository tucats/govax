;
; testdata/asm/rights.asm -- docs/PHASE-26.md subtask 40's acceptance
; fixture: rights identifiers.
;
;   1. $ASCTOID translates "interactive" to its value (ID).
;   2. $FAO "!%I and !%I" with that value and SYSTEM's UIC ([1,4]) makes
;      "INTERACTIVE and [SYSTEM]" (TEXT, TEXTLEN).
;   3. $IDTOASC translates the value back to its name (NAME, NAMLEN).
;   4. $IDTOASC with id -1 lists the rights database until SS$_NOSUCHID;
;      COUNT is how many identifiers it returned.
;
; R0 is 1 at the end if every call behaved.

	.microkernel
	.p1vector

	.entry	main, ^m<r2>

; ---- 1. name to value ----
	pushl	#0			; attrib
	pushal	@#id
	pushal	@#iname
	calls	#3, @#sys$asctoid
	jsb	@#check

; ---- 2. !%I ----
	pushl	#^X00010004		; SYSTEM's UIC
	pushl	@#id
	pushal	@#textd			; outbuf
	pushal	@#textlen		; outlen
	pushal	@#ctrl
	calls	#5, @#sys$fao
	jsb	@#check

; ---- 3. value to name ----
	pushl	#0			; contxt
	pushl	#0			; attrib
	pushl	#0			; resid
	pushal	@#named			; nambuf
	pushal	@#namlen
	pushl	@#id
	calls	#6, @#sys$idtoasc
	jsb	@#check

; ---- 4. the whole database ----
list:	pushal	@#contxt
	pushl	#0
	pushl	#0
	pushal	@#named
	pushal	@#namlen
	pushl	#-1
	calls	#6, @#sys$idtoasc
	cmpl	r0, #^X21EC		; SS$_NOSUCHID: done
	beql	done
	jsb	@#check
	incl	@#count
	brb	list

done:	movl	#1, r0
	ret

; CHECK (by JSB): return from MAIN with R0 0 unless R0 is a success.
check:	blbs	r0, checkok
	clrl	r0
	ret
checkok: rsb

iname:	.ascid	"interactive"
ctrl:	.ascid	"!%I and !%I"
text:	.blkb	^D40
textd:	.long	^D40, text
textlen: .word	0
name:	.blkb	^D31
named:	.long	^D31, name
namlen:	.word	0
id:	.long	0
contxt:	.long	0
count:	.long	0

	.end	main
