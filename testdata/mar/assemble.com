$ ! ASSEMBLE.COM - assemble the govax Phase 27 fixtures (docs/PHASE-27.md).
$ !
$ ! For each fixture: MACRO/LIST (making NAME.OBJ and NAME.LIS), then
$ ! ANALYZE/OBJECT into NAME.ANL. ENTRY, HELLO, and PSECTS are complete
$ ! programs, so they are also linked (with maps) and run.
$ !
$ ! GV_NAME.OBJ, when present, is govax's own object for NAME.MAR, written
$ ! by govax's MACRO command. Each is checked with ANALYZE/OBJECT into
$ ! GV_NAME.ANL, and the complete programs are linked and run too.
$ !
$ ! Run it with the volume holding the fixtures as the default directory,
$ ! keeping a log of everything it prints:
$ !
$ !     @ASSEMBLE/OUTPUT=ASSEMBLE.LOG
$ !
$ SET NOON
$ SET VERIFY
$ LIST = "EMPTY,DATA,ENTRY,RELOC,EXTERN,GLOBALS,BRANCH,EXPRS,HELLO," + -
         "MODES,PSECTS,GENERAL"
$ PROGRAMS = "ENTRY,HELLO,PSECTS"
$ I = 0
$ NEXT:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO GOVAX
$   MACRO/LIST 'F'
$   ANALYZE/OBJECT/OUTPUT='F'.ANL 'F'.OBJ
$   IF F$SEARCH("GV_''F'.OBJ") .NES. "" THEN -
        ANALYZE/OBJECT/OUTPUT=GV_'F'.ANL GV_'F'.OBJ
$   I = I + 1
$   GOTO NEXT
$ GOVAX:
$ I = 0
$ LINKS:
$   F = F$ELEMENT(I, ",", PROGRAMS)
$   IF F .EQS. "," THEN GOTO DONE
$   LINK/MAP 'F'
$   RUN 'F'
$   SHOW SYMBOL $STATUS
$   IF F$SEARCH("GV_''F'.OBJ") .EQS. "" THEN GOTO NEXTLINK
$   LINK/MAP GV_'F'
$   RUN GV_'F'
$   SHOW SYMBOL $STATUS
$ NEXTLINK:
$   I = I + 1
$   GOTO LINKS
$ DONE:
$ DIRECTORY/FULL/OUTPUT=OBJECTS.LST *.OBJ
$ SET NOVERIFY
$ EXIT
