C     Fortran 90's array assignments (Phase 102, subtask 4): whole arrays,
C     sections, a scalar broadcast, an elemental expression, a section of
C     a column, a negative stride, extents known only at run time, and an
C     adjustable array, each a nest of counted loops over normalized indexes.
      PROGRAM F90ARR
      REAL A(10), B(10), C(4, 6)
      LOGICAL L(10)
      INTEGER N
      N = 5
      A = 0
      B = A + 2.0 * B
      L = A .GT. B
      A(2:10) = B(1:9)
      B(10:1:-1) = A
      C(:, 3) = A(1:4)
      C(2, :) = 1.0
      A(1:N) = B(N:N+N-1) * 2
      C = C + 1.0
      CALL S(A, N)
      END
      SUBROUTINE S(X, M)
      REAL X(M)
      X = X * 2.0
      X(2:) = 1.0
      END
