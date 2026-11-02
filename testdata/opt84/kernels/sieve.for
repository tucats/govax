      PROGRAM SIEVE
C     The sieve of Eratosthenes, run ten times over.
      INTEGER M
      PARAMETER (M = 20000)
      LOGICAL*1 F(M)
      INTEGER I, K, ITER, COUNT, P
      DO 100 ITER = 1, 10
         COUNT = 0
         DO 10 I = 1, M
            F(I) = .TRUE.
   10    CONTINUE
         DO 30 I = 2, M
            IF (.NOT. F(I)) GO TO 30
            P = I
            COUNT = COUNT + 1
            K = I + P
   20       IF (K .GT. M) GO TO 30
            F(K) = .FALSE.
            K = K + P
            GO TO 20
   30    CONTINUE
  100 CONTINUE
      PRINT *, COUNT
      END
