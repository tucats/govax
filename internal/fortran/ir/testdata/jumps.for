C     Phase 79, subtask 2: GO TO in every form, ASSIGN, and the
C     arithmetic IF.
      PROGRAM JUMPS
      INTEGER K, L(3)
      INTEGER*2 N2
      REAL X
      ASSIGN 20 TO K
      GO TO K
   10 GO TO (20, 30, 40), L(2)
   20 GO TO (30, 40), N2 + 1
   30 GO TO (40), X
      IF (X - 1.0) 40, 40, 50
   40 IF (N2) 50, 60, 50
   50 IF (L(1)) 60, 70, 80
   60 ASSIGN 70 TO K
      GO TO K, (70, 80)
   70 IF (X .GT. 0.0) GO TO (80, 10), N2
      CALL SUB(K)
   80 IF (2) 10, 20, 90
   90 GO TO (10, 20), 3
      END

      SUBROUTINE SUB(K)
      INTEGER K
      ASSIGN 10 TO K
      GO TO K
   10 RETURN
      END
