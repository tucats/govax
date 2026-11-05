C     RUNAWAY: a recursion that never ends, printing its depth every
C     10,000 levels. Each level's frame is the call's 24 bytes, so a 2 MB
C     stack (paging off) holds some 87,000 levels, and the 10 MB a
C     process's table area leaves P1 with the defaults (paging on) some
C     430,000. Then the image ends with no message and $STATUS
C     %X1000000C (docs/PHASE-89 - reentrant FORTRAN.md, subtask 9).
      PROGRAM RUNAWAY
      INTEGER LEVELS
      COMMON /DEEP/ LEVELS
      LEVELS = 0
      CALL DOWN
      END

      RECURSIVE SUBROUTINE DOWN
      INTEGER LEVELS
      COMMON /DEEP/ LEVELS
      LEVELS = LEVELS + 1
      IF (MOD(LEVELS, 10000) .EQ. 0) PRINT *, LEVELS
      CALL DOWN
      END
