C     SIGNAL: a condition in a routine whose 800,000-byte frame is
C     reserved but not yet touched. The signal and mechanism arrays go
C     below it, on pages that don't exist yet with paging on, so the
C     stack grows for the report, and the program's handler and the
C     traceback run as for any frame (docs/PHASE-89 - reentrant
C     FORTRAN.md, subtask 9).
      PROGRAM SIGNAL
      INTEGER ZERO, I
      COMMON /Z/ ZERO
      ZERO = 0
      I = 7
      CALL BIG(I)
      PRINT *, 'BACK', I
      CALL BIG(I)
      END

      RECURSIVE SUBROUTINE BIG(N)
      INTEGER N, ZERO, HANDLER, COUNT
      COMMON /Z/ ZERO
      EXTERNAL HANDLER
      REAL*8 A(100000)
      SAVE COUNT
      DATA COUNT /0/
      COUNT = COUNT + 1
      IF (COUNT .EQ. 1) CALL LIB$ESTABLISH(HANDLER)
      N = N / ZERO
      A(1) = N
      A(100000) = N
      PRINT *, 'CONTINUED', COUNT, A(1) + A(100000)
      END

      INTEGER FUNCTION HANDLER(SIGARGS, MECHARGS)
      INTEGER SIGARGS(*), MECHARGS(*)
      INCLUDE '($SSDEF)'
      PRINT 10, SIGARGS(2)
10    FORMAT (' HANDLER SAW %X', Z8.8)
      HANDLER = SS$_CONTINUE
      END
