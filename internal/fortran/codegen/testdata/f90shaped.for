      OPTIONS /F90/CHECK=BOUNDS
C     Fortran 90's assumed-shape dummies (Phase 104, subtask 5a): the entry's
C     copy of the caller's descriptor, with a dummy's own lower bounds, and an
C     OPTIONAL one's passed over when it is left out; elements through the
C     strides, checked against the copy's bounds; and a caller's descriptors of
C     a whole array, sections (one of an adjustable array), a CHARACTER array,
C     and an allocatable array's own, through an interface block.
      SUBROUTINE SHAPED(A, N, C, O)
      INTEGER N
      REAL A(N:, :)
      CHARACTER*(*) C(:)
      REAL, OPTIONAL :: O(:)
      A(N, 1) = A(N + 1, 2) + SIZE(A)
      PRINT *, C(2), LBOUND(A, 1), UBOUND(A, 2)
      IF (PRESENT(O)) O(1) = 0.0
      END
      OPTIONS /F90/CHECK=BOUNDS
      SUBROUTINE CALLER(X, M)
      INTEGER M
      REAL X(M, 3), Y(4, 3)
      REAL, ALLOCATABLE :: H(:, :)
      CHARACTER*4 S(6)
      INTERFACE
        SUBROUTINE SHAPED(A, N, C, O)
        INTEGER N
        REAL A(N:, :)
        CHARACTER*(*) C(:)
        REAL, OPTIONAL :: O(:)
        END SUBROUTINE
      END INTERFACE
      ALLOCATE (H(2, 2))
      CALL SHAPED(Y, 0, S)
      CALL SHAPED(Y(1:4:2, 3:1:-1), 1, S(2:6:2), Y(2, :))
      CALL SHAPED(X, M, S, X(M, :))
      CALL SHAPED(H, 2, S(6:1:-1))
      END
