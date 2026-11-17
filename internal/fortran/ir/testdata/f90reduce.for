      OPTIONS /F90
C     Fortran 90's array functions in line (Phase 102, subtask 9): a
C     reduction with MASK=, one with DIM= at each element, COUNT, MAXLOC,
C     DOT_PRODUCT, MATMUL, TRANSPOSE, and SIZE and UBOUND of an adjustable
C     array.
      PROGRAM F90RED
      REAL A(3, 4), B(4), C(4, 2), D(3, 2), E(4, 3), X
      INTEGER K(2), N
      X = SUM(A, MASK=A .GT. 0.0)
      B = MAXVAL(A, DIM=1)
      N = COUNT(A .GT. X)
      K = MAXLOC(A)
      X = DOT_PRODUCT(B, B)
      D = MATMUL(A, C)
      E = TRANSPOSE(A)
      CALL S(B, 4)
      END
      OPTIONS /F90
      SUBROUTINE S(V, M)
      REAL V(M)
      N = SIZE(V) + UBOUND(V, 1)
      END
