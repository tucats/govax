
        .entry test, ^m<>

        movl   #^X14, r0
        insv   #^X0d, r0, #3, @#bits
        ret

bits:   .long   0
        .long   0

