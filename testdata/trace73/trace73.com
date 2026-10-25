$ ! TRACE73.COM - Phase 73's probe of the trace trap
$ ! (testdata/trace73/README.md). Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @TRACE73/OUTPUT=TRACE73.LOG
$ !
$ ! TRACE73.LOG gets everything: the builds, each part's run with
$ ! $STATUS after it, and the debugger sessions (each also logged to a
$ ! .DLG of its own). P1, if given, is the list of parts to run instead
$ ! of all of them (VMS's run gives none).
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ !
$ ! The program three ways: with traceback (LINK's default),
$ ! /NOTRACEBACK, and /DEBUG.
$ !
$ MACRO TRACE73
$ MACRO/DEBUG/OBJECT=TRACE73D TRACE73
$ LINK/MAP=TRACE73/FULL TRACE73
$ SHOW SYMBOL $STATUS
$ LINK/MAP=TRACE73NT/FULL/NOTRACEBACK/EXECUTABLE=TRACE73NT TRACE73
$ SHOW SYMBOL $STATUS
$ LINK/MAP=TRACE73D/FULL/DEBUG/EXECUTABLE=TRACE73D TRACE73D
$ SHOW SYMBOL $STATUS
$ !
$ ! Every part, with traceback.
$ !
$ LIST = "A,B,C,D,E,F,G,H,I,J,K,N"
$ IF P1 .NES. "" THEN LIST = P1
$ I = 0
$ LOOP:
$   P = F$ELEMENT(I, ",", LIST)
$   IF P .EQS. "," THEN GOTO DONE
$   DEFINE TRACE73_PART 'P'
$   RUN TRACE73
$   SHOW SYMBOL $STATUS
$   I = I + 1
$   GOTO LOOP
$ DONE:
$ !
$ ! The unhandled SS$_TBIT again: without traceback (the "improperly
$ ! handled condition" dump), and with /DEBUG's line numbers but not
$ ! under the debugger.
$ !
$ DEFINE TRACE73_PART J
$ RUN TRACE73NT
$ SHOW SYMBOL $STATUS
$ RUN/NODEBUG TRACE73D
$ SHOW SYMBOL $STATUS
$ !
$ ! The debugger beside a traced program: each session runs one part
$ ! under the debugger, reading NAME.DBG's commands and logging the
$ ! session to NAME.DLG.
$ !
$ CALL DEBUG TRSTEP A
$ CALL DEBUG TRGO A
$ CALL DEBUG TREXC A
$ CALL DEBUG TRHAND A
$ CALL DEBUG TRCALL C
$ CALL DEBUG TRSVC E
$ CALL DEBUG TRDEP N
$ CALL DEBUG TRDEPGO N
$ CALL DEBUG TRUNH J
$ CALL DEBUG TRPRIV N
$ DIRECTORY/SIZE/DATE
$ EXIT
$ !
$ DEBUG: SUBROUTINE
$   DEFINE TRACE73_PART 'P2'
$   OPEN/WRITE DBGF RUN.DBG
$   WRITE DBGF "SET LOG ''P1'.DLG"
$   WRITE DBGF "SET OUTPUT LOG,VERIFY"
$   WRITE DBGF "@''P1'.DBG"
$   WRITE DBGF "EXIT"
$   CLOSE DBGF
$   DEFINE/USER_MODE DBG$INPUT RUN.DBG
$   DEFINE/USER_MODE DBG$DECW$DISPLAY " "
$   RUN/DEBUG TRACE73D
$   SHOW SYMBOL $STATUS
$ ENDSUBROUTINE
