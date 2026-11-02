      PROGRAM OPTFA
C     Faults that mustn't appear, and those that mustn't go (Phase 84's Decision
C     8): an operation that would fault is safe where it never runs, however
C     constant it looks, and a division by zero that does run still traps, even
C     when the result is never used and both operands are known. There is no
C     handler until the end, so a fault moved before the loops ends the image.
      INCLUDE '($SSDEF)'
      EXTERNAL TALLY
      INTEGER TALLY, NCOND, I, N, K, L, M, TRIPS, RB
      REAL X, Y, Z, R, S
      EQUIVALENCE (R, RB)
      COMMON /CONDS/ NCOND
  100 FORMAT (1X, A, 2I8)
      Y = 1.0
      Z = 0.0
      X = 5.0
      K = 0
      L = 7
      M = 9
C
C     A loop that runs no times, around a floating division by zero.
      N = 0
      DO 10 I = 1, N
         X = Y/Z
   10 CONTINUE
      M = X
      PRINT 100, 'NO TRIPS', M
C
C     A division by zero in a loop, under a test that is never true.
      TRIPS = 0
      DO 20 I = 1, 3
         IF (Z .NE. 0.0) X = Y/Z
         TRIPS = TRIPS + 1
   20 CONTINUE
      M = X
      PRINT 100, 'GUARDED', TRIPS, M
C
C     An integer one, the same way.
      TRIPS = 0
      M = 9
      DO 30 I = 1, 3
         IF (K .NE. 0) M = L/K
         TRIPS = TRIPS + 1
   30 CONTINUE
      PRINT 100, 'INTEGER', TRIPS, M
C
C     A reserved floating operand, which faults when it is loaded, under a test.
      RB = '8000'X
      S = 2.0
      TRIPS = 0
      DO 40 I = 1, 3
         IF (K .NE. 0) S = R
         TRIPS = TRIPS + 1
   40 CONTINUE
      M = S
      PRINT 100, 'RESERVED', TRIPS, M
C
C     A DO WHILE never entered.
      M = 4
      DO WHILE (Z .NE. 0.0)
         M = L/K
      END DO
      PRINT 100, 'WHILE', M
C
C     Now divisions by zero that run, one on values only known at run time and one
C     on constants a compiler can see: each is a condition, though M isn't used.
      NCOND = 0
      CALL LIB$ESTABLISH(TALLY)
      M = L/K
      PRINT 100, 'TRAPPED', NCOND
      K = 0
      L = 12
      M = L/K
      PRINT 100, 'KNOWN', NCOND
      END
C
      INTEGER FUNCTION TALLY(SIGARGS, MECHARGS)
      INTEGER SIGARGS(*), MECHARGS(*), NCOND
      COMMON /CONDS/ NCOND
      INCLUDE '($SSDEF)'
      NCOND = NCOND + 1
      TALLY = SS$_CONTINUE
      END
