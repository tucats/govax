      PROGRAM ARRAYS
C     Elements in several dimensions, with lower bounds of every sign,
C     constant and computed subscripts, and whole arrays.
      PARAMETER (N = 3)
      DIMENSION M(N, 0:1), B(-1:1, 2, 2)
      INTEGER M
      DO 10 J = 0, 1
      DO 10 I = 1, N
   10 M(I, J) = I + 10 * J
      B(0, 2, 1) = M(2, 1)
      B(I - 1, J + 1, 2) = 1.5
      PRINT *, M
      CALL TWO(M, 3)
      END
      SUBROUTINE TWO(A, L)
      INTEGER A(3, *)
      A(L, 2) = A(1, L - 2) + 1
      CALL ONE(A(1, 2))
      END
