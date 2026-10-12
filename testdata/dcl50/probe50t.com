$ ! PROBE50T.COM - round 5 of Phase 50's probe (testdata/dcl50/README.md):
$ ! what subtasks 10 to 19 chose without VMS. Run from SYSTEM, with the
$ ! exchange volume as the default directory:
$ !
$ !     @PROBE50T/OUTPUT=PROBE50T.LOG
$ !
$ ! Nothing waits. Each section starts by showing a marker symbol M, and
$ ! each lexical case sets X to "-" first, so a failing one shows "-".
$ SET NOON
$ SET VERIFY
$ !
$ ! T1: statuses and ON (DEVIATIONS.md's "[Phase 50] Statuses and ON").
$ M = "T1"
$ SHOW SYMBOL M
$ @PROBE50U EXITOK
$ SHOW SYMBOL $STATUS
$ @PROBE50U ONGOTO
$ SHOW SYMBOL $STATUS
$ @PROBE50U ONSUBST
$ SHOW SYMBOL $STATUS
$ @PROBE50U ONSEV
$ SHOW SYMBOL $STATUS
$ @PROBE50U SKPDAT
$ SHOW SYMBOL $STATUS
$ ON FOO THEN WRITE SYS$OUTPUT "foo"
$ SHOW SYMBOL $STATUS
$ @PROBE50U EXIT34
$ SHOW SYMBOL $STATUS
$ @PROBE50U/OUTPUT=PROBE50N.LOG EXITMSG
$ SHOW SYMBOL $STATUS
$ TYPE PROBE50N.LOG
$ DIRECTORY PROBE50T.COM
$ SHOW SYMBOL $STATUS
$ TYPE PROBE50E.COM
$ SHOW SYMBOL $STATUS
$ X = -7 / 0
$ SHOW SYMBOL X
$ X = "a"
$ WRITE SYS$OUTPUT "unclosed ''X here"
$ !
$ ! T2: labels, IF blocks, GOSUB, and CALL (DEVIATIONS.md's "[Phase 50]
$ ! Labels, IF blocks, GOSUB, and CALL").
$ M = "T2"
$ SHOW SYMBOL M
$ @PROBE50U FWDREC
$ @PROBE50U THENEMPTY
$ SHOW SYMBOL $STATUS
$ @PROBE50U NOTTHEN
$ SHOW SYMBOL $STATUS
$ @PROBE50U FLOWSTATUS
$ @PROBE50U RETURNN
$ SHOW SYMBOL $STATUS
$ @PROBE50U GOSUBMISS
$ SHOW SYMBOL $STATUS
$ @PROBE50U CALLMISS
$ SHOW SYMBOL $STATUS
$ @PROBE50U ABBREV
$ SHOW SYMBOL $STATUS
$ !
$ ! T3: lexical functions (DEVIATIONS.md's "[Phase 50] Lexical
$ ! functions"): keyword abbreviations, ranges, F$TYPE, F$EDIT, F$FAO,
$ ! F$ENVIRONMENT, F$GETJPI's formats, F$SEARCH, F$PARSE, F$CVTIME.
$ M = "T3"
$ SHOW SYMBOL M
$ X = "-"
$ X = F$EDIT("  a   b  ", "COMP")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$EDIT("  Abc  ", "UPCASE,TRIM")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$EDIT("x ""a  b"" y", "COLLAPSE,UPCASE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$EDIT("abc ! comment", "UNCOMMENT")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$EDIT("ABC", "LOWERCASE,UPCASE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$EXTRACT(-1, 2, "abc")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$EXTRACT(1, -1, "abc")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$EXTRACT(5, 2, "abc")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$ELEMENT(-1, ",", "a,b")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$ELEMENT(5, ",", "a,b")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$ELEMENT(0, ",,", "a,b")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$LOCATE("", "abc")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVSI(0, 40, "abcdefgh")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVSI(60, 8, "ab")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVUI(4, 4, "A")
$ SHOW SYMBOL X
$ Y = " 12 "
$ X = "-"
$ X = F$TYPE(Y)
$ SHOW SYMBOL X
$ Y = "%X1F"
$ X = "-"
$ X = F$TYPE(Y)
$ SHOW SYMBOL X
$ Y = "+5"
$ X = "-"
$ X = F$TYPE(Y)
$ SHOW SYMBOL X
$ Y = "1 2"
$ X = "-"
$ X = F$TYPE(Y)
$ SHOW SYMBOL X
$ X = "-"
$ X = F$TYPE("abc")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$FAO("!UL", "12")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$FAO("!AS", 12)
$ SHOW SYMBOL X
$ X = "-"
$ X = F$FAO("!8AS|!3ZL", "ab", 7)
$ SHOW SYMBOL X
$ X = "-"
$ X = F$ENVIRONMENT("DEP")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$ENVIRONMENT("CONTROL")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$ENVIRONMENT("NOCONTROL")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$ENVIRONMENT("ON_SEVERITY")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$ENVIRONMENT("PROTECTION")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$GETSYI("NODEN")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$GETJPI("", "PRCN")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$GETJPI("", "USERNAME")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$GETJPI("", "UIC")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$USER()
$ SHOW SYMBOL X
$ X = "-"
$ X = F$GETJPI("", "MODE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$GETJPI("", "JOBTYPE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$GETJPI("", "STATE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$GETJPI("", "CURPRIV")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$SEARCH("PROBE50T.COM")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$SEARCH("PROBE50T.COM")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$SEARCH("PROBE50T.COM")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$SEARCH("PROBE50T")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$PARSE("NOSUCH:[X]Y.Z")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$PARSE("[.NOSUCHDIR]Y.Z")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$PARSE("[.NOSUCHDIR]Y.Z", , , "NAME", "SYNTAX_ONLY")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$PARSE("A.B", , , "NAM")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$PARSE("A.B;3", "C.D", , "VERSION")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$MESSAGE(%X109110A2)
$ SHOW SYMBOL X
$ X = "-"
$ X = F$MESSAGE(%X00000002)
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00", "COMPARISON")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00", "ABSOLUTE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026", "ABSOLUTE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00", , "DAYOFYEAR")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00", "ABSOLUTE", "DAYOFYEAR")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00", "ABSOLUTE", "WEEKDAY")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00", "ABSOLUTE", "DATE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("1-OCT-2026 14:00", "ABSOLUTE", "DATE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00", "COMPARISON", "MONTH")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00", "ABSOLUTE", "MONTH")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("1-02:03:04.5", "DELTA")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("1 02:03:04.5", "DELTA")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("1-02:03:04.5", "DELTA", "HOUR")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00+1-02:00", "ABSOLUTE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00-0-02:00", "ABSOLUTE")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00", "ABS")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026 14:00", , "WEEK")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("9-OCT-2026:14:30:15.25", "ABSOLUTE")
$ SHOW SYMBOL X
$ !
$ ! T5: a subprocess's DCL (subtask 19): SPAWN of a command, STOP in a
$ ! subprocess's procedure, and EXIT n from one: what each prints, and
$ ! $STATUS after the SPAWN.
$ M = "T5"
$ SHOW SYMBOL M
$ SPAWN WRITE SYS$OUTPUT "T5 depth ", F$ENVIRONMENT("DEPTH")
$ SHOW SYMBOL $STATUS
$ SPAWN @PROBE50U SUBSTOP
$ SHOW SYMBOL $STATUS
$ SPAWN @PROBE50U SUBEXIT
$ SHOW SYMBOL $STATUS
$ SPAWN FROBNICATE
$ SHOW SYMBOL $STATUS
$ M = "END"
$ SHOW SYMBOL M
$ EXIT
