$ ! DST.COM - Phase 29's second debugger-record probe (docs/PHASE-29.md,
$ ! subtask 12): real MACRO's DBG records for sources written to settle
$ ! the symbol records and the line-number tables. Each source is
$ ! assembled /DEBUG/LIST (DSTDBG without /DEBUG), and each object
$ ! analyzed. The directory listings give the sources' dates and sizes,
$ ! which the objects' source-file records hold.
$ !
$ ! Run it with the exchange volume as the default directory, keeping a
$ ! log of everything it prints:
$ !
$ !     @DST/OUTPUT=DST.LOG
$ !
$ SET NOON
$ SET VERIFY
$ !
$ ! DSTVAR is DSTLN1 with variable-length records, for the source-file
$ ! record's format fields.
$ !
$ CREATE VAR.FDL
RECORD
	CARRIAGE_CONTROL	carriage_return
	FORMAT			variable
$ CONVERT/FDL=VAR.FDL DSTLN1.MAR DSTVAR.MAR
$ DIRECTORY/FULL *.MAR
$ !
$ LIST = "DSTSYM,DSTLN1,DSTLN2,DSTLN3,DSTLN4,DSTLN5,DSTLN6,DSTDIS,DSTVAR"
$ I = 0
$ NEXT:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO NODEBUG
$   MACRO/DEBUG/LIST 'F'
$   SHOW SYMBOL $STATUS
$   ANALYZE/OBJECT/OUTPUT='F'.ANL 'F'.OBJ
$   I = I + 1
$   GOTO NEXT
$ !
$ ! DSTDBG turns debugger records on partway, with .ENABLE DEBUG.
$ !
$ NODEBUG:
$ MACRO/LIST DSTDBG
$ SHOW SYMBOL $STATUS
$ ANALYZE/OBJECT/OUTPUT=DSTDBG.ANL DSTDBG.OBJ
$ DIRECTORY/SIZE/DATE *.*
$ EXIT
