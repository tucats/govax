
	.entry	main, ^m<r4,r5,r6,r7>

        movl    #^X00110022, r6
        movl    #^X00330044, r7
        movq    r6, data2

	movq	@#data, r4
	movl	#1, r0
	ret

data:	.long	^x11111111
	.long	^x22222222

data2:  .long    0,0

	.end	main

