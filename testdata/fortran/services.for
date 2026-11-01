C     Calling system services and RTL routines as functions (Phase 83, subtask
C     5), with arguments left out as a VMS programmer leaves them out: item lists
C     from a structure, descriptors for character arguments, quadword times, %VAL,
C     and a service govax lacks, which a handler hears of. What depends on the
C     system is compared, not printed.
      PROGRAM SERVICES
      IMPLICIT NONE
      INCLUDE '($SSDEF)'
      INCLUDE '($RMSDEF)'
      INCLUDE '($JPIDEF)'
      INCLUDE '($SYIDEF)'
      INCLUDE '($LNMDEF)'
      INCLUDE '($SYSSRVNAM)'
      STRUCTURE /ITEM/
        INTEGER*2 BUFLEN, CODE
        INTEGER*4 BUFADR, RETADR
      END STRUCTURE
      RECORD /ITEM/ LIST(3)
      INTEGER*4 ST, PID, PID2, RLEN, IOSB(2), TIME(2), PARAMS(2)
      INTEGER*4 LIB$GETJPI, LIB$SPAWN, LIB$GET_INPUT
      INTEGER*2 W
      CHARACTER*12 USER
      CHARACTER*8 VERSION
      CHARACTER*40 EQUIV, LINE
      CHARACTER*23 TEXT
      CHARACTER*11 CLOCK
      EXTERNAL HANDLER
C
C     $GETJPIW's list: two items and the end, the arguments before and after it
C     left out, and LIB$GETJPI's answer the same.
      LIST(1).BUFLEN = 4
      LIST(1).CODE = JPI$_PID
      LIST(1).BUFADR = %LOC(PID)
      LIST(1).RETADR = %LOC(RLEN)
      LIST(2).BUFLEN = LEN(USER)
      LIST(2).CODE = JPI$_USERNAME
      LIST(2).BUFADR = %LOC(USER)
      LIST(2).RETADR = 0
      LIST(3).BUFLEN = 0
      LIST(3).CODE = 0
      USER = ' '
      ST = SYS$GETJPIW(,,,LIST,IOSB,,)
      PRINT *, ST .EQ. SS$_NORMAL, IOSB(1) .EQ. SS$_NORMAL, RLEN,
     1         USER .NE. ' '
      ST = LIB$GETJPI(JPI$_PID,,,PID2)
      PRINT *, ST .EQ. SS$_NORMAL, PID .EQ. PID2
C
C     $GETSYIW: the version is 8 characters.
      LIST(1).BUFLEN = LEN(VERSION)
      LIST(1).CODE = SYI$_VERSION
      LIST(1).BUFADR = %LOC(VERSION)
      LIST(1).RETADR = %LOC(RLEN)
      LIST(2).BUFLEN = 0
      LIST(2).CODE = 0
      ST = SYS$GETSYIW(,,,LIST,IOSB,,)
      PRINT *, ST .EQ. SS$_NORMAL, RLEN, VERSION(1:1)
C
C     $BINTIM and $ASCTIM: a quadword time, a date and time, and the time alone.
      ST = SYS$BINTIM('19-OCT-1987 13:30:15.25', TIME)
      ST = SYS$ASCTIM(W, TEXT, TIME,)
      PRINT *, ST .EQ. SS$_NORMAL, W, ' ', TEXT
      ST = SYS$ASCTIM(W, CLOCK, TIME, %VAL(1))
      PRINT *, ST .EQ. SS$_NORMAL, W, ' ', CLOCK
      ST = SYS$BINTIM('NOT A TIME', TIME)
      PRINT *, ST .EQ. SS$_IVTIME
