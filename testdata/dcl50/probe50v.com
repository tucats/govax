$ ! PROBE50V.COM - Phase 50's second probe: verification (SET VERIFY,
$ ! SET PREFIX, F$VERIFY), for subtask 15 (testdata/dcl50/README.md).
$ ! Run it from SYSTEM with the exchange volume as the default directory:
$ !
$ !     @PROBE50V/OUTPUT=PROBE50V.LOG
$ !
$ ! Each section starts with a SHOW SYMBOL of a marker, so the log can be
$ ! split into sections even where verification is off.
$ SET NOON
$ NAME = "MYFILE"
$ X = 1
$ Y = 2
$ SET VERIFY
$ !
$ ! V1: continued lines, with and without substitution.
$ M = "V1"
$ SHOW SYMBOL M
$ A = "one" + -
  " two"
$ A := 'NAME' -
  .DAT
$ A = "''NAME'" + -
  ".DAT"
$ SHOW SYMBOL A
$ !
$ ! V2: indentation, labels, blank and comment lines.
$ M = "V2"
$ SHOW SYMBOL M
$    SHOW SYMBOL X
$LBL1: SHOW SYMBOL X
$   LBL2:   SHOW SYMBOL X
$ LBL3:
$
$!no blank after the dollar
$   ! an indented comment
$ !
$ ! V3: IF blocks: the lines of the branch that doesn't run, and its
$ ! ELSE and ENDIF.
$ M = "V3"
$ SHOW SYMBOL M
$ IF X .EQ. 0
$ THEN
$   SHOW SYMBOL X
$   ! a comment in the branch that doesn't run
$ ELSE
$   SHOW SYMBOL Y
$ ENDIF
$ IF X .EQ. 1
$ THEN
$   SHOW SYMBOL X
$ ELSE
$   SHOW SYMBOL Y
$   ! a comment in the branch that doesn't run
$ ENDIF
$ IF X .EQ. 1 THEN SHOW SYMBOL 'NAME'
$ !
$ ! V4: GOTO, GOSUB, and CALL: the lines passed over, SUBROUTINE,
$ ! ENDSUBROUTINE, and RETURN.
$ M = "V4"
$ SHOW SYMBOL M
$ GOTO V4A
$ SHOW SYMBOL X
$ V4A:
$ GOSUB V4G
$ CALL V4S
$ GOTO V4END
$ V4G:
$   SHOW SYMBOL Y
$ RETURN
$ V4S: SUBROUTINE
$   SHOW SYMBOL X
$ ENDSUBROUTINE
$ V4END:
$ !
$ ! V5: F$VERIFY in comments and in commands.
$ M = "V5"
$ SHOW SYMBOL M
$ ! 'F$VERIFY(0)' - this line, and the next, should not be shown
$ SHOW SYMBOL X
$ ! 'F$VERIFY(1)' - this one should be
$ SHOW SYMBOL Y
$ X = 1 ! 'F$VERIFY(0)' in a command's comment
$ SHOW SYMBOL X
$ X = 1 ! 'F$VER(1)' abbreviated
$ V = 'F$VERIFY(0)'
$ SHOW SYMBOL V
$ V = F$VERIFY(1)
$ SHOW SYMBOL V
$ ! 'F$VERIFY(0) without its closing apostrophe
$ SHOW SYMBOL X
$ V = F$VERIFY(1)
$ ! 'X' 'F$VERIFY(0)' after another apostrophe pair
$ SHOW SYMBOL Y
$ V = F$VERIFY(1)
$ !
$ ! V6: data lines, read by an image (TYPE SYS$INPUT) and not read.
$ M = "V6"
$ SHOW SYMBOL M
$ TYPE SYS$INPUT
data line one
  data line two, indented
$ SET VERIFY=(PROCEDURE,NOIMAGE)
$ TYPE SYS$INPUT
data line three (image verification off)
$ SET VERIFY=(NOPROCEDURE,IMAGE)
$ TYPE SYS$INPUT
data line four (procedure verification off)
$ SET VERIFY
data line five, which nothing reads
$ SHOW SYMBOL M
$ !
$ ! V7: SET PREFIX.
$ M = "V7"
$ SHOW SYMBOL M
$ SET PREFIX "(!5%T) "
$ SHOW SYMBOL X
$ ! a comment with a prefix
$ A = "one" + -
  " two"
$ TYPE SYS$INPUT
data line with a prefix set
$ P = F$ENVIRONMENT("VERIFY_PREFIX")
$ SHOW SYMBOL P
$ SET PREFIX "[!UL|!AS|!XL] "
$ SHOW SYMBOL X
$ SET PREFIX "!%D !"
$ SHOW SYMBOL X
$ SET NOPREFIX
$ P = F$ENVIRONMENT("VERIFY_PREFIX")
$ SHOW SYMBOL P
$ !
$ ! V8: ON's action, a failing command, and messages.
$ M = "V8"
$ SHOW SYMBOL M
$ SET ON
$ ON WARNING THEN SHOW SYMBOL Y
$ FOOBAR
$ SET NOON
$ SHOW SYMBOL NOSUCH
$ !
$ ! V9: the settings, SET VERIFY's forms, and a nested procedure's
$ ! changes.
$ M = "V9"
$ SHOW SYMBOL M
$ SET VERIFY=PROCEDURE
$ E = F$ENVIRONMENT("VERIFY_IMAGE")
$ SHOW SYMBOL E
$ SET VERIFY=(PROC,IMAG)
$ SET NOVERIFY=IMAGE
$ SET VERIFY
$ @PROBE50W
$ M = "V9 after PROBE50W"
$ SHOW SYMBOL M
$ E = F$ENVIRONMENT("VERIFY_PROCEDURE")
$ SHOW SYMBOL E
$ SET VERIFY
$ @PROBE50W QUIET
$ M = "V9 after PROBE50W QUIET"
$ SHOW SYMBOL M
$ E = F$ENVIRONMENT("VERIFY_PROCEDURE")
$ SHOW SYMBOL E
$ SET VERIFY
$ !
$ ! The end.
$ M = "END"
$ SHOW SYMBOL M
$ SET NOVERIFY
