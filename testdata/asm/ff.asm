        .entry  main, ^m<>

        ffs     #1, #^x3, @#data, r0
        ret

data:   .long   ^X0505050

        .end    main
