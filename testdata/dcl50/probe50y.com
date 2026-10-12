$ ! PROBE50Y.COM - round 5's interactive part: STOP and Ctrl/Y in command
$ ! procedures (testdata/dcl50/README.md, "Round 5"). Run each case at the
$ ! terminal, WITHOUT /OUTPUT, from SYSTEM, with the exchange volume as
$ ! the default directory, as the README lists them:
$ !
$ !     @PROBE50Y case
$ !     SHOW SYMBOL $STATUS
$ !
$ ! and do what the comment shown before each wait asks for. Keep the
$ ! whole session for vax/probe50y.log.
$ SET VERIFY
$ X = "y-level"
$ IF F$SEARCH("PROBE50X.EXE") .NES. "" THEN GOTO 'P1'
$ MACRO PROBE50X
$ LINK PROBE50X
$ GOTO 'P1'
$ !
$ ! Y1: STOP in a nested procedure; the outer one has an ON action.
$ STOP:
$ ON WARNING THEN WRITE SYS$OUTPUT "Y1: outer action"
$ @PROBE50Z STOP
$ WRITE SYS$OUTPUT "Y1: outer after"
$ EXIT
$ !
$ ! Y2: ON ERROR THEN STOP in a nested procedure.
$ ONSTOP:
$ @PROBE50Z ONSTOP
$ WRITE SYS$OUTPUT "Y2: outer after"
$ EXIT
$ !
$ ! Y3 to Y7, and Y15: Ctrl/Y (Ctrl/C for Y15) at command level 1, with
$ ! no ON CONTROL_Y.
$ WAIT:
$ WRITE SYS$OUTPUT "Press Ctrl/Y now, then type what the README says"
$ WAIT 00:00:20
$ WRITE SYS$OUTPUT "after the wait: X is ''X', $STATUS ''$STATUS'"
$ EXIT
$ !
$ ! Y8: ON CONTROL_Y THEN GOTO a cleanup, which waits again: is the
$ ! action still in effect for the second Ctrl/Y?
$ ONGOTO:
$ N = 0
$ ON CONTROL_Y THEN GOTO ONGOTO_CLEAN
$ WRITE SYS$OUTPUT "Press Ctrl/Y now"
$ WAIT 00:00:20
$ WRITE SYS$OUTPUT "Y8: no Ctrl/Y"
$ EXIT
$ ONGOTO_CLEAN:
$ N = N + 1
$ WRITE SYS$OUTPUT "Y8: cleanup ''N', $STATUS ''$STATUS', on_control_y ''F$ENVIRONMENT("ON_CONTROL_Y")'"
$ IF N .GE. 2 THEN EXIT
$ WRITE SYS$OUTPUT "Press Ctrl/Y again"
$ WAIT 00:00:20
$ WRITE SYS$OUTPUT "Y8: no second Ctrl/Y"
$ EXIT
$ !
$ ! Y9: ON CONTROL_Y THEN a command after which the procedure goes on.
$ ONCONT:
$ ON CONTROL_Y THEN WRITE SYS$OUTPUT "Y9: interrupted"
$ WRITE SYS$OUTPUT "Press Ctrl/Y now"
$ WAIT 00:00:20
$ WRITE SYS$OUTPUT "Y9: after the wait, $STATUS ''$STATUS'"
$ EXIT
$ !
$ ! Y10: Ctrl/Y in a nested procedure with no ON CONTROL_Y, the outer one
$ ! having one: does the inner one exit and the outer go on, does the
$ ! outer's action run, or does the Ctrl/Y prompt appear?
$ NESTED:
$ ON CONTROL_Y THEN GOTO NESTED_CLEAN
$ @PROBE50Z WAIT
$ WRITE SYS$OUTPUT "Y10: outer goes on, $STATUS ''$STATUS'"
$ EXIT
$ NESTED_CLEAN:
$ WRITE SYS$OUTPUT "Y10: outer's cleanup"
$ EXIT
$ !
$ ! Y11: the same with no ON CONTROL_Y in either.
$ NESTED0:
$ @PROBE50Z WAIT
$ WRITE SYS$OUTPUT "Y11: outer goes on, $STATUS ''$STATUS'"
$ EXIT
$ !
$ ! Y12: SET NOCONTROL=Y: Ctrl/Y does nothing; the settings as
$ ! F$ENVIRONMENT reports them.
$ NOCONTROL:
$ ON CONTROL_Y THEN WRITE SYS$OUTPUT "Y12: interrupted"
$ SET NOCONTROL=Y
$ WRITE SYS$OUTPUT "Y12: ", F$ENVIRONMENT("CONTROL"), "/", F$ENVIRONMENT("NOCONTROL"), " ", F$ENVIRONMENT("ON_CONTROL_Y")
$ WRITE SYS$OUTPUT "Press Ctrl/Y now (it should do nothing)"
$ WAIT 00:00:10
$ SET CONTROL=(T,Y)
$ WRITE SYS$OUTPUT "Y12: ", F$ENVIRONMENT("CONTROL"), "/", F$ENVIRONMENT("NOCONTROL"), " ", F$ENVIRONMENT("ON_CONTROL_Y")
$ SET NOCONTROL=T
$ WRITE SYS$OUTPUT "Y12: ", F$ENVIRONMENT("CONTROL"), "/", F$ENVIRONMENT("NOCONTROL")
$ SET CONTROL=X
$ SHOW SYMBOL $STATUS
$ SET CONTROL
$ EXIT
$ !
$ ! Y13: Ctrl/Y while an image runs (PROBE50X counts for a while, then
$ ! writes "spun"), at level 1 with no ON CONTROL_Y: CONTINUE resumes it?
$ IMAGE:
$ WRITE SYS$OUTPUT "Press Ctrl/Y while it runs"
$ RUN PROBE50X
$ WRITE SYS$OUTPUT "Y13: after the image, $STATUS ''$STATUS'"
$ EXIT
$ !
$ ! Y14: Ctrl/Y while an image runs, with ON CONTROL_Y: is the image
$ ! ended, and does the procedure go on after RUN?
$ IMAGEON:
$ ON CONTROL_Y THEN WRITE SYS$OUTPUT "Y14: interrupted, $STATUS ''$STATUS'"
$ WRITE SYS$OUTPUT "Press Ctrl/Y while it runs"
$ RUN PROBE50X
$ WRITE SYS$OUTPUT "Y14: after the image, $STATUS ''$STATUS'"
$ EXIT
