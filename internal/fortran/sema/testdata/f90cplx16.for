C     Fortran 90's COMPLEX(16) (Phase 100, subtask 7): COMPLEX*32 and
C     COMPLEX(16), a constant with a part of kind 16, sums, differences,
C     products, quotients, negation, comparisons, conversions both ways,
C     CMPLX with KIND=16, an initial value, an element, and a function's
C     result, arrays and dummy arrays with indexes.
      PROGRAM F90CPLX16
      COMPLEX(16) :: Z = (1.5_16, -2.0_16), W, A(3)
      COMPLEX*32 V
      COMPLEX*16 D
      REAL*16 H
      COMPLEX(16) TWICE
      W = Z + (0.5_16, 1.0)
      V = Z * W - A(2)
      V = Z / W
      V = -V
      A(3) = V
      D = V
      V = D
      H = V
      V = H
      V = CMPLX(H, H, KIND=16)
      H = AIMAG(V) + REAL(V)
      V = CONJG(V)
      D = DCMPLX(V)
      IF (Z .EQ. W) V = Z
      IF (Z .NE. W) V = TWICE(Z)
      CALL TAKE(Z, A)
      V = (2.0_16, 3.0_16)
      A(I) = V
      V = A(I) * A(I+1)
      CALL TAKE(A(I), V / A(I))
      END

      COMPLEX(16) FUNCTION TWICE(X)
      COMPLEX(16) X
      TWICE = X + X
      END

      SUBROUTINE TAKE(X, Y)
      COMPLEX(16) X, Y(*)
      Y(N) = X
      Y(N+2) = Y(N-1) / X
      END
