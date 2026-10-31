$ ! MTH77.COM - the Phase 77 probe of MTHRTL.
$ ! Written by testdata/mth77/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @MTH77/OUTPUT=MTH77.LOG
$ !
$ ! MTH77.OUT gets the program's output; MTH77.LOG this procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEFINE/USER_MODE SYS$OUTPUT MTH77.MLG
$ MACRO/NOLIST MTH77
$ LINK/NOMAP MTH77
$ DEFINE/USER_MODE SYS$OUTPUT MTH77.OUT
$ RUN MTH77
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE MTH77.*
$ EXIT
