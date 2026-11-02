      PROGRAM MATMUL
C     Matrix multiplication, the classic triple loop.
      INTEGER N
      PARAMETER (N = 60)
      REAL A(N,N), B(N,N), C(N,N), S
      INTEGER I, J, K
      DO 20 J = 1, N
         DO 10 I = 1, N
            A(I,J) = I + J
            B(I,J) = 2*I - J
            C(I,J) = 0.0
   10    CONTINUE
   20 CONTINUE
      DO 50 J = 1, N
         DO 40 K = 1, N
            DO 30 I = 1, N
               C(I,J) = C(I,J) + A(I,K)*B(K,J)
   30       CONTINUE
   40    CONTINUE
   50 CONTINUE
      S = 0.0
      DO 60 I = 1, N
         S = S + C(I,I)
   60 CONTINUE
      PRINT *, S
      END
