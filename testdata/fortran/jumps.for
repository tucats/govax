      PROGRAM JUMPS
C     The jumps of sections 5.6 and 5.7.1: the computed GO TO, in range
C     and out of it, with INTEGER, INTEGER*2, and REAL indexes; ASSIGN
C     and the assigned GO TO, with a list and without, in this unit and
C     through a dummy argument; the arithmetic IF of INTEGER, INTEGER*2,
C     REAL, and REAL*8 values, with three labels and with two the same.
      INTEGER I, K, M, N
      INTEGER*2 I2
      REAL X
      DOUBLE PRECISION D

C     The computed GO TO: an index out of range goes on.
      DO 30 I = 0, 4
         GO TO (11, 12, 13), I
         PRINT *, 'Index', I, ' goes on'
         GO TO 30
   11    PRINT *, 'Index', I, ' goes to 11'
         GO TO 30
   12    PRINT *, 'Index', I, ' goes to 12'
         GO TO 30
   13    PRINT *, 'Index', I, ' goes to 13'
   30 CONTINUE

C     A REAL index is truncated; an INTEGER*2 expression is one too.
      X = 2.7
      GO TO (41, 42, 43), X
   41 PRINT *, 'Wrong: 41'
   42 PRINT *, 'X = 2.7 goes to 42'
      X = -0.5
      GO TO (41, 43), X
      PRINT *, 'X = -0.5 goes on'
   43 I2 = 2
      GO TO (51, 52, 53), I2 + 1
   51 PRINT *, 'Wrong: 51 or 52'
   52 STOP 'Wrong'
   53 PRINT *, 'I2 + 1 = 3 goes to 53'

C     ASSIGN and the assigned GO TO: each step says where to go next.
      ASSIGN 61 TO K
      N = 0
   60 N = N + 1
      GO TO K, (61, 62, 63)
   61 PRINT *, 'Step', N, ' at 61'
      ASSIGN 62 TO K
      GO TO 60
   62 PRINT *, 'Step', N, ' at 62'
      ASSIGN 63 TO K
      GO TO 60
   63 PRINT *, 'Step', N, ' at 63'
      CALL PICK(1, K)
      CALL PICK(2, M)

C     The arithmetic IF.
      DO 80 I = -1, 1
         IF (I) 71, 72, 73
   71    PRINT *, I, ' is negative'
         GO TO 80
   72    PRINT *, I, ' is zero'
         GO TO 80
   73    PRINT *, I, ' is positive'
   80 CONTINUE
      X = 0.25
      IF (X - 0.5) 91, 91, 92
   91 PRINT *, 'X - 0.5 is not positive'
      D = -1.0D-30
      IF (D) 93, 92, 92
   92 STOP 'Wrong'
   93 PRINT *, 'D is negative'
      I2 = 0
      IF (I2) 92, 94, 92
   94 PRINT *, 'I2 is zero'
      END

C     ASSIGN to a dummy argument, and GO TO it.
      SUBROUTINE PICK(J, K)
      INTEGER J, K
      IF (J .EQ. 1) ASSIGN 10 TO K
      IF (J .NE. 1) ASSIGN 20 TO K
      GO TO K
   10 PRINT *, 'PICK', J, ' went to 10'
      RETURN
   20 PRINT *, 'PICK', J, ' went to 20'
      END
