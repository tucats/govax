      PROGRAM ARRAYS
C     Sections 2.2.3 and 4.5: bounds of every kind, and subscripts.
      PARAMETER (N = 3, LO = -N)
      DIMENSION BOS(3,3,3), V(0:9), W(LO:N, 2*N), X(2.9)
      INTEGER M(N:N+1)
      VIRTUAL Z(-1:1)
      BOS(2, 1, 3) = V(0) + W(-3, 6)
      M(N + 1) = 1
      X(2) = BOS(1.9, 1, 1)
      Z(0) = 0
      CALL SUM2(BOS, 3, 3)
      END
      SUBROUTINE SUM2(A, B, C)
C     Section 6.1.1.2's forms: constant lower bounds, and *.
      REAL A(3, 3, *), B(-1:*), C(*)
      A(1, 2, 3) = B(-1) + C(1)
      END
      SUBROUTINE ADJ(X, I, J, Y, K)
C     Section 6.1.1.1's example: bounds that aren't constant, and
C     section 4.5's, which has one on an assumed-size array.
      DIMENSION X(-I/2:I/2, J), Y(K:*)
      END
