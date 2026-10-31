      PROGRAM LOOPS
C     The loops of sections 5.3 and 5.4: DO without a label, ended by
C     END DO, with positive, negative, and REAL steps, nested, and run no
C     times; DO WHILE, ended by END DO, by a labeled terminal statement,
C     and by a labeled END DO, and run no times; a labeled DO ended by a
C     labeled END DO, which a jump goes to for the next pass; a DO and a
C     DO WHILE sharing a terminal statement; a jump out of a loop, and
C     out and back in again (the extended range, 5.3.1.4).
      INTEGER I, J, K, N, NUMBER, PASSES
      REAL X
      CHARACTER*20 LINE

C     The manual's example: nine passes, and NUMBER is 41 after them.
      PASSES = 0
      DO NUMBER = 5, 40, 4
         PASSES = PASSES + 1
      END DO
      PRINT *, 'Passes', PASSES, ', NUMBER', NUMBER

C     And its negative step: 27 passes, and J is -4 after them.
      PASSES = 0
      DO J = 50, -2, -2
         PASSES = PASSES + 1
      END DO
      PRINT *, 'Passes', PASSES, ', J', J

C     Nested loops, each with its own END DO.
      N = 0
      DO I = 1, 4
         DO J = I, 4
            N = N + 1
         END DO
      END DO
      PRINT *, 'Triangle', N, ', I', I, ', J', J

C     A count of zero or less runs the body no times.
      K = 7
      DO I = 5, 1
         K = 0
      END DO
      PRINT *, 'No passes: K', K, ', I', I

C     A REAL loop: (2.0 - 0.5 + 0.5)/0.5 passes.
      PASSES = 0
      DO X = 0.5, 2.0, 0.5
         PASSES = PASSES + 1
      END DO
      PRINT *, 'REAL loop passes', PASSES, ', 10X', INT(X * 10.0)

C     DO WHILE tests before every pass, the first included.
      K = 1
      DO WHILE (K .LT. 100)
         K = K * 3
      END DO
      PRINT *, 'Powers of 3: K', K
      DO WHILE (K .LT. 0)
         K = 0
      END DO
      PRINT *, 'Still', K

C     The manual's DO WHILE, with a labeled terminal statement.
      LINE = '    TEXT'
      I = 1
      DO 10 WHILE (LINE(I:I) .EQ. ' ')
         I = I + 1
   10 CONTINUE
      PRINT *, 'First nonblank at', I, ': ', LINE(I:I+3)

C     A labeled END DO, and a jump to it for the next pass.
      N = 0
      DO 20 I = 1, 10
         IF (MOD(I, 3) .NE. 0) GO TO 20
         N = N + I
   20 END DO
      PRINT *, 'Multiples of 3 add to', N
      K = 0
      DO 30 WHILE (K .LT. 5)
         K = K + 2
   30 END DO
      PRINT *, 'Twos: K', K

C     A jump out of a loop leaves its variable as it was.
      DO I = 1, 100
         IF (I * I .GT. 50) GO TO 40
      END DO
   40 PRINT *, 'First square past 50:', I

C     A DO and a DO WHILE share a terminal statement.
      N = 0
      DO 50 I = 1, 3
         K = I
         DO 50 WHILE (K .GT. 0)
            N = N + 1
            K = K - 1
   50 CONTINUE
      PRINT *, 'Shared terminal: N', N

C     The extended range: out of the loop and back in, every pass.
      N = 0
      DO 70 I = 1, 3
         GO TO 60
   65    N = N + 10
   70 CONTINUE
      GO TO 80
   60 N = N + I
      GO TO 65
   80 PRINT *, 'Extended range: N', N
      N = 0
      DO I = 1, 2
         GO TO 90
   85    N = N + 100
      END DO
      GO TO 95
   90 N = N + 1
      GO TO 85
   95 PRINT *, 'Back into a loop without a label: N', N
      END
