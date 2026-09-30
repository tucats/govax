$ ! LINK.COM - the govax Phase 30 subtask 4c link fixtures (docs/PHASE-30.md).
$ !
$ ! Assembles the fixtures (the new ones here and five Phase 27 ones),
$ ! links them in the combinations below with maps, analyzes each image
$ ! with ANALYZE/IMAGE (NAME.ANI), and runs the complete programs.
$ !
$ ! Run it with the volume holding the fixtures as the default directory,
$ ! keeping a log of everything it prints:
$ !
$ !     @LINK/OUTPUT=LINK.LOG
$ !
$ SET NOON
$ SET VERIFY
$ LIST = "DEFS,SHARE1,SHARE2,ADDR,EXTERN,EXPRS,MODES,GENERAL,GLOBALS"
$ I = 0
$ ASM:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO LINKS
$   MACRO/LIST 'F'
$   I = I + 1
$   GOTO ASM
$ LINKS:
$ ! The Phase 27 fixtures with the module that defines their externals.
$ CALL LNK EXTERN  "EXTERN,DEFS"
$ CALL LNK EXPRS   "EXPRS,DEFS"
$ CALL LNK MODES   "MODES,DEFS"
$ CALL LNK GENERAL "GENERAL,DEFS"
$ CALL LNK GLOBALS "GLOBALS,DEFS"
$ ! Undefined symbols: EXTERN alone.
$ CALL LNK EXTERNU "EXTERN"
$ ! A user object library, searched.
$ LIBRARY/CREATE/OBJECT MYLIB DEFS
$ CALL LNK EXTERNL "EXTERN,MYLIB/LIBRARY"
$ ! Psects shared across modules, in both orders.
$ CALL LNK SHARE   "SHARE1,SHARE2"
$ CALL LNK SHARER  "SHARE2,SHARE1"
$ ! .ADDRESS of a shareable image routine.
$ CALL LNK ADDR    "ADDR"
$ ! An options file.
$ CALL LNK PROG    "PROG/OPTIONS"
$ ! The complete programs.
$ RUN SHARE
$ SHOW SYMBOL $STATUS
$ RUN SHARER
$ SHOW SYMBOL $STATUS
$ RUN ADDR
$ SHOW SYMBOL $STATUS
$ RUN PROG
$ SHOW SYMBOL $STATUS
$ DIRECTORY/FULL/OUTPUT=FILES.LST *.OBJ,*.OLB,*.EXE,*.MAP
$ SET NOVERIFY
$ EXIT
$ !
$ ! LNK name files: link files into NAME.EXE with NAME.MAP, then
$ ! ANALYZE/IMAGE into NAME.ANI.
$ LNK: SUBROUTINE
$   LINK/MAP='P1'.MAP/EXECUTABLE='P1'.EXE 'P2'
$   SHOW SYMBOL $STATUS
$   IF F$SEARCH("''P1'.EXE") .NES. "" THEN -
        ANALYZE/IMAGE/OUTPUT='P1'.ANI 'P1'.EXE
$   EXIT
$ ENDSUBROUTINE
