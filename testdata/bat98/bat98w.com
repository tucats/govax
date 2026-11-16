$ ! BAT98W.COM - Phase 98's probe: a job that runs for a while. Written
$ ! by testdata/bat98/gen.go. P1 is its name: it writes
$ ! BAT98$DISK:[BAT98SCR]P1.RUN when it starts, which BAT98.COM waits for.
$ ! P2 is how many seconds it runs at most; then it ends by itself.
$ SET NOON
$ OPEN/WRITE MARK BAT98$DISK:[BAT98SCR]'P1'.RUN
$ WRITE MARK F$TIME()
$ CLOSE MARK
$ WRITE SYS$OUTPUT "BAT98W ", P1, " STARTED"
$ N = 0
$ LOOP:
$ IF N .GE. F$INTEGER(P2) THEN GOTO DONE
$ WAIT 0:0:1
$ N = N + 1
$ GOTO LOOP
$ DONE:
$ WRITE SYS$OUTPUT "BAT98W ", P1, " DONE"
$ EXIT
