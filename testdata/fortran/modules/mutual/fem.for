C     F(0) = 1, F(N) = N - M(F(N-1)).
      RECURSIVE INTEGER FUNCTION FEM(N) RESULT(F)
      INTEGER N, MALE
      IF (N .EQ. 0) THEN
         F = 1
      ELSE
         F = N - MALE(FEM(N - 1))
      END IF
      END

C     Move N discs from FROM to TO: STEP, in the other file, does the work.
      RECURSIVE SUBROUTINE HANOI(N, FROM, TO, VIA)
      INTEGER N
      CHARACTER*1 FROM, TO, VIA
      IF (N .GT. 0) CALL STEP(N, FROM, TO, VIA)
      END
