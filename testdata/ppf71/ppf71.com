$ ! PPF71.COM - Phase 71's probe of process-permanent files
$ ! (testdata/ppf71/README.md). Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @PPF71/OUTPUT=PPF71.LOG
$ !
$ ! PPF71.LOG gets this procedure's log; PPF71S.LOG the nested
$ ! procedure's (@PPF71S/OUTPUT=PPF71S.LOG); PPF71P.LOG the subprocess's;
$ ! PPF71B.DAT the file DCL and the image both write.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ MACRO PPF71
$ LINK PPF71
$ DEFINE PPF71_ALIAS SYS$OUTPUT
$ !
$ ! L1: DCL's view of the process-permanent names, at level 1 under
$ ! /OUTPUT.
$ CALL LEX SYS$INPUT
$ CALL LEX SYS$OUTPUT
$ CALL LEX SYS$ERROR
$ CALL LEX SYS$COMMAND
$ CALL LEX TT
$ CALL LEX PPF71_ALIAS
$ SHOW LOGICAL/FULL SYS$INPUT
$ SHOW LOGICAL/FULL SYS$OUTPUT
$ SHOW LOGICAL/FULL SYS$ERROR
$ SHOW LOGICAL/FULL SYS$COMMAND
$ SHOW TRANSLATION SYS$OUTPUT
$ SHOW LOGICAL/PROCESS
$ WRITE SYS$OUTPUT "parse X.DAT with SYS$OUTPUT: """, F$PARSE("X.DAT","SYS$OUTPUT"), """"
$ WRITE SYS$OUTPUT "parse SYS$OUTPUT:X.DAT: """, F$PARSE("SYS$OUTPUT:X.DAT"), """"
$ WRITE SYS$OUTPUT "search SYS$OUTPUT: """, F$SEARCH("SYS$OUTPUT"), """"
$ WRITE PPF71_ALIAS "L1 written to PPF71_ALIAS"
$ SHOW SYMBOL $STATUS
$ !
$ ! L2: the copied string under another name, by DCL.
$ X = F$TRNLNM("SYS$OUTPUT")
$ DEFINE PPF71_DCLCOPY "''X'"
$ SHOW SYMBOL $STATUS
$ SHOW LOGICAL/FULL PPF71_DCLCOPY
$ CALL LEX PPF71_DCLCOPY
$ WRITE PPF71_DCLCOPY "L2 written to PPF71_DCLCOPY"
$ SHOW SYMBOL $STATUS
$ !
$ ! Part T: the program's translations, and part D, its devices.
$ DEFINE/USER_MODE PPF71_PART T
$ RUN PPF71
$ SHOW SYMBOL $STATUS
$ DEFINE/USER_MODE PPF71_PART D
$ RUN PPF71
$ SHOW SYMBOL $STATUS
$ !
$ ! Part O: SYS$OUTPUT through RMS.
$ DEFINE/USER_MODE PPF71_PART O
$ RUN PPF71
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "after part O"
$ SHOW LOGICAL PPF71_COPY
$ !
$ ! Part I: SYS$INPUT through RMS, from these eight data lines.
$ DEFINE/USER_MODE PPF71_PART I
$ RUN PPF71
data record 1
data record 2
data record 3
data record 4
data record 5
data record 6
data record 7
data record 8
$ SHOW SYMBOL $STATUS
$ !
$ ! Part F: files DCL's OPEN opened, read and written by DCL and the
$ ! image in turn.
$ CREATE PPF71A.DAT
A1 first record
A2 second record
A3 third record
A4 fourth record
A5 fifth record
A6 sixth record
A7 seventh record
A8 eighth record
$ OPEN/READ PPF71_IN PPF71A.DAT
$ READ PPF71_IN REC
$ SHOW SYMBOL REC
$ OPEN/WRITE PPF71_OUT PPF71B.DAT
$ WRITE PPF71_OUT "B1 from DCL"
$ CALL LEX PPF71_IN
$ CALL LEX PPF71_OUT
$ SHOW LOGICAL/FULL PPF71_IN
$ DEFINE/USER_MODE PPF71_PART F
$ RUN PPF71
$ SHOW SYMBOL $STATUS
$ READ PPF71_IN REC
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL REC
$ WRITE PPF71_OUT "B5 from DCL, after the image"
$ SHOW SYMBOL $STATUS
$ !
$ ! The subprocess, while PPF71_IN and PPF71_OUT are open.
$ SPAWN/OUTPUT=PPF71P.LOG/INPUT=NL: @PPF71P
$ SHOW SYMBOL $STATUS
$ READ PPF71_IN REC
$ SHOW SYMBOL REC
$ CLOSE PPF71_IN
$ CLOSE PPF71_OUT
$ TYPE PPF71B.DAT
$ DIRECTORY/FULL PPF71B.DAT
$ !
$ ! A nested procedure with an output file of its own.
$ @PPF71S/OUTPUT=PPF71S.LOG
$ SHOW SYMBOL $STATUS
$ TYPE PPF71S.LOG
$ !
$ ! Part X: SYS$OUTPUT left open at image exit.
$ DEFINE/USER_MODE PPF71_PART X
$ RUN PPF71
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "after part X"
$ SHOW LOGICAL/PROCESS
$ EXIT
$ !
$ ! LEX name: the name's string as F$TRNLNM returns it, at the mode
$ ! that finds it first and at executive mode: its length, its first
$ ! four bytes (two bytes and a word, as ESC/IFI would be), the rest,
$ ! and its attributes; then F$PARSE and F$GETDVI of the name.
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
