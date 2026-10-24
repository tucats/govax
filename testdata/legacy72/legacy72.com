$ ! LEGACY72.COM - Phase 72's probe of a VMS V1 image (docs/PHASE-72 -
$ ! legacy images.md, subtask 2; testdata/legacy72/README.md): how VMS
$ ! reports, activates, and runs DUNGEON.EXE (image format 02.01, linked
$ ! 7-DEC-1978), and VMSRTL.EXE, the shareable image its global section
$ ! ISDs name.
$ !
$ ! 1. ANALYZE/IMAGE of DUNGEON.EXE and VMSRTL.EXE, and what INSTALL
$ !    knows of VMSRTL and its global sections.
$ ! 2. DUNGEON under the debugger (DUN72.DBG's commands, logged to
$ !    DUN72.DLG): where its images went, the old system service vector,
$ !    and VMSRTL's transfer vector where DUNGEON calls it.
$ ! 3. DUNGEON, and DUNGEON_V552 if it's here, playing a few moves read
$ !    from this procedure.
$ !
$ ! Run it with the exchange volume as the default directory, keeping a
$ ! log of everything it prints:
$ !
$ !     @LEGACY72/OUTPUT=LEGACY72.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DIRECTORY/FULL DUNGEON.EXE
$ !
$ ! 1. The images, as ANALYZE and INSTALL see them.
$ !
$ ANALYZE/IMAGE/OUTPUT=DUNGEON.ANI DUNGEON.EXE
$ ANALYZE/IMAGE/OUTPUT=VMSRTL.ANI SYS$SHARE:VMSRTL.EXE
$ SHOW LOGICAL VMSRTL
$ SHOW LOGICAL SYS$SHARE
$ INSTALL LIST/FULL SYS$SHARE:VMSRTL.EXE
$ INSTALL LIST/GLOBAL/OUTPUT=GLOBAL.LIS
$ SEARCH GLOBAL.LIS VMSRTL,LIBRTL,FORRTL,MTHRTL
$ !
$ ! The game's files, by the names DUNGEON.COM gave them too.
$ !
$ DEFINE DINDX 'F$ENVIRONMENT("DEFAULT")'DINDX.DAT
$ DEFINE DTEXT 'F$ENVIRONMENT("DEFAULT")'DTEXT.DAT
$ !
$ ! 2. The debugger session. DUN72.DBG's commands, with the session
$ !    logged to DUN72.DLG.
$ !
$ OPEN/WRITE DBGF RUN.DBG
$ WRITE DBGF "SET LOG DUN72.DLG"
$ WRITE DBGF "SET OUTPUT LOG,VERIFY"
$ WRITE DBGF "@DUN72.DBG"
$ WRITE DBGF "EXIT"
$ CLOSE DBGF
$ DEFINE/USER_MODE DBG$INPUT RUN.DBG
$ DEFINE/USER_MODE DBG$DECW$DISPLAY " "
$ RUN/DEBUG DUNGEON
$ SHOW SYMBOL $STATUS
$ !
$ ! 3. The game, a few moves from this procedure's data lines.
$ !
$ RUN DUNGEON
LOOK
OPEN MAILBOX
TAKE LEAFLET
READ LEAFLET
INVENTORY
SCORE
QUIT
Y
$ SHOW SYMBOL $STATUS
$ IF F$SEARCH("DUNGEON_V552.EXE") .EQS. "" THEN GOTO DONE
$ RUN DUNGEON_V552
LOOK
OPEN MAILBOX
TAKE LEAFLET
READ LEAFLET
INVENTORY
SCORE
QUIT
Y
$ SHOW SYMBOL $STATUS
$ DONE:
$ DIRECTORY/SIZE=ALL/DATE *.*
$ EXIT
