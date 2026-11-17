      OPTIONS /CHECK=BOUNDS
C     Fortran 90's array assignments under /CHECK=BOUNDS (Phase 102,
C     subtask 4): a section's first and last subscripts checked once,
C     before its loop, only when it has elements; a section in range by
C     its constants checked not at all; extents not known to be equal
C     compared, FTN$NOTCONFOR if they differ; an adjustable array's.
      PROGRAM F90CHK
      REAL A(10), B(10)
      INTEGER I, J, N
      A(2:9) = B(1:8)
      A(I:J) = B(1:N)
      A(1:N:2) = 0.0
      CALL S(A, B, N)
      END
      OPTIONS /CHECK=BOUNDS
      SUBROUTINE S(X, Y, M)
      REAL X(M), Y(M)
      X(3:) = Y(1:M-2) + 1.0
      END
