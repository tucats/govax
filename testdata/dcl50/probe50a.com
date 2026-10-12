$ ! PROBE50A.COM - round 6 of Phase 50's probe (testdata/dcl50/README.md):
$ ! what round 5 left open, and a few older choices. Run from SYSTEM, with
$ ! the exchange volume as the default directory:
$ !
$ !     @PROBE50A/OUTPUT=PROBE50A.LOG
$ !
$ ! It waits for two seconds in R1; nothing else waits. Each section starts
$ ! by showing a marker symbol M, and most cases are followed by SHOW
$ ! SYMBOL $STATUS.
$ SET NOON
$ SET VERIFY
$ !
$ ! R1: WAIT's $STATUS when it runs to its end, after a success and after
$ ! a warning; a bad time.
$ M = "R1"
$ SHOW SYMBOL M
$ WRITE SYS$OUTPUT "before the wait"
$ SHOW SYMBOL $STATUS
$ WAIT 00:00:01
$ SHOW SYMBOL $STATUS
$ FROBNICATE
$ WAIT 00:00:01
$ SHOW SYMBOL $STATUS
$ WAIT NOTATIME
$ SHOW SYMBOL $STATUS
$ !
$ ! R2: flow. ELSE before a block's THEN (PROBE50C, its P1 the IF's
$ ! value); CALL of a label that isn't a SUBROUTINE; how deep GOSUB and
$ ! block IF nest.
$ M = "R2"
$ SHOW SYMBOL M
$ @PROBE50C 1
$ SHOW SYMBOL $STATUS
$ @PROBE50C 0
$ SHOW SYMBOL $STATUS
$ @PROBE50B CALLPLAIN
$ SHOW SYMBOL $STATUS
$ R6N == 0
$ @PROBE50B GOSUBMAX
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R6N
$ R6N == 0
$ @PROBE50B IFMAX
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R6N
$ !
$ ! R3: @'s /OUTPUT: the default file type, /O, and blanks around "=".
$ M = "R3"
$ SHOW SYMBOL M
$ DELETE R6OUT*.*;*
$ @PROBE50B/OUTPUT=R6OUTA HELLO
$ SHOW SYMBOL $STATUS
$ @PROBE50B/O=R6OUTB.LOG HELLO
$ SHOW SYMBOL $STATUS
$ @PROBE50B/OUTPUT = R6OUTC.LOG HELLO
$ SHOW SYMBOL $STATUS
$ DIRECTORY R6OUT*.*
$ !
$ ! R4: the fixed control bytes of a file OPEN/WRITE makes.
$ M = "R4"
$ SHOW SYMBOL M
$ DELETE PROBE50AV.DAT;*
$ OPEN/WRITE OUT PROBE50AV.DAT
$ WRITE OUT "abc"
$ WRITE OUT ""
$ WRITE OUT "defgh"
$ CLOSE OUT
$ DUMP/BLOCK=COUNT=1 PROBE50AV.DAT
$ !
$ ! R5: SHOW LOGICAL's indentation: a name, a name whose value is a
$ ! logical name (the numbered level), a search list, /FULL, a wildcard,
$ ! and a table's listing.
$ M = "R5"
$ SHOW SYMBOL M
$ DEFINE R6A "plain"
$ DEFINE R6B R6A
$ DEFINE R6C "one", "two"
$ SHOW LOGICAL R6A
$ SHOW LOGICAL R6B
$ SHOW LOGICAL R6C
$ SHOW LOGICAL/FULL R6B
$ SHOW LOGICAL R6*
$ SHOW LOGICAL/PROCESS
$ DEASSIGN R6A
$ DEASSIGN R6B
$ DEASSIGN R6C
$ !
$ ! R6: a delta time's hundredths, and commands that don't start with a
$ ! letter.
$ M = "R6"
$ SHOW SYMBOL M
$ X = "-"
$ X = F$CVTIME("1-02:03:04.56", "DELTA")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("1-02:03:04.05", "DELTA")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("1-02:03:04", "DELTA")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("0-0:0:0.123", "DELTA")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("3-4", "DELTA")
$ SHOW SYMBOL X
$ X = "-"
$ X = F$CVTIME("1-02:03:04.56", "DELTA", "HUNDREDTH")
$ SHOW SYMBOL X
$ 1ABC
$ SHOW SYMBOL $STATUS
$ .X
$ SHOW SYMBOL $STATUS
$ (X)
$ SHOW SYMBOL $STATUS
$ M = "END"
$ SHOW SYMBOL M
