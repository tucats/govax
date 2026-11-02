      PROGRAM SUBS
C     Work through dummy arguments: a bubble sort and a dot product,
C     an adjustable array, a function called from a loop.
      INTEGER N
      PARAMETER (N = 400)
      REAL X(N), Y(N), DOT, D
      INTEGER I, L
      DO 5 L = 1, 5
      DO 10 I = 1, N
         X(I) = MOD(I*7919, 1009)
         Y(I) = FLT(I)
   10 CONTINUE
      CALL BUBBLE(X, N)
      D = DOT(X, Y, N)
    5 CONTINUE
      PRINT *, D
      END
      REAL FUNCTION FLT(I)
      INTEGER I
      FLT = I*0.5 + 1.0
      END
      SUBROUTINE BUBBLE(A, N)
      INTEGER N, I, J
      REAL A(N), T
      DO 20 I = 1, N-1
         DO 10 J = 1, N-I
            IF (A(J) .GT. A(J+1)) THEN
               T = A(J)
               A(J) = A(J+1)
               A(J+1) = T
            END IF
   10    CONTINUE
   20 CONTINUE
      END
      REAL FUNCTION DOT(A, B, N)
      INTEGER N, I
      REAL A(N), B(N)
      DOT = 0.0
      DO 10 I = 1, N
         DOT = DOT + A(I)*B(I)
   10 CONTINUE
      END
