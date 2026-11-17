C     Vector subscripts in Fortran 90's array assignments (Phase 102,
C     subtask 6): read and assigned, an expression as a vector, a vector
C     beside a triplet, a vector of a vector, and an extent known only from
C     an adjustable vector.
      PROGRAM F90VEC
      REAL A(10), B(3), C(4, 10), D(4, 3)
      INTEGER IV(3), JV(5)
      B = A(IV)
      A(IV) = B + 1.0
      B = A(IV + 1) * A(11 - IV(3:1:-1))
      D = C(:, IV)
      B = A(IV(JV(1:3)))
      CALL S(A, IV, 3)
      END
      SUBROUTINE S(X, KV, M)
      REAL X(10)
      INTEGER KV(M)
      X(KV) = 0.0
      END
