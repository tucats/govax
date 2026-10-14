$ ! LIB59.COM - the Phase 59 probe of LIBRTL's memory routines.
$ ! Written by testdata/lib59/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @LIB59/OUTPUT=LIB59.LOG
$ !
$ ! LIB59.OUT gets the program's output; LIB59.LOG this procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ SHOW PROCESS/QUOTAS
$ DEFINE/USER_MODE SYS$OUTPUT LIB59.MLG
$ MACRO/NOLIST LIB59
$ LINK/NOMAP LIB59
$ DEFINE/USER_MODE SYS$OUTPUT LIB59.OUT
$ RUN LIB59
$ SHOW SYMBOL $STATUS
