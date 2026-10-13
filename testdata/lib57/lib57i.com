$ ! LIB57I.COM - the Phase 57 probe's interactive cases, one per P1
$ ! (testdata/lib57/README.md). Run @LIB57 first: it defines LIB57.
$ SET NOON
$ SET VERIFY
$ GOTO 'P1'
$ PAUSE:
$ X = "in LIB57I"
$ LIB57 PAUSE
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "LIB57I goes on"
$ EXIT
$ GETCMD:
$ LIB57 GETCMD
a data line
$ SHOW SYMBOL $STATUS
$ EXIT
$ CTLY:
$ ON CONTROL_Y THEN GOTO CAUGHT
$ LIB57 CTLDY
$ WRITE SYS$OUTPUT "Press Ctrl/Y now (10 seconds)"
$ WAIT 00:00:10
$ LIB57 CTLEY
$ WRITE SYS$OUTPUT "Press Ctrl/Y now (10 seconds)"
$ WAIT 00:00:10
$ WRITE SYS$OUTPUT "no Ctrl/Y"
$ EXIT
$ CAUGHT:
$ WRITE SYS$OUTPUT "ON CONTROL_Y caught it"
$ LIB57 CTLEY
$ EXIT
