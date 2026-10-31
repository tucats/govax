$ ! FOR81F.COM - round 2's FORRTL program again, after its first run ran away.
$ ! Written by testdata/for81/gen.go. Run it with the exchange volume's [000000]
$ ! as the default directory:
$ !
$ !     @FOR81F/OUTPUT=FOR81F.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ MACRO/NOLIST FOR81C
$ LINK/NOMAP FOR81C,FOR81/OPTIONS
$ @FOR81E/OUTPUT=FOR81F-VAX.TXT
$ SHOW SYMBOL $STATUS
$ EXIT
