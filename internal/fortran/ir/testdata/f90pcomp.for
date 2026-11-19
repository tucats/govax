      OPTIONS /F90
C     Pointer components (Phase 107, subtask 5a): a type pointing at
C     itself, a chain loaded a pointer at a time, => and NULLIFY of a
C     component, a CHARACTER component's header, ASSOCIATED of one,
C     ALLOCATE of one with the clear flag, a constructor's targets, and a
C     recursive unit's record and array of records cleared at entry.
      RECURSIVE SUBROUTINE F90PCOMP(N)
      TYPE NODE
        INTEGER V
        TYPE(NODE), POINTER :: NEXT
        CHARACTER(LEN=4), POINTER :: TAG
      END TYPE
      TYPE(NODE), POINTER :: H
      TYPE(NODE), TARGET :: R, RA(10)
      CHARACTER(LEN=4), TARGET :: S
      INTEGER N
      H => R
      H%NEXT%NEXT%V = N
      R%NEXT => RA(N)%NEXT
      R%TAG => S
      NULLIFY (RA(2)%NEXT, R%TAG)
      IF (ASSOCIATED (R%NEXT, H)) N = 1
      ALLOCATE (H%NEXT)
      R = NODE(3, H, S)
      DEALLOCATE (H%NEXT)
      END
