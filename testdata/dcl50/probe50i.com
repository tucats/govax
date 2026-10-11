$ ! PROBE50I.COM - Phase 50's third probe: SYS$INPUT from a procedure,
$ ! DECK, and EOD, for subtask 16 (testdata/dcl50/README.md). Run it
$ ! from SYSTEM with the exchange volume as the default directory:
$ !
$ !     @PROBE50I/OUTPUT=PROBE50I.LOG
$ !
$ ! Each section starts with a SHOW SYMBOL of a marker. Most data lines
$ ! are WRITE commands without a "$": when DCL reads one as a command,
$ ! its text is written; when DCL skips it, %DCL-W-SKPDAT is.
$ SET NOON
$ X = 1
$ SET VERIFY
$ !
$ ! I1: a data line after each kind of command. Which does DCL read as a
$ ! command, and which skip?
$ M = "I1"
$ SHOW SYMBOL M
$ X = 1
WRITE SYS$OUTPUT "I1 after =: read"
$ X := ONE
WRITE SYS$OUTPUT "I1 after :=: read"
$ SHOW SYMBOL X
WRITE SYS$OUTPUT "I1 after SHOW SYMBOL: read"
$ SHOW SYMBOL NOSUCH
WRITE SYS$OUTPUT "I1 after a failing SHOW SYMBOL: read"
$ SHOW DEFAULT
WRITE SYS$OUTPUT "I1 after SHOW DEFAULT: read"
$ SHOW TRANSLATION SYS$INPUT
WRITE SYS$OUTPUT "I1 after SHOW TRANSLATION: read"
$ SHOW LOGICAL SYS$INPUT
WRITE SYS$OUTPUT "I1 after SHOW LOGICAL: read"
$ SET DEFAULT []
WRITE SYS$OUTPUT "I1 after SET DEFAULT: read"
$ SET PROMPT="$ "
WRITE SYS$OUTPUT "I1 after SET PROMPT: read"
$ SET ON
WRITE SYS$OUTPUT "I1 after SET ON: read"
$ SET NOON
WRITE SYS$OUTPUT "I1 after SET NOON: read"
$ SET VERIFY
WRITE SYS$OUTPUT "I1 after SET VERIFY: read"
$ SET SYMBOL/SCOPE=NOLOCAL
WRITE SYS$OUTPUT "I1 after SET SYMBOL: read"
$ SET SYMBOL/SCOPE=LOCAL
$ DEFINE PROBE_LNM X
WRITE SYS$OUTPUT "I1 after DEFINE: read"
$ ASSIGN X PROBE_LNM2
WRITE SYS$OUTPUT "I1 after ASSIGN: read"
$ DEASSIGN PROBE_LNM
WRITE SYS$OUTPUT "I1 after DEASSIGN: read"
$ DEASSIGN PROBE_LNM2
$ IF 0 THEN SHOW SYMBOL X
WRITE SYS$OUTPUT "I1 after a false IF: read"
$ IF 1 THEN SHOW SYMBOL X
WRITE SYS$OUTPUT "I1 after IF 1 THEN SHOW SYMBOL: read"
$ IF 1 THEN DIRECTORY/NOHEADING/NOTRAILING PROBE50I.COM
WRITE SYS$OUTPUT "I1 after IF 1 THEN DIRECTORY: read"
$ CONTINUE
WRITE SYS$OUTPUT "I1 after CONTINUE: read"
$ WRITE SYS$OUTPUT "a WRITE"
WRITE SYS$OUTPUT "I1 after WRITE: read"
$ WAIT 00:00:00.01
WRITE SYS$OUTPUT "I1 after WAIT: read"
$ DIRECTORY/NOHEADING/NOTRAILING PROBE50I.COM
WRITE SYS$OUTPUT "I1 after DIRECTORY: read"
$ TYPE NOSUCH.TXT
WRITE SYS$OUTPUT "I1 after a failing TYPE: read"
$ FOOBAR
WRITE SYS$OUTPUT "I1 after an unknown verb: read"
$ GOTO I1L
$ I1L:
WRITE SYS$OUTPUT "I1 after GOTO and a label: read"
$ GOSUB I1S
WRITE SYS$OUTPUT "I1 after GOSUB's RETURN: read"
$ CALL I1C
WRITE SYS$OUTPUT "I1 after CALL: read"
$ @PROBE50J
WRITE SYS$OUTPUT "I1 after @: read"
$ SHOW SYMBOL M
WRITE SYS$OUTPUT "I1 two data lines, one"
WRITE SYS$OUTPUT "I1 two data lines, two"
$ !
$ ! I2: a data line as the first record of a procedure (PROBE50J,
$ ! run in I1) is above, after "@".
$ !
$ ! I3: DECK and EOD as commands, with no image reading; their effect on
$ ! $STATUS (SHOW SYMBOL NOSUCH leaves a warning first); DECK/DOLLARS as
$ ! a command (in PROBE50K, in case its deck never ends).
$ M = "I3"
$ SHOW SYMBOL M
$ SHOW SYMBOL NOSUCH
$ DECK
$ WRITE SYS$OUTPUT "I3 a $ line inside a deck: run"
WRITE SYS$OUTPUT "I3 a data line inside a deck: read"
$ EOD
$ SHOW SYMBOL $STATUS
WRITE SYS$OUTPUT "I3 after EOD: read"
$ SHOW SYMBOL NOSUCH
$ EOD
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL NOSUCH
$ DECK
$ EOD
$ SHOW SYMBOL $STATUS
$ @PROBE50K COMMAND
$ !
$ ! I4: TYPE SYS$INPUT through a DECK and EOD: are they shown? What
$ ! becomes of the data after the EOD? $EOD without a blank.
$ M = "I4"
$ SHOW SYMBOL M
$ TYPE SYS$INPUT
I4 before the deck
$ DECK
$ WRITE SYS$OUTPUT "I4 a $ line inside a deck: typed"
$ EOD
WRITE SYS$OUTPUT "I4 after the deck: read"
$ TYPE SYS$INPUT
I4 before an EOD with no deck
$EOD
WRITE SYS$OUTPUT "I4 after $EOD: read"
$ !
$ ! I5: DECK/DOLLARS's string: its case, quoted and not, and whether a
$ ! record that starts with it ends the deck. Each case is a section of
$ ! PROBE50K, so a deck that never ends stops at its end of file.
$ M = "I5"
$ SHOW SYMBOL M
$ @PROBE50K UNQUOTED
$ @PROBE50K QUOTED
$ @PROBE50K BARE
$ @PROBE50K INDENTED
$ !
$ ! I6: READ SYS$INPUT (for subtask 17): the record after the READ; at a
$ ! $ line; at an EOD; and the record after the one READ took.
$ M = "I6"
$ SHOW SYMBOL M
$ L = "-"
$ READ/END_OF_FILE=I6E1 SYS$INPUT L
I6 record one
$ I6E1:
$ SHOW SYMBOL L
$ L = "-"
$ READ/END_OF_FILE=I6E2 SYS$INPUT L
$ WRITE SYS$OUTPUT "I6 READ at a $ line didn't reach end of file"
$ I6E2:
$ SHOW SYMBOL L
$ L = "-"
$ READ/END_OF_FILE=I6E3 SYS$INPUT L
$ EOD
$ WRITE SYS$OUTPUT "I6 READ at EOD didn't reach end of file"
$ I6E3:
$ SHOW SYMBOL L
$ L = "-"
$ READ/END_OF_FILE=I6E4 SYS$INPUT L
I6 record two
WRITE SYS$OUTPUT "I6 the record after READ's: read"
$ I6E4:
$ SHOW SYMBOL L
$ !
$ ! I7: TYPE of a logical name that translates to SYS$INPUT, directly
$ ! and through another.
$ M = "I7"
$ SHOW SYMBOL M
$ DEFINE INFILE SYS$INPUT
$ TYPE INFILE
I7 through INFILE
$ DEFINE OTHER INFILE:
$ TYPE OTHER
I7 through OTHER
$ DEASSIGN OTHER
$ DEASSIGN INFILE
$ !
$ ! I8: which records end the data: a "$" after blanks or a tab, a "$"
$ ! alone, an empty record.
$ M = "I8"
$ SHOW SYMBOL M
$ TYPE SYS$INPUT
I8 one

  $ WRITE SYS$OUTPUT "I8 a $ after blanks: run"
I8 after a $ after blanks
$ TYPE SYS$INPUT
I8 two
	$ WRITE SYS$OUTPUT "I8 a $ after a tab: run"
I8 after a $ after a tab
$ TYPE SYS$INPUT
I8 three
$
I8 after a lone $
$ !
$ ! I9: image verification off: SET VERIFY=(PROCEDURE,NOIMAGE) with a
$ ! deck, and data read as a command with procedure verification off.
$ M = "I9"
$ SHOW SYMBOL M
$ SET VERIFY=(PROCEDURE,NOIMAGE)
$ TYPE SYS$INPUT
$ DECK
I9 in a deck, not shown
$ EOD
$ SET VERIFY=(NOPROCEDURE,IMAGE)
$ X = 1
WRITE SYS$OUTPUT "I9 read as a command, procedure verification off"
$ SET VERIFY
$ SHOW SYMBOL M
$ EXIT
$ !
$ I1S:
$ RETURN
$ I1C: SUBROUTINE
$ ENDSUBROUTINE
