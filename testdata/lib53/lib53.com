$ ! LIB53.COM - the Phase 53 probe of LIBRTL's instruction routines.
$ ! Written by testdata/lib53/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @LIB53/OUTPUT=LIB53.LOG
$ !
$ ! LIB53.OUT gets the program's output; LIB53.LOG this procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ DEFINE/USER_MODE SYS$OUTPUT LIB53.MLG
$ MACRO/NOLIST LIB53
$ LINK/NOMAP LIB53
$ DEFINE/USER_MODE SYS$OUTPUT LIB53.OUT
$ RUN LIB53
$ SHOW SYMBOL $STATUS
$ ! The shareable images Phases 54, 59, and 61 need the symbols of.
$ ! These are licensed files: they are erased from this volume once
$ ! their symbols are read.
$ IF F$SEARCH("SYS$SHARE:LIBRTL2.EXE") .NES. "" THEN COPY SYS$SHARE:LIBRTL2.EXE []
$ IF F$SEARCH("SYS$SHARE:CRFSHR.EXE") .NES. "" THEN COPY SYS$SHARE:CRFSHR.EXE []
$ IF F$SEARCH("SYS$LIBRARY:CRFSHR.EXE") .NES. "" .AND. F$SEARCH("CRFSHR.EXE") .EQS. "" -
	THEN COPY SYS$LIBRARY:CRFSHR.EXE []
$ DIRECTORY/SIZE/DATE
