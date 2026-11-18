      OPTIONS /F90
C     Arrays in I/O lists (Phase 102, subtask 11): a whole array, one
C     call; a section and an expression, a call per element; a constructor,
C     its values in turn; reading into a section whose extent the same
C     statement reads, and into a vector-subscripted one.
      PROGRAM F90IO
      REAL A(10), B(2, 3)
      INTEGER IV(3)
      PRINT *, A
      PRINT *, A(1:9:2), B(2, :) + 1.0
      PRINT *, (/ 0, (I, I = 1, N) /)
      READ *, N, A(1:N)
      READ *, A(IV)
      END
