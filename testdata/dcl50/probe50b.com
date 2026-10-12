$ ! PROBE50B.COM - round 6's nested cases (testdata/dcl50/README.md),
$ ! one per P1, run by PROBE50A.COM. Each case ends with EXIT (or ends
$ ! the procedure some other way, which is what it asks about).
$ GOTO 'P1'
$ !
$ ! R3: a procedure run with /OUTPUT.
$ HELLO:
$ WRITE SYS$OUTPUT "HELLO from PROBE50B"
$ EXIT
$ !
$ ! R2: CALL of a label that isn't a SUBROUTINE.
$ CALLPLAIN:
$ CALL CP_PLAIN
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "CALLPLAIN: after the CALL"
$ EXIT
$ CP_PLAIN:
$ WRITE SYS$OUTPUT "CALLPLAIN: at the plain label"
$ EXIT
$ !
$ ! R2: how deep GOSUB nests. R6N (global) is the depth reached.
$ GOSUBMAX:
$ N = 0
$ GOSUB GM_DOWN
$ WRITE SYS$OUTPUT "GOSUBMAX: back at the top"
$ SHOW SYMBOL $STATUS
$ EXIT
$ GM_DOWN:
$ N = N + 1
$ R6N == N
$ IF N .LT. 20 THEN GOSUB GM_DOWN
$ RETURN
$ !
$ ! R2: how deep block IFs nest: 16 levels. R6N (global) is the depth
$ ! reached. (Last in the file, so no other case's search passes it.)
$ IFMAX:
$ IF 1
$ THEN
$ R6N == 1
$ IF 1
$ THEN
$ R6N == 2
$ IF 1
$ THEN
$ R6N == 3
$ IF 1
$ THEN
$ R6N == 4
$ IF 1
$ THEN
$ R6N == 5
$ IF 1
$ THEN
$ R6N == 6
$ IF 1
$ THEN
$ R6N == 7
$ IF 1
$ THEN
$ R6N == 8
$ IF 1
$ THEN
$ R6N == 9
$ IF 1
$ THEN
$ R6N == 10
$ IF 1
$ THEN
$ R6N == 11
$ IF 1
$ THEN
$ R6N == 12
$ IF 1
$ THEN
$ R6N == 13
$ IF 1
$ THEN
$ R6N == 14
$ IF 1
$ THEN
$ R6N == 15
$ IF 1
$ THEN
$ R6N == 16
$ WRITE SYS$OUTPUT "IFMAX: at the 16th"
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ ENDIF
$ WRITE SYS$OUTPUT "IFMAX: after the ENDIFs"
$ SHOW SYMBOL $STATUS
$ EXIT
