      PROGRAM OPTAL
C     What an optimizer must not assume about where values are (Phase 84's
C     Decisions 6 and 7). Each case reads wrong if a value is kept, in a register
C     or as a known constant, past something that changes it.
      INTEGER I, J, K, N, M, TOTAL, IW, NEXT, LAST, STEP, LIMIT, HITS, P
      INTEGER*2 HALF(2)
      REAL A(3), R
      CHARACTER*3 DIGITS(3)
      CHARACTER*1 C
      LOGICAL ABOVE
      EQUIVALENCE (I, J), (IW, HALF)
      COMMON /SHARED/ LIMIT, HITS
      COMMON /PTR/ P
      DATA STEP /2/, A /1.0, 2.0, 3.0/
      DATA DIGITS /'  7', ' 11', ' 13'/
  100 FORMAT (1X, A, 3I8)
C
C     One place under two names, changed in a loop by one and read by the other.
      TOTAL = 0
      I = 1
      DO 10 K = 1, 3
         J = I + K
         TOTAL = TOTAL + I
   10 CONTINUE
      PRINT 100, 'EQUIVALENCE', I, J, TOTAL
C
C     A longword changed through its halves.
      IW = 65537
      DO 20 K = 1, 2
         HALF(K) = HALF(K) + 1
   20 CONTINUE
      PRINT 100, 'OVERLAP', IW, HALF(1), HALF(2)
C
C     COMMON changed by a routine called in a loop that tests it.
      LIMIT = 5
      HITS = 0
      K = 0
   30 K = K + 1
      CALL BUMP
      IF (K .LT. LIMIT) GO TO 30
      PRINT 100, 'COMMON', K, HITS, LIMIT
C
C     And across one call, in straight-line code.
      N = LIMIT
      CALL BUMP
      N = N + LIMIT
      PRINT 100, 'ACROSS CALL', N, HITS, LIMIT
C
C     A local passed by reference, changed each trip.
      N = 1
      DO 40 K = 1, 3
         CALL TWICE(N)
         TOTAL = N + K
   40 CONTINUE
      PRINT 100, 'BY REFERENCE', N, TOTAL
C
C     A local changed through an address, by a call that doesn't name it: KEEP
C     puts M's address in COMMON, and POKE stores through that.
      M = 1
      CALL KEEP(M)
      M = 10
      CALL POKE(20)
      N = M + 1
      PRINT 100, 'KEPT ADDRESS', M, N
C
C     A DO variable after the loop: run out, left by GO TO, and never entered.
      DO 50 K = 1, 3
   50 CONTINUE
      N = K
      DO 60 K = 1, 10
         IF (K .EQ. 4) GO TO 61
   60 CONTINUE
   61 M = K
      DO 62 K = 5, 1
   62 CONTINUE
      PRINT 100, 'DO VARIABLE', N, M, K
C
C     Values kept between calls with no SAVE, which VAX FORTRAN's rule (section
C     4.14) promises: a count, and the DO variable the last call left.
      DO 65 K = 1, 3
         CALL COUNTS(N, M)
   65 CONTINUE
      PRINT 100, 'NO SAVE', N, M
C
C     A variable with a DATA value, then changed.
      K = STEP
      STEP = 3
      N = K + STEP
      PRINT 100, 'DATA', K, STEP, N
C
C     A variable an I/O statement stores, in a loop.
      TOTAL = 0
      DO 70 K = 1, 3
         READ (DIGITS(K), '(I3)') N
         TOTAL = TOTAL + N
   70 CONTINUE
      PRINT 100, 'READ', N, TOTAL
C
C     The variable each direct access statement sets, though none names it.
      OPEN (UNIT=1, STATUS='SCRATCH', ACCESS='DIRECT', RECL=1,
     1      FORM='UNFORMATTED', ASSOCIATEVARIABLE=NEXT)
      WRITE (1, REC=3) 33
      LAST = NEXT
      READ (1, REC=3) N
      WRITE (1, REC=5) 55
      PRINT 100, 'ASSOCIATE', LAST, NEXT, N
      CLOSE (1)
C
C     Dummy arguments may share storage as long as neither is changed.
      CALL SUMS(A, A, 3, R)
      N = R
      PRINT 100, 'SAME ARRAY', N
C
C     A dummy argument's array changed in a loop, and an element of it reread.
      CALL RAISE(A, 3)
      N = A(3)
      PRINT 100, 'THROUGH DUMMY', N
C
C     A character's order is unsigned: CHAR(200) is above 'A', though a signed
C     byte comparison says it isn't.
      C = CHAR(200)
      ABOVE = C .GT. 'A'
      IF (ABOVE) PRINT 100, 'UNSIGNED'
      END
C
      SUBROUTINE BUMP
      INTEGER LIMIT, HITS
      COMMON /SHARED/ LIMIT, HITS
      HITS = HITS + 1
      LIMIT = LIMIT - 1
      END
C
      SUBROUTINE TWICE(N)
      INTEGER N
      N = N + N
      END
C
      SUBROUTINE KEEP(X)
      INTEGER X, P
      COMMON /PTR/ P
      P = %LOC(X)
      END
C
      SUBROUTINE POKE(V)
      INTEGER V, P
      COMMON /PTR/ P
      CALL STORE(%VAL(P), V)
      END
C
      SUBROUTINE STORE(A, V)
      INTEGER A, V
      A = V
      END
C
      SUBROUTINE COUNTS(N, M)
      INTEGER N, M, TIMES, D
      DATA TIMES /0/, D /0/
      M = D
      TIMES = TIMES + 1
      N = TIMES
      DO 10 D = 1, TIMES
   10 CONTINUE
      END
C
      SUBROUTINE SUMS(X, Y, N, R)
      INTEGER N, I
      REAL X(N), Y(N), R
      R = 0.0
      DO 10 I = 1, N
         R = R + X(I)*Y(I)
   10 CONTINUE
      END
C
      SUBROUTINE RAISE(X, N)
      INTEGER N, I
      REAL X(N)
      DO 10 I = 2, N
         X(I) = X(I-1) + X(I)
   10 CONTINUE
      END
