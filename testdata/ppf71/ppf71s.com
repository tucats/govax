$ ! PPF71S.COM - Phase 71's probe, a nested procedure with an output file
$ ! of its own (@PPF71S/OUTPUT=PPF71S.LOG, from PPF71.COM): SYS$OUTPUT is
$ ! this procedure's log, and the program writes into it.
$ SET NOON
$ SET VERIFY
$ CALL LEX SYS$INPUT
$ CALL LEX SYS$OUTPUT
$ SHOW LOGICAL/FULL SYS$OUTPUT
$ DEFINE/USER_MODE PPF71_PART T
$ RUN PPF71
$ DEFINE/USER_MODE PPF71_PART O
$ RUN PPF71
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "PPF71S: after part O"
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
