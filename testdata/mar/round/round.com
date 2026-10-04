$ ! ROUND.COM - Phase 29's VMS round (docs/PHASE-29.md, subtask 14): what
$ ! VMS makes of govax's objects and images.
$ !
$ ! 1. ANALYZE/OBJECT of each object govax assembled (GV*.OBJ), which
$ !    should find no errors.
$ ! 2. ANALYZE/IMAGE of each image govax linked (GV*.EXE), and of the
$ !    images real LINK makes from govax's objects (RL*.EXE).
$ ! 3. The programs run, each linked by govax and by real LINK, so the
$ !    log keeps the traceback VMS prints for each; the probe's list.log
$ !    has what it printed for real MACRO's objects.
$ ! 4. Real MACRO's NOTITLE2 and NOTITLE3, for the title header record a
$ !    module with no .TITLE gets.
$ !
$ ! Run it with the exchange volume as the default directory, keeping a
$ ! log of everything it prints:
$ !
$ !     @ROUND/OUTPUT=ROUND.LOG
$ !
$ SET NOON
$ SET VERIFY
$ !
$ ! 1. govax's objects.
$ !
$ LIST = "GVLCTL,GVBINARY,GVSYMTAB,GVNOTITLE,GVNOTITLE2,GVNOTITLE3," + -
    "GVXREF,GVTRACE,GVFAILMAIN,GVFAILSUB,GVFAILSIG"
$ I = 0
$ ONEXT:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO LINKS
$   ANALYZE/OBJECT/OUTPUT='F'.ANL 'F'.OBJ
$   SHOW SYMBOL $STATUS
$   I = I + 1
$   GOTO ONEXT
$ !
$ ! 2. Real LINK of govax's objects, as govax's own links (EXCHANGE.CMD).
$ !
$ LINKS:
$ LINK/MAP=RLTRACE/EXECUTABLE=RLTRACE GVTRACE
$ LINK/MAP=RLTRNOTB/EXECUTABLE=RLTRNOTB/NOTRACEBACK GVTRACE
$ LINK/MAP=RLFAIL/EXECUTABLE=RLFAIL GVFAILMAIN,GVFAILSUB
$ LINK/MAP=RLFAILNT/EXECUTABLE=RLFAILNT/NOTRACEBACK GVFAILMAIN,GVFAILSUB
$ LINK/MAP=RLFSIG/EXECUTABLE=RLFSIG GVFAILSIG
$ LINK/MAP=RLFSIGNT/EXECUTABLE=RLFSIGNT/NOTRACEBACK GVFAILSIG
$ LIST = "GVTRACE,GVTRNOTB,GVFAIL,GVFAILNT,GVFSIG,GVFSIGNT," + -
    "RLTRACE,RLTRNOTB,RLFAIL,RLFAILNT,RLFSIG,RLFSIGNT"
$ I = 0
$ INEXT:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO RUNS
$   ANALYZE/IMAGE/OUTPUT='F'.ANI 'F'.EXE
$   SHOW SYMBOL $STATUS
$   I = I + 1
$   GOTO INEXT
$ !
$ ! 3. The programs, each linked by govax (GV) and by real LINK (RL).
$ !
$ RUNS:
$ I = 0
$ RNEXT:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO TITLES
$   RUN 'F'
$   SHOW SYMBOL $STATUS
$   I = I + 1
$   GOTO RNEXT
$ !
$ ! 4. Real MACRO's title headers.
$ !
$ TITLES:
$ MACRO/LIST NOTITLE2
$ ANALYZE/OBJECT/OUTPUT=NOTITLE2.ANL NOTITLE2.OBJ
$ MACRO/LIST NOTITLE3
$ ANALYZE/OBJECT/OUTPUT=NOTITLE3.ANL NOTITLE3.OBJ
$ DIRECTORY/SIZE=ALL/DATE
$ SET NOVERIFY
$ EXIT
