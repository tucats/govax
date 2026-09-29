
	.entry test,^m<>

	movl	#^X0ff,@#rbuff
	pushal	@#rbuff
	pushal	@#pbuff
	pushal	@#rbuff
	calls	#3,@#lib$get_input
	ret

rbuff:	.ascid  ""
	.blkb	^X255

pbuff:	.ascid	"Prompt> "

