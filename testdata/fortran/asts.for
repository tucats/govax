C     AST routines written in FORTRAN (Phase 83, subtask 7): $DCLAST with a parameter,
C     $SETIMR and $QIO completion with $HIBER and $WAKE, $SETAST holding one off, I/O
C     from an AST during a statement, and a routine shared with the interrupted code.
      PROGRAM ASTS
      INCLUDE '($SSDEF)'
      INCLUDE '($IODEF)'
      EXTERNAL PARM, WAKER, DONE, LATER
      INTEGER SYS$DCLAST, SYS$SETIMR, SYS$HIBER, SYS$ASSIGN, SYS$QIO
      INTEGER SYS$SETAST, SYS$CANTIM
      INTEGER DELTA(2), ST, CHAN, I
      INTEGER*2 IOSB(4)
      CHARACTER*12 BUF, LINE
      CHARACTER*6 INLIST
      COMMON /FLAGS/ NLATER
      DATA DELTA /-100000, -1/
C
C     An AST's parameter is its first argument, by value: %LOC gets it.
      ST = SYS$DCLAST(PARM, %VAL(1234),)
      PRINT *, 'DCLAST', ST
C
C     A timer's AST wakes the program, and its identification is the parameter.
      I = SYS$SETIMR(, DELTA, WAKER, %VAL(77))
      ST = SYS$HIBER()
      PRINT *, 'SETIMR', I
      PRINT *, 'WOKEN', ST
C
C     A timer canceled by its identification never comes.
      ST = SYS$SETIMR(, DELTA, WAKER, %VAL(88))
      ST = SYS$CANTIM(%VAL(88),)
      PRINT *, 'CANTIM', ST
C
C     A $QIO's completion AST, its IOSB set by then.
      LINE = 'LINE BY QIO' // CHAR(10)
      ST = SYS$ASSIGN('SYS$OUTPUT', CHAN,,)
      ST = SYS$QIO(, %VAL(CHAN), %VAL(IO$_WRITEVBLK), IOSB, DONE,
     1  %VAL(5), %REF(LINE), %VAL(12),,,,)
      ST = SYS$HIBER()
      PRINT *, 'IOSB', IOSB(1), IOSB(2)
C
C     $SETAST holds an AST off until they're enabled again, and says how it was.
      ST = SYS$SETAST(%VAL(0))
      PRINT *, 'WASSET', ST .EQ. SS$_WASSET
      I = SYS$DCLAST(LATER,,)
      PRINT *, 'HELD', NLATER
      ST = SYS$SETAST(%VAL(1))
      PRINT *, 'WASCLR', ST .EQ. SS$_WASCLR, NLATER
C
C     I/O from an AST during a statement, on another unit, an internal file, and
C     unit 6: the statement goes on.
      OPEN (12, STATUS='SCRATCH')
      WRITE (12, '(A,A)') 'first', INLIST()
      REWIND 12
      READ (12, '(A)') BUF
      CLOSE (12)
      PRINT *, BUF
C
C     A routine the interrupted code is in: its AUTOMATIC variables are its own
C     in each call, its static ones shared.
      CALL SHARED(1)
      END

      SUBROUTINE PARM(P)
      PRINT *, 'PARM', %LOC(P)
      END

      SUBROUTINE WAKER(P)
      INTEGER SYS$WAKE
      PRINT *, 'WAKER', %LOC(P)
      I = SYS$WAKE(,)
      END

      SUBROUTINE DONE(P)
      INTEGER SYS$WAKE
      PRINT *, 'DONE', %LOC(P)
      I = SYS$WAKE(,)
      END

      SUBROUTINE LATER
      COMMON /FLAGS/ NLATER
      NLATER = NLATER + 1
      END

C     An item of a statement on unit 12: an AST, then the rest of the statement.
      CHARACTER*6 FUNCTION INLIST()
      EXTERNAL ASTIO
      INTEGER SYS$DCLAST
      I = SYS$DCLAST(ASTIO,,)
      PRINT *, 'AFTER THE AST'
      INLIST = 'second'
      END

      SUBROUTINE ASTIO
      CHARACTER*10 TEXT
      WRITE (TEXT, '(I5)') 42
      WRITE (6, '(1X,A,A)') 'INTERNAL', TEXT
      WRITE (13, '(A)') 'ANOTHER'
      REWIND 13
      READ (13, '(A)') TEXT
      CLOSE (13, DISPOSE='DELETE')
      PRINT *, 'UNIT 13 ', TEXT
      END

C     At the first level, an AST comes in the middle and calls this routine again.
      SUBROUTINE SHARED(LEVEL)
      EXTERNAL AGAIN
      INTEGER SYS$DCLAST
      INTEGER STAT
      AUTOMATIC AUTO
      INTEGER AUTO
      SAVE STAT
      AUTO = LEVEL * 10
      STAT = LEVEL * 10
      IF (LEVEL .EQ. 1) I = SYS$DCLAST(AGAIN,,)
      PRINT *, 'LEVEL', LEVEL, AUTO, STAT
      END

      SUBROUTINE AGAIN
      CALL SHARED(2)
      END
