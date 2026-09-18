        .entry  main, ^m<>
        pushal  @#msg
        calls   #1,@#lib$put_output

        ret

msg:    .ascid  "Hello world"
        .end    main

