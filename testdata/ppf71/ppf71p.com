$ ! PPF71P.COM - Phase 71's probe, run in a subprocess
$ ! (SPAWN/OUTPUT=PPF71P.LOG/INPUT=NL: @PPF71P, from PPF71.COM, while
$ ! PPF71_IN and PPF71_OUT are open in the parent): the subprocess's own
$ ! process-permanent names, and whether the parent's OPEN names came
$ ! with it.
$ SET NOON
$ SET VERIFY
$ CALL LEX SYS$INPUT
$ CALL LEX SYS$OUTPUT
$ CALL LEX SYS$ERROR
$ CALL LEX SYS$COMMAND
$ CALL LEX PPF71_IN
$ SHOW LOGICAL/PROCESS
$ DEFINE/USER_MODE PPF71_PART T
$ RUN PPF71
$ DEFINE/USER_MODE PPF71_PART D
$ RUN PPF71
$ READ PPF71_IN REC
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "PPF71P: done"
$ EXIT
$ !
$ ! LEX name: as PPF71.COM's.
$ LEX: SUBROUTINE
$   X = F$TRNLNM(P1)
$   GOSUB BYTES
$   WRITE SYS$OUTPUT "  access mode ", F$TRNLNM(P1,,,,,"ACCESS_MODE"), -
        ", terminal ", F$TRNLNM(P1,,,,,"TERMINAL"), -
        ", confine ", F$TRNLNM(P1,,,,,"CONFINE"), -
        ", length ", F$TRNLNM(P1,,,,,"LENGTH"), -
        ", table ", F$TRNLNM(P1,,,,,"TABLE_NAME")
$   X = F$TRNLNM(P1,,,"EXECUTIVE")
$   WRITE SYS$OUTPUT "  executive mode:"
$   GOSUB BYTES
$   WRITE SYS$OUTPUT "  parse """, F$PARSE(P1), """"
$   WRITE SYS$OUTPUT "  devnam """, F$GETDVI(P1,"DEVNAM"), """"
$   EXIT
$ BYTES:
$   L = F$LENGTH(X)
$   IF L .LT. 4
$   THEN
$     WRITE SYS$OUTPUT P1, ": length ", L, " """, X, """"
$   ELSE
$     WRITE SYS$OUTPUT P1, ": length ", L, " bytes ", F$CVUI(0,8,X), " ", -
          F$CVUI(8,8,X), " ", F$CVUI(16,16,X), " after 4 """, -
          F$EXTRACT(4,L-4,X), """"
$   ENDIF
$   RETURN
$ ENDSUBROUTINE
