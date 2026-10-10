$ ! PROBE50W.COM - PROBE50V's nested procedure (section V9): its lines
$ ! are shown, and it turns verification off, with SET NOVERIFY or, given
$ ! QUIET, F$VERIFY in a comment, before it ends. Is the change still in
$ ! effect after it returns?
$ IF P1 .EQS. "QUIET" THEN GOTO QUIET
$ SHOW SYMBOL P1
$ SET NOVERIFY
$ EXIT
$ QUIET:
$ ! 'F$VERIFY(0)'
$ SHOW SYMBOL P1
$ EXIT
