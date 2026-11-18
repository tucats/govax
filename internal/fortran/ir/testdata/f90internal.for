      OPTIONS /F90
C     Internal procedures (Phase 103, subtask 3): each a routine of its own,
C     HOST.INNER, called at its local label. A host's variable or COMMON
C     variable is its cell, reached (K, the dummies' cells); a dummy the
C     internal procedures reach goes through a cell the host fills as it
C     starts (N, S, and the adjustable array V's descriptor), one they don't
C     stays in the argument list (F); a recursive one calls itself.
      SUBROUTINE HOST(V, N, F, S)
      INTEGER N
      REAL V(N), F
      CHARACTER*(*) S
      COMMON /BLK/ C
      INTEGER K
      K = 0
      CALL EACH
      F = F * TWICE(F)
      CONTAINS
      SUBROUTINE EACH
      DO J = 1, N
        V(J) = V(J) + C
      END DO
      S(1:1) = 'X'
      K = K + FACT(3)
      END SUBROUTINE EACH
      REAL FUNCTION TWICE(X)
      TWICE = X + X
      END FUNCTION
      RECURSIVE INTEGER FUNCTION FACT(M)
      IF (M .LE. 1) THEN
        FACT = 1
      ELSE
        FACT = M * FACT(M - 1)
      END IF
      END FUNCTION FACT
      END SUBROUTINE HOST