C
C     $CRELNM and $TRNLNM: the same list, for the value and for the answer.
      EQUIV = 'THE VALUE'
      LIST(1).BUFLEN = 9
      LIST(1).CODE = LNM$_STRING
      LIST(1).BUFADR = %LOC(EQUIV)
      LIST(1).RETADR = 0
      ST = SYS$CRELNM(,'LNM$PROCESS','SERVICES_NAME',,LIST)
      PRINT *, ST .EQ. SS$_NORMAL
      ST = SYS$CRELNM(,'LNM$PROCESS','SERVICES_NAME',,LIST)
      PRINT *, ST .EQ. SS$_SUPERSEDE
      EQUIV = ' '
      LIST(1).BUFLEN = LEN(EQUIV)
      LIST(1).RETADR = %LOC(RLEN)
      ST = SYS$TRNLNM(,'LNM$PROCESS','SERVICES_NAME',,LIST)
      PRINT *, ST .EQ. SS$_NORMAL, RLEN, ' ', EQUIV(:RLEN)
      ST = SYS$TRNLNM(,'LNM$PROCESS','NO_SUCH_NAME',,LIST)
      PRINT *, ST .EQ. SS$_NOLOGNAM
C
C     $FAO with arguments by value and by descriptor, and $FAOL with a list.
      ST = SYS$FAO('!AS has !UL files', W, LINE, 'DISK', %VAL(5))
      PRINT *, ST .EQ. SS$_NORMAL, W, ' ', LINE(:W)
      PARAMS(1) = 7
      PARAMS(2) = 255
      ST = SYS$FAOL('!UL then !XL', W, LINE, PARAMS)
      PRINT *, ST .EQ. SS$_NORMAL, W, ' ', LINE(:W)
C
C     LIB$DATE_TIME as a subroutine: only where its punctuation falls.
      CALL LIB$DATE_TIME(TEXT)
      PRINT *, TEXT(3:3), TEXT(7:7), TEXT(15:15), TEXT(18:18),
     1         TEXT(21:21)
C
C     A subprocess's command, and its output before the status comes back.
      ST = LIB$SPAWN('WRITE SYS$OUTPUT "FROM THE SUBPROCESS"')
      PRINT *, ST .EQ. SS$_NORMAL
C
C     A line from the terminal, with a prompt, and the end of the input.
      ST = LIB$GET_INPUT(LINE, 'Name? ', W)
      PRINT *, ST .EQ. SS$_NORMAL, W, ' ', LINE(:W)
      ST = LIB$GET_INPUT(LINE,,)
      PRINT *, ST .EQ. RMS$_EOF
C
C     A service govax lacks signals, naming itself, and returns SS$_UNSUPPORTED
C     to a program whose handler continues.
      CALL LIB$ESTABLISH(HANDLER)
      ST = SYS$GETCHN(,,,,)
      PRINT *, ST .EQ. SS$_UNSUPPORTED
      CALL LIB$REVERT
C
C     This compiler's own routines take null arguments the same way.
      CALL NULLS
      END

      INTEGER*4 FUNCTION HANDLER(SIGARGS, MECHARGS)
      INCLUDE '($SSDEF)'
      INTEGER*4 SIGARGS(*), MECHARGS(*)
      PRINT *, 'HANDLER', SIGARGS(1), SIGARGS(2) .EQ. SS$_UNSUPPORTED,
     1         SIGARGS(4)
      HANDLER = SS$_CONTINUE
      END

      SUBROUTINE NULLS
      INTEGER WHICH
      CHARACTER*3 NAMEIT
      PRINT *, WHICH(,,), WHICH(1,,), WHICH(,2,), WHICH(,,3),
     1         WHICH(1,2,3)
      PRINT *, WHICH(,2,) + WHICH(1,,) * 10, WHICH(WHICH(,,3),,)
      PRINT *, NAMEIT(,2), NAMEIT(1,), NAMEIT(1,2)
      END

C     Which of its arguments it was given, as the digits of a number: a null
C     argument's address is 0.
      INTEGER FUNCTION WHICH(A, B, C)
      WHICH = 0
      IF (%LOC(A) .NE. 0) WHICH = WHICH + 100
      IF (%LOC(B) .NE. 0) WHICH = WHICH + 10
      IF (%LOC(C) .NE. 0) WHICH = WHICH + 1
      END

C     A value through a hidden first argument: the null ones keep their places.
C     (A CHARACTER dummy is reached through its descriptor, so a program can't
C     ask where a null one is.)
      CHARACTER*3 FUNCTION NAMEIT(A, B)
      NAMEIT = '---'
      IF (%LOC(A) .NE. 0) NAMEIT(1:1) = 'A'
      IF (%LOC(B) .NE. 0) NAMEIT(3:3) = 'B'
      END
