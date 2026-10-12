$ ! PROBE50Z.COM - round 5's nested procedure for PROBE50Y.COM, one case
$ ! per P1 (testdata/dcl50/README.md, "Round 5").
$ GOTO 'P1'
$ !
$ ! Y1: STOP.
$ STOP:
$ WRITE SYS$OUTPUT "Y1: inner"
$ STOP
$ WRITE SYS$OUTPUT "Y1: inner after"
$ EXIT
$ !
$ ! Y2: ON ERROR THEN STOP, and an error.
$ ONSTOP:
$ ON ERROR THEN STOP
$ OPEN/READ F NOSUCH.DAT
$ WRITE SYS$OUTPUT "Y2: inner after"
$ EXIT
$ !
$ ! Y10, Y11: a wait with no ON CONTROL_Y.
$ WAIT:
$ WRITE SYS$OUTPUT "Press Ctrl/Y now (in the nested procedure)"
$ WAIT 00:00:20
$ WRITE SYS$OUTPUT "inner: after the wait"
$ EXIT
