$ ! FOR81.COM - the Phase 81 probe of FORRTL's file statements and RMS's files.
$ ! Written by testdata/for81/gen.go. Run it with the exchange volume's [000000]
$ ! as the default directory:
$ !
$ !     @FOR81/OUTPUT=FOR81.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ MACRO/NOLIST FOR81
$ LINK/NOMAP FOR81,FOR81/OPTIONS
$ MACRO/NOLIST FOR81R
$ LINK/NOMAP FOR81R
$ !
$ ! RMS first: the files' bytes.
$ DEFINE/USER_MODE SYS$OUTPUT FOR81R-VAX.TXT
$ RUN FOR81R
$ SHOW SYMBOL $STATUS
$ !
$ ! FORRTL, through a procedure with /OUTPUT, so that SYS$OUTPUT is a process-permanent
$ ! file: the cases that open SYS$OUTPUT's units then add to this file and don't make
$ ! a new version of it. Unit 11 has a logical name.
$ DEFINE/NOLOG FOR011 NAMED.DAT
$ @FOR81B/OUTPUT=FOR81-VAX.TXT
$ SHOW SYMBOL $STATUS
$ DEASSIGN FOR011
$ !
$ ! The queues' state, for what PRINT and SUBMIT did, and what's left.
$ SHOW QUEUE/ALL
$ DIRECTORY/SIZE=ALL/DATE *.DAT
$ DUMP/BLOCK=COUNT:1 REL1.DAT
$ EXIT
