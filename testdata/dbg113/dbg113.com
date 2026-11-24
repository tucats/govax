$ ! DBG113.COM - Phase 113's debugger round (docs/planning/PHASE-113 - Fortran
$ ! errata.md, subtask 12c; README.md): what Phases 42, 85, and 89 left
$ ! unconfirmed. testdata/err113's EXCHANGE.CMD put the source, the command
$ ! files, and govax's own image GVDBG113.EXE in [DBG113] on the round's volume.
$ !
$ ! Run it with that directory as the default:
$ !
$ !     @DBG113/OUTPUT=DBG113.LOG
$ !
$ SET NOON
$ SET VERIFY
$ !
$ ! 1. The image.
$ !
$ MACRO/DEBUG/LIST DBG113
$ LINK/DEBUG/MAP DBG113
$ !
$ ! 2. Debugger sessions: image, command file, log. The program's own output
$ ! (DBG113: done, the exit handler's line) comes here, between them.
$ !
$ CALL DEBUG DBG113 DBG113S DBG113S
$ CALL DEBUG DBG113 DBG113A DBG113A
$ CALL DEBUG DBG113 DBG113I DBG113I
$ CALL DEBUG DBG113 DBG113T DBG113T
$ CALL DEBUG DBG113 DBG113R DBG113R
$ CALL DEBUG DBG113 DBG113N DBG113N
$ CALL DEBUG DBG113 DBG113O DBG113O
$ CALL DEBUG DBG113 DBG113L DBG113L
$ CALL DEBUG DBG113 DBG113X DBG113X
$ CALL DEBUG DBG113 DBG113M DBG113M
$ CALL DEBUG DBG113 DBG113Q DBG113Q
$ !
$ ! 3. govax's image of the same source: govax's MACRO named the source
$ ! DUA5:[DBG113]DBG113.MAR, where EXCHANGE.CMD had the volume, so DUA5 is made
$ ! a name for this device while VMS's debugger reads it.
$ !
$ DEFINE/NOLOG DUA5 'F$PARSE("[]",,,"DEVICE")'
$ CALL DEBUG GVDBG113 DBG113S GVDBG113S
$ DEASSIGN DUA5
$ DIRECTORY/SIZE=ALL/DATE *.*
$ EXIT
$ !
$ DEBUG: SUBROUTINE
$   OPEN/WRITE DBGF RUN.DBG
$   WRITE DBGF "SET LOG ''P3'.DLG"
$   WRITE DBGF "SET OUTPUT LOG,VERIFY"
$   WRITE DBGF "@''P2'.DBG"
$   WRITE DBGF "EXIT"
$   CLOSE DBGF
$   DEFINE/USER_MODE DBG$INPUT RUN.DBG
$   DEFINE/USER_MODE DBG$DECW$DISPLAY " "
$   RUN/DEBUG 'P1'
$   SHOW SYMBOL $STATUS
$ ENDSUBROUTINE
