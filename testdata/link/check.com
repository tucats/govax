$ ! CHECK.COM - the govax Phase 30 subtask 5 checks (docs/PHASE-30.md).
$ !
$ ! Every GV_*.EXE on this volume was made by govax alone: govax's MACRO
$ ! assembled the fixtures into GV_*.OBJ, and govax's LINK linked them,
$ ! writing the images (and GV_*.MAP) straight onto the volume. This
$ ! analyzes each image with ANALYZE/IMAGE (NAME.ANI), runs the complete
$ ! programs, showing $STATUS, and lists the images' file attributes.
$ !
$ ! GV_HELLONT and GV_SHARENT were linked /NOTRACEBACK. GV_EXTERNU has
$ ! undefined symbols, and GV_EXPRS and GV_GLOBALS no transfer address, so
$ ! they, GV_MODES, and GV_GENERAL are analyzed but not run.
$ !
$ ! Run it with the volume as the default directory, keeping a log:
$ !
$ !     @CHECK/OUTPUT=CHECK.LOG
$ !
$ SET NOON
$ SET VERIFY
$ IMAGES = "GV_HELLO,GV_HELLONT,GV_ENTRY,GV_PSECTS,GV_SHARE,GV_SHARER," + -
           "GV_SHARENT,GV_ADDR,GV_PROG,GV_EXTERN,GV_EXTERNU,GV_EXPRS," + -
           "GV_GLOBALS,GV_MODES,GV_GENERAL"
$ RUNS = "GV_HELLO,GV_HELLONT,GV_ENTRY,GV_PSECTS,GV_SHARE,GV_SHARER," + -
         "GV_SHARENT,GV_ADDR,GV_PROG,GV_EXTERN"
$ I = 0
$ ANALYZE:
$   F = F$ELEMENT(I, ",", IMAGES)
$   IF F .EQS. "," THEN GOTO RUNS
$   ANALYZE/IMAGE/OUTPUT='F'.ANI 'F'.EXE
$   I = I + 1
$   GOTO ANALYZE
$ RUNS:
$ I = 0
$ RUN:
$   F = F$ELEMENT(I, ",", RUNS)
$   IF F .EQS. "," THEN GOTO DONE
$   RUN 'F'
$   SHOW SYMBOL $STATUS
$   I = I + 1
$   GOTO RUN
$ DONE:
$ DIRECTORY/FULL/OUTPUT=IMAGES.LST GV_*.EXE
$ SET NOVERIFY
$ EXIT
