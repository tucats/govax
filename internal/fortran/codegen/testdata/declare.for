C     Declarations of every type and length, with IMPLICIT and IMPLICIT
C     NONE (Phase 76, subtask 3): each variable takes its place in
C     $LOCAL at its natural boundary, and the compiler's own cells
C     follow on a longword. Only the longword types are used: the
C     others' code is Phase 78's.
      PROGRAM DECLARE
      IMPLICIT NONE
      BYTE FLAG
      INTEGER*2 SHORT
      CHARACTER*5 NAME
      REAL*8 BIG
      INTEGER I, TOTAL
      REAL AVG
      DIMENSION SCORES(3)
      INTEGER SCORES
      TOTAL = 0
      DO 10 I = 1, 3
         SCORES(I) = I*I
         TOTAL = TOTAL + SCORES(I)
   10 CONTINUE
      AVG = TOTAL/3.0
      PRINT *, TOTAL, AVG
      END
      SUBROUTINE HALF(X, Y)
      IMPLICIT DOUBLE PRECISION (D), LOGICAL (X-Y)
      LOGICAL*1 B
      IF (X) Y = .NOT. Y
      END
