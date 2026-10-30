C     Semantic errors, and statements with syntax errors that cause no
C     further messages.
      PROGRAM ERRS
      INTEGER I, A(10)
      REAL X
      I = 1
      INTEGER J
      IF (X .GT.) THEN
         I = 2
      ELSE
         I = 3
      END IF
      DO 10 I = 1, 10
         READ *, A(I)
         IF (A(I)) GO TO 30
   10 CONTINUE
      X = A
      CALL SUB(A, X)
      GO TO 20
      END
C     A unit whose header has an error is left out.
      SUBROUTINE (B)
      B = 'NOT CHECKED'
      END
      SUBROUTINE SUB(A, X)
      INTEGER A(*)
      DO 40 I = 1, 3
         IF (I .EQ. 2) THEN
            X = A(I)
   40    CONTINUE
      END IF
      END
