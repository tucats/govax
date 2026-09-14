
	.entry test,^m<>

	movl	#0ff,@#rbuff
	pushal	@#rbuff
	pushal	@#pbuff
	pushal	@#rbuff
	calls	#3,@#lib$get_input
	ret

rbuff:	.ascid  ""
	.blkb	255

pbuff:	.ascid	"Prompt> "

