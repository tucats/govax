$ ! RUN2.COM - Phase 68's second round (testdata/link68/round2/README.md):
$ ! VMS runs and analyzes the images govax's MACRO and LINK made from the
$ ! probe's sources. Run it with the exchange volume's [000000] as the
$ ! default directory:
$ !
$ !     @RUN2/OUTPUT=RUN2.LOG
$ !
$ ! Every image is govax's, as govax's run of LINK68.COM left them (the
$ ! newest version of each). ANALYZE/IMAGE writes NAME.AN2, so VMS's
$ ! analysis of govax's image sits beside govax's own NAME.ANI.
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ HERE = F$ENVIRONMENT("DEFAULT")
$ !
$ ! VMS's ANALYZE/IMAGE of each of govax's images.
$ !
$ LIST = "SHR1,SHR1A,SHR2,SHR3,SHR3P,SHR4,SHR5,SHR6,MAIN1,MAIN1A,MAIN1N," + -
         "MAIN1L,MAIN1U,MAIN2,MAIN3,MAIN3P,MAIN4,MAIN6,EXE1"
$ I = 0
$ ANI:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO RUNS
$   ANALYZE/IMAGE/OUTPUT='F'.AN2 'F'.EXE
$   SHOW SYMBOL $STATUS
$   I = I + 1
$   GOTO ANI
$ RUNS:
$ !
$ ! The runs: each of govax's programs against govax's shareable
$ ! images, found through logical names, as in LINK68.COM. SHR1 is the
$ ! last relink (LEQUAL,1,1000), SHR4 the ALWAYS,7,7 relink, and SHR1A
$ ! the relink MAIN1A doesn't match.
$ !
$ DEFINE SHR1 'HERE'SHR1.EXE
$ DEFINE SHR1A 'HERE'SHR1A.EXE
$ DEFINE SHR2 'HERE'SHR2.EXE
$ DEFINE SHR3 'HERE'SHR3.EXE
$ DEFINE SHR3P 'HERE'SHR3P.EXE
$ DEFINE SHR4 'HERE'SHR4.EXE
$ DEFINE SHR6 'HERE'SHR6.EXE
$ LIST = "MAIN1,MAIN1N,MAIN1L,MAIN1U,MAIN1A,MAIN2,MAIN3,MAIN3P,MAIN4,MAIN6,EXE1"
$ I = 0
$ RUN:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO LINKS
$   RUN 'F'
$   SHOW SYMBOL $STATUS
$   I = I + 1
$   GOTO RUN
$ LINKS:
$ RUN SHR1.EXE
$ SHOW SYMBOL $STATUS
$ !
$ ! VMS's LINK against govax's shareable images: does it read their
$ ! global symbol tables? MAIN1V uses SHR1, MAIN2V uses SHR2 (which uses
$ ! SHR1), each from VMS's MACRO's object, as LINK68.COM made them.
$ !
$ MACRO/NOLIST/OBJECT=MAIN1V.OBJ MAIN1.MAR
$ MACRO/NOLIST/OBJECT=MAIN2V.OBJ MAIN2.MAR
$ LINK/MAP=MAIN1V/FULL MAIN1V.OBJ,SYS$INPUT/OPTIONS
SHR1/SHAREABLE
$ SHOW SYMBOL $STATUS
$ RUN MAIN1V
$ SHOW SYMBOL $STATUS
$ LINK/MAP=MAIN2V/FULL MAIN2V.OBJ,SYS$INPUT/OPTIONS
SHR2/SHAREABLE
$ SHOW SYMBOL $STATUS
$ RUN MAIN2V
$ SHOW SYMBOL $STATUS
$ !
$ ! VMS's librarian on govax's images, and VMS's LINK through the
$ ! library govax made.
$ !
$ LIBRARY/CREATE/SHARE MYSHRV SHR1.EXE,SHR2.EXE
$ SHOW SYMBOL $STATUS
$ LIBRARY/LIST=MYSHRV.LIS/FULL/NAMES/SHARE MYSHRV
$ LIBRARY/LIST=MYSHRG.LIS/FULL/NAMES/SHARE MYSHR
$ LINK/MAP=MAIN1W/FULL MAIN1V.OBJ,MYSHR/LIBRARY
$ SHOW SYMBOL $STATUS
$ RUN MAIN1W
$ SHOW SYMBOL $STATUS
$ !
$ DEASSIGN SHR1
$ DEASSIGN SHR1A
$ DEASSIGN SHR2
$ DEASSIGN SHR3
$ DEASSIGN SHR3P
$ DEASSIGN SHR4
$ DEASSIGN SHR6
$ DIRECTORY/SIZE=ALL/DATE *.EXE,*.AN2,*.OLB
$ SET NOVERIFY
$ EXIT
