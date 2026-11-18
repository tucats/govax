      OPTIONS /F90
C     OPTIONAL dummies and PRESENT (Phase 103, subtask 6): each OPTIONAL
C     dummy is reached through a cell its entry fills (with 0 if the caller's
C     list is too short for it), PRESENT is a test of the cell, an internal
C     procedure tests its host's, and a call through an explicit interface
C     with an argument left out passes 0 for it.
      SUBROUTINE OPT(A, B, C)
      REAL A
      REAL, OPTIONAL :: B
      CHARACTER*(*), OPTIONAL :: C
      LOGICAL L
      L = PRESENT(B)
      IF (PRESENT(C)) A = B
      CALL INNER(A)
      CALL INNER(Y=A, X=2.0)
      CONTAINS
      SUBROUTINE INNER(X, Y)
      REAL X
      REAL, OPTIONAL :: Y
      IF (PRESENT(B) .AND. PRESENT(Y)) Y = X
      END SUBROUTINE INNER
      END
