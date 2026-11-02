C Phase 83 end to end: one program using VMS as a VMS programmer did. The
C definition modules, a service's item list from a structure, RTL routines with
C null arguments, STR$, a handler that recovers from a failed OPEN, a timer AST
C the main program sleeps for, an exit handler, and the system subroutines.
      PROGRAM TOUR
      IMPLICIT NONE
      INCLUDE '($SSDEF)'
      INCLUDE '($JPIDEF)'
      INCLUDE '($SYSSRVNAM)'
      STRUCTURE /ITEM/
          INTEGER*2 BUFLEN, CODE
          INTEGER*4 BUFADR, RETADR
      END STRUCTURE
      RECORD /ITEM/ LIST(3)
      INTEGER*4 PID, PID2, STATUS, IOSB(2), EXITBLOCK(4), EXITVALUE
      INTEGER*4 LIB$GETJPI, LIB$DATE_TIME
      INTEGER*4 STR$CONCAT, STR$UPCASE, STR$TRIM, OPENED
      INTEGER*2 NLEN, NLEN2, OUTLEN
      CHARACTER*15 NAME, NAME2
      CHARACTER*40 LINE
      CHARACTER*9 TODAY
      CHARACTER*23 NOW
      INTEGER*2 M, D, Y
      EXTERNAL TICK, ATEXIT
      COMMON /TIMER/ TICKS, DELAY, AGAIN
      INTEGER*4 TICKS, DELAY(2), AGAIN
      VOLATILE TICKS

C The exit handler goes first: it runs last of what the program leaves behind.
      EXITBLOCK(1) = 0
      EXITBLOCK(2) = %LOC(ATEXIT)
      EXITBLOCK(3) = 1
      EXITBLOCK(4) = %LOC(EXITVALUE)
      CALL CHECK(SYS$DCLEXH(EXITBLOCK))

C An item list from a structure, ended by a longword of zero in the next code.
      LIST(1).BUFLEN = 4
      LIST(1).CODE = JPI$_PID
      LIST(1).BUFADR = %LOC(PID)
      LIST(1).RETADR = 0
      LIST(2).BUFLEN = LEN(NAME)
      LIST(2).CODE = JPI$_PRCNAM
      LIST(2).BUFADR = %LOC(NAME)
      LIST(2).RETADR = %LOC(NLEN)
      LIST(3).BUFLEN = 0
      LIST(3).CODE = 0
      CALL CHECK(SYS$GETJPIW(, , , LIST, IOSB, , ))
      CALL CHECK(IOSB(1))
      CALL CHECK(LIB$GETJPI(JPI$_PID, , , PID2))
      CALL CHECK(LIB$GETJPI(JPI$_PRCNAM, , , , NAME2, NLEN2))
      PRINT 100, 'The two ways agree', PID .EQ. PID2 .AND.
     1      NAME(1:NLEN) .EQ. NAME2(1:NLEN2)

C STR$ on CHARACTER variables: fixed strings, padded and cut.
      CALL CHECK(STR$CONCAT(LINE, 'one ', 'two', '   '))
      CALL CHECK(STR$UPCASE(LINE, LINE))
      CALL CHECK(STR$TRIM(LINE, LINE, OUTLEN))
      PRINT 101, LINE(1:OUTLEN), OUTLEN

C A failed OPEN, with no ERR=: the handler sees FTNRTL's condition and unwinds.
C It prints, so this isn't called in a PRINT's list: that would be the one unit
C twice at once, FOR$_RECIO_OPE.
      STATUS = OPENED('NOSUCH.DAT')
      PRINT 100, 'NOSUCH.DAT opened', STATUS .NE. 0

C A timer AST, which sets itself again until it has run three times.
      CALL CHECK(SYS$BINTIM('0 00:00:00.10', DELAY))
      AGAIN = %LOC(TICK)
      CALL CHECK(SYS$SETIMR(, DELAY, TICK, %VAL(7), ))
      CALL CHECK(SYS$HIBER())
      CALL CHECK(SYS$FAO('!UL tick!%S', OUTLEN, LINE, %VAL(TICKS)))
      PRINT 102, LINE(1:OUTLEN)

