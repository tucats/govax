      OPTIONS /F90
C     Array pointer components (Phase 107, subtask 5b): a component's
C     descriptor reached where its record is, in $LOCAL (H%V) and through
C     a pointer (P%V, its record's address in a temporary); => to a
C     whole array and a section, the header written first; an element
C     and a section; ALLOCATE and DEALLOCATE with the descriptor's address;
C     ASSOCIATED; a constructor filling the component in its record; and
C     an array pointer of the type itself, a field of each record it
C     points at.
      SUBROUTINE F90PCARR(N)
      TYPE HOLDER
        INTEGER K
        INTEGER, POINTER :: V(:)
      END TYPE
      TYPE TREE
        INTEGER KEY
        TYPE(TREE), POINTER :: KIDS(:)
      END TYPE
      TYPE(HOLDER) :: H
      TYPE(HOLDER), POINTER :: P
      TYPE(TREE) :: T
      INTEGER, TARGET :: A(10)
      INTEGER N
      H%V => A
      P%V => A(2:N:2)
      H%V(N) = P%V(1) + 1
      ALLOCATE (P%V(0:N))
      IF (ASSOCIATED (H%V, A)) N = 1
      DEALLOCATE (P%V)
      H = HOLDER(3, A(N:1:-1))
      N = SUM (T%KIDS%KEY)
      END
