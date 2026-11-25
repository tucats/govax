$ ! FOR81K.COM - round 3b of testdata/for81, round 3's follow-up.
$ ! Written by testdata/for81/gen.go. Run it from the exchange volume's [000000]
$ ! (testdata/for81's README, "Round 3b"):
$ !
$ !     @FOR81K/OUTPUT=FOR81K.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ MACRO/NOLIST FOR81K
$ LINK/NOMAP FOR81K,FOR81/OPTIONS
$ MACRO/NOLIST FOR81W
$ LINK/NOMAP FOR81W,FOR81/OPTIONS
$ !
$ ! Through a procedure with /OUTPUT, so that unit 6's records join the cases'
$ ! lines in one file.
$ @FOR81L/OUTPUT=FOR81K-VAX.TXT
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE=ALL *.DAT,*.IDX
$ EXIT
