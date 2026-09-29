$ ! ASSEMBLE.COM - assemble the govax Phase 27 fixtures (docs/PHASE-27.md).
$ !
$ ! For each fixture: MACRO/LIST (making NAME.OBJ and NAME.LIS), then
$ ! ANALYZE/OBJECT into NAME.ANL. ENTRY and HELLO are complete programs,
$ ! so they are also linked (with maps) and run. Run it with the volume
$ ! holding the fixtures as the default directory: @ASSEMBLE
$ !
$ SET NOON
$ LIST = "EMPTY,DATA,ENTRY,RELOC,EXTERN,GLOBALS,BRANCH,EXPRS,HELLO"
$ I = 0
$ NEXT:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO LINKS
$   WRITE SYS$OUTPUT "Assembling ''F'"
$   MACRO/LIST 'F'
$   ANALYZE/OBJECT/OUTPUT='F'.ANL 'F'.OBJ
$   I = I + 1
$   GOTO NEXT
$ LINKS:
$ LINK/MAP ENTRY
$ RUN ENTRY
$ LINK/MAP HELLO
$ RUN HELLO
$ DIRECTORY/FULL/OUTPUT=OBJECTS.DIR *.OBJ
$ EXIT