C The system subroutines, against an RTL routine's idea of the date.
      CALL CHECK(LIB$DATE_TIME(NOW))
      CALL DATE(TODAY)
      CALL IDATE(M, D, Y)
      PRINT 100, 'DATE agrees', TODAY(1:6) .EQ. NOW(1:6) .AND.
     1      TODAY(8:9) .EQ. NOW(10:11)
      PRINT 100, 'IDATE in range', M .GE. 1 .AND. M .LE. 12 .AND.
     1      D .GE. 1 .AND. D .LE. 31 .AND. Y .GE. 0 .AND. Y .LE. 99

      CALL EXIT(SS$_NORMAL)

  100 FORMAT (1X, A, ': ', L1)
  101 FORMAT (1X, '"', A, '" ', I2)
  102 FORMAT (1X, A)
      END

C Stop with the failure's own message if a status says so.
      SUBROUTINE CHECK(STATUS)
      INTEGER*4 STATUS
      IF (.NOT. STATUS) CALL LIB$STOP(%VAL(STATUS))
      END

C OPENED is 1 if the file opens, and 0 if a handler had to take it back.
      INTEGER*4 FUNCTION OPENED(NAME)
      CHARACTER*(*) NAME
      INTEGER*4 RECOVER
      EXTERNAL RECOVER
      CALL LIB$ESTABLISH(RECOVER)
      OPEN (UNIT=12, FILE=NAME, STATUS='OLD')
      OPENED = 1
      CLOSE (UNIT=12)
      END

C A handler: its value says what to do, unless it has unwound, and the unwind
C calls it again with SS$_UNWIND, which it passes over. The mechanism's depth is
C the establisher, and one more its caller, so OPENED itself returns, with the
C R0 left here as its value.
      INTEGER*4 FUNCTION RECOVER(SIGARGS, MECHARGS)
      IMPLICIT NONE
      INCLUDE '($SSDEF)'
      INCLUDE '($FORDEF)'
      INCLUDE '($CHFDEF)'
      INTEGER*4 SIGARGS(*), LIB$MATCH_COND, DEPTH
      RECORD /CHFDEF2/ MECHARGS
      RECOVER = SS$_RESIGNAL
      IF (LIB$MATCH_COND(SIGARGS(2), FOR$_FILNOTFOU) .NE. 0) THEN
          PRINT *, 'Handler: no such file'
          MECHARGS.CHF$L_MCH_SAVR0 = 0
          DEPTH = MECHARGS.CHF$L_MCH_DEPTH + 1
          CALL SYS$UNWIND(DEPTH, )
      END IF
      END

C An AST routine. Its parameter comes by value, where a dummy expects an address,
C so %LOC(PARAM) is the way to it. A routine can't name itself in EXTERNAL, so
C the main program left its address, for %VAL.
      SUBROUTINE TICK(PARAM)
      IMPLICIT NONE
      INCLUDE '($SYSSRVNAM)'
      COMMON /TIMER/ TICKS, DELAY, AGAIN
      INTEGER*4 TICKS, DELAY(2), AGAIN, PARAM
      VOLATILE TICKS
      IF (%LOC(PARAM) .EQ. 7) TICKS = TICKS + 1
      IF (TICKS .LT. 3) THEN
          CALL SYS$SETIMR(, DELAY, %VAL(AGAIN), %VAL(7), )
      ELSE
          CALL SYS$WAKE(, )
      END IF
      END

C The exit handler's first argument is the address of the status, by $DCLEXH's
C rule, a longword VMS fills in.
      SUBROUTINE ATEXIT(EXITSTATUS)
      INTEGER*4 EXITSTATUS
      PRINT 100, EXITSTATUS
  100 FORMAT (1X, 'Exit handler: status ', Z8.8)
      END
