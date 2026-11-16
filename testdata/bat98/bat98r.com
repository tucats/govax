$ ! BAT98R.COM - Phase 98's probe: a job that can be restarted. Written
$ ! by testdata/bat98/gen.go. Its first step says where a restart is to
$ ! go on from (SET RESTART_VALUE), writes BAT98$DISK:[BAT98SCR]P1.RUN, and
$ ! waits a minute at most; BAT98.COM requeues it meanwhile. A restart
$ ! goes on from the second step.
$ SET NOON
$ WRITE SYS$OUTPUT "BAT98R ", P1, " STARTED"
$ SHOW SYMBOL $RESTART
$ SHOW SYMBOL BATCH$RESTART
$ IF F$TYPE(BATCH$RESTART) .EQS. "" THEN GOTO STEP1
$ IF BATCH$RESTART .NES. "" THEN GOTO 'BATCH$RESTART'
$ STEP1:
$ WRITE SYS$OUTPUT "BAT98R AT STEP1"
$ SET RESTART_VALUE=STEP2
$ SHOW SYMBOL BATCH$RESTART
$ OPEN/WRITE MARK BAT98$DISK:[BAT98SCR]'P1'.RUN
$ WRITE MARK F$TIME()
$ CLOSE MARK
$ N = 0
$ LOOP:
$ IF N .GE. 60 THEN GOTO STEP2
$ WAIT 0:0:1
$ N = N + 1
$ GOTO LOOP
$ STEP2:
$ WRITE SYS$OUTPUT "BAT98R AT STEP2"
$ EXIT
