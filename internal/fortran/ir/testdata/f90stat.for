      OPTIONS /F90
C     Fortran 90's allocation status (Phase 104, subtask 3): STAT=, which
C     ends the statement at its first failure; a subprogram's allocatable
C     arrays deallocated at each return if allocated, and marked not
C     allocated at each entry point; a SAVE'd one left alone; RETURN i's
C     value computed before the deallocations.
      SUBROUTINE S(N, IS, *)
      REAL, ALLOCATABLE :: A(:), B(:)
      REAL, ALLOCATABLE, SAVE :: K(:)
      INTEGER*2 IS(2)
      ALLOCATE (A(N), B(N), STAT=IS(2))
      IF (IS(2) .NE. 0) RETURN 1
      DEALLOCATE (A, STAT=IS(1))
      ENTRY T(N)
      IF (.NOT. ALLOCATED(K)) ALLOCATE (K(N))
      END
