C     Phase 79, subtask 3: DO loops without labels, DO WHILE, END DO,
C     and a jump out of a loop and back into it (the extended range).
      PROGRAM LOOPS
      INTEGER I, J, K, N
      LOGICAL MORE
      N = 0
      DO I = 1, 3
         DO J = I, 3
            N = N + J
         END DO
      END DO
      K = 1
      DO WHILE (K .LT. 100)
         K = K * 2
      END DO
      DO 10 WHILE (K .GT. 0)
         K = K - 30
   10 CONTINUE
      DO 20, I = 1, N
         IF (I .GT. 2) GO TO 20
         MORE = I .EQ. 1
   20 END DO
      DO 40 I = 1, 2
         GO TO 30
   25    N = N + 1
   40 CONTINUE
      STOP
   30 N = N - 1
      GO TO 25
      END
