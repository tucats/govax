$ ! PROBE50C.COM - round 6's ELSE before a block's THEN
$ ! (testdata/dcl50/README.md), run by PROBE50A.COM with P1 1 and 0, the
$ ! IF's value. Its own file, so no other case's search passes the block;
$ ! SET NOON, so the lines after an error still run.
$ SET NOON
$ IF P1
$ ELSE
$ WRITE SYS$OUTPUT "ELSEFIRST ''P1': after ELSE"
$ SHOW SYMBOL $STATUS
$ THEN
$ WRITE SYS$OUTPUT "ELSEFIRST ''P1': after THEN"
$ ENDIF
$ WRITE SYS$OUTPUT "ELSEFIRST ''P1': after ENDIF"
$ SHOW SYMBOL $STATUS
$ EXIT
