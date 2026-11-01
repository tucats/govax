C     Exit handlers written in FORTRAN (Phase 83, subtask 7): last declared first run,
C     the status each is given, one canceled, one that changes the status by
C     $EXIT, a unit still open, one closed already, and one opened by a handler.
      PROGRAM EXITS
      EXTERNAL FIRST, SECOND, THIRD, GONE
      INTEGER SYS$DCLEXH, SYS$CANEXH
      INTEGER B1(4), B2(5), B3(4), B4(4), ST1, ST2, ST3, ST4, ST
      COMMON /BLOCKS/ B1, B2, B3, B4, ST1, ST2, ST3, ST4
C
C     Declared before any unit is used, FTNRTL's own exit handler being declared at
C     the first use of one, so it runs after the units are closed.
      B1(2) = %LOC(FIRST)
      B1(3) = 1
      B1(4) = %LOC(ST1)
      ST = SYS$DCLEXH(B1)
      PRINT *, 'DECLARE', ST
      OPEN (12, FILE='EXITS.DAT', STATUS='NEW')
      WRITE (12, '(A)') 'FROM THE PROGRAM'
C
C     These run before FTNRTL's, so unit 12 is still open in them. A second argument
C     after the status is the handler's own.
      B2(2) = %LOC(SECOND)
      B2(3) = 2
      B2(4) = %LOC(ST2)
      B2(5) = %LOC(ST1)
      ST1 = 123
      PRINT *, 'DECLARE', SYS$DCLEXH(B2)
      B4(2) = %LOC(GONE)
      B4(3) = 1
      B4(4) = %LOC(ST4)
      PRINT *, 'DECLARE', SYS$DCLEXH(B4)
      B3(2) = %LOC(THIRD)
      B3(3) = 1
      B3(4) = %LOC(ST3)
      PRINT *, 'DECLARE', SYS$DCLEXH(B3)
C
C     Canceled: it doesn't run.
      PRINT *, 'CANCEL', SYS$CANEXH(B4)
C
C     Running off the end is an exit with SS$_NORMAL; a handler changes the status.
      PRINT *, 'END'
      END

      SUBROUTINE THIRD(ST)
      INTEGER ST
      PRINT *, 'THIRD', ST
      WRITE (12, '(A)') 'FROM THIRD'
      CALL SYS$EXIT(%VAL(3))
      PRINT *, 'NOT REACHED'
      END

      SUBROUTINE SECOND(ST, X)
      INTEGER ST, X
      PRINT *, 'SECOND', ST, X
      WRITE (12, '(A)') 'FROM SECOND'
      END

      SUBROUTINE GONE(ST)
      INTEGER ST
      PRINT *, 'CANCELED, NOT REACHED'
      END

      SUBROUTINE FIRST(ST)
      INTEGER ST
      CHARACTER*20 TEXT
      PRINT *, 'FIRST', ST
      OPEN (12, FILE='EXITS.DAT', STATUS='OLD')
10    READ (12, '(A)', END=20) TEXT
      PRINT *, TEXT
      GO TO 10
20    CLOSE (12, DISPOSE='DELETE')
      WRITE (13, '(A)') 'OPENED LATE'
      REWIND 13
      READ (13, '(A)') TEXT
      CLOSE (13, DISPOSE='DELETE')
      PRINT *, TEXT
      END
