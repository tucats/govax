$ ! DBGCMD.COM - Phase 42's probe (docs/PHASE-42.md, subtask 1): how the
$ ! VMS debugger's run-control commands behave and word their output
$ ! (breakpoints, tracepoints, watchpoints, STEP, EXAMINE and DEPOSIT,
$ ! CALL, exceptions, and its error messages), for govax's debugger to
$ ! be checked against.
$ !
$ ! 1. Builds DBGCMD with /DEBUG, linked /DEBUG, with its listing, map,
$ !    and ANALYZE/IMAGE.
$ ! 2. Runs it under the debugger once for each file of debugger commands
$ !    (*.DBG), logging each session to a file of the same name with
$ !    type DLG.
$ !
$ ! Run it with the exchange volume as the default directory, keeping a
$ ! log of everything it prints:
$ !
$ !     @DBGCMD/OUTPUT=DBGCMD.LOG
$ !
$ SET NOON
$ SET VERIFY
$ !
$ ! 1. The build.
$ !
$ MACRO/DEBUG/LIST DBGCMD
$ LINK/DEBUG/MAP DBGCMD
$ ANALYZE/IMAGE/OUTPUT=DBGCMD.ANI DBGCMD.EXE
$ !
$ ! 2. Debugger sessions. DEBUG writes a file of debugger commands that
$ !    opens P1.DLG as the log, runs the probe's commands (P1.DBG), and
$ !    exits, then runs DBGCMD under the debugger reading it.
$ !
$ CALL DEBUG BREAK
$ CALL DEBUG BRKCLS
$ CALL DEBUG STEP
$ CALL DEBUG WATCH
$ CALL DEBUG TRACE
$ CALL DEBUG EXAM
$ CALL DEBUG CALL
$ CALL DEBUG EXCEPT
$ CALL DEBUG ERRORS
$ DIRECTORY/SIZE=ALL/DATE *.*
$ EXIT
$ !
$ DEBUG: SUBROUTINE
$   OPEN/WRITE DBGF RUN.DBG
$   WRITE DBGF "SET LOG ''P1'.DLG"
$   WRITE DBGF "SET OUTPUT LOG,VERIFY"
$   WRITE DBGF "@''P1'.DBG"
$   WRITE DBGF "EXIT"
$   CLOSE DBGF
$   DEFINE/USER_MODE DBG$INPUT RUN.DBG
$   DEFINE/USER_MODE DBG$DECW$DISPLAY " "
$   RUN/DEBUG DBGCMD
$   SHOW SYMBOL $STATUS
$ ENDSUBROUTINE
