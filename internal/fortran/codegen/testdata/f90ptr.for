      OPTIONS /F90
C     Fortran 90's scalar pointers (Phase 107, subtask 2): a pointer's
C     cell is its target's address, a CHARACTER pointer's a class S
C     descriptor; a reference is its target, reached through the cell;
C     => stores an address, NULLIFY 0, ASSOCIATED compares; an actual
C     argument is the cell's address itself; a recursive unit's pointer
C     is in the frame, cleared at entry.
      PROGRAM F90PTR
      INTEGER, TARGET :: N, M
      INTEGER, POINTER :: P, Q
      CHARACTER*4, TARGET :: S
      CHARACTER*2, POINTER :: C
      NULLIFY (Q)
      P => N
      P = P + 1
      Q => P
      IF (ASSOCIATED(Q, P) .AND. .NOT. ASSOCIATED(P, M)) CALL X(Q)
      C => S(2:3)
      C = 'AB'
      PRINT *, ASSOCIATED(C), C
      END
      OPTIONS /F90
      RECURSIVE SUBROUTINE R(K)
      INTEGER, TARGET :: K
      INTEGER, POINTER :: L
      IF (.NOT. ASSOCIATED(L)) L => K
      L = 0
      END
