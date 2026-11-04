$ ! FOR88.COM - round 2 of the Phase 88 probe: FORRTL on indexed files.
$ ! Written by testdata/for88/gen.go. Run it with the exchange volume's [000000]
$ ! as the default directory:
$ !
$ !     @FOR88/OUTPUT=FOR88.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ MACRO/NOLIST FOR88
$ LINK/NOMAP FOR88,FOR88/OPTIONS
$ ! Through a procedure with /OUTPUT, so that SYS$OUTPUT is a process-permanent file.
$ @FOR88B/OUTPUT=FOR88-VAX.TXT
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE=ALL/DATE *.IDX
$ DIRECTORY/FULL STD.IDX
$ ANALYZE/RMS_FILE/FDL/OUTPUT=STD.FDL STD.IDX
$ TYPE STD.FDL
$ ANALYZE/RMS_FILE/CHECK W7.IDX
$ ! The files FORRTL made, for comparing with FTNRTL's.
$ BACKUP *.IDX;* FOR88.BCK/SAVE_SET/LOG
$ EXIT
