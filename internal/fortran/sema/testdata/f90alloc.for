      OPTIONS /F90
C     Fortran 90's allocatable arrays (Phase 104, subtask 2): each is its
C     class NCA descriptor; ALLOCATE and DEALLOCATE are FTNRTL's calls,
C     ALLOCATED a test of POINTER; an element is A0 indexed by its first
C     subscript, plus each other subscript times its stride; a whole
C     array in an output list is a class A descriptor copied from it; a
C     recursive unit's is in the frame, cleared at entry.
      PROGRAM F90ALC
      REAL, ALLOCATABLE :: A(:), B(:, :)
      INTEGER N, I, J
      IF (.NOT. ALLOCATED(A)) ALLOCATE (A(N), B(0:N, 3))
      A(I) = B(I, J) + B(1, 2)
      PRINT *, A, SIZE(B, 2), UBOUND(B, 1)
      DEALLOCATE (A, B)
      END
      RECURSIVE SUBROUTINE R(N)
      INTEGER, ALLOCATABLE :: W(:)
      ALLOCATE (W(N))
      W(N) = N
      END
