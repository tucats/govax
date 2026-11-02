C     What the branch rules of /OPTIMIZE must get right (Phase 84, subtask 3):
C     GO TO loops that close with AOBLEQ, AOBLSS, and ACBL, which read their limit
C     before they step and go by the step's sign; a limit that overlaps the index;
C     loops too long for a byte branch; condition codes read after a step; chains
C     of branches; and code nothing reaches.
      PROGRAM OPTBRN
      INTEGER I, N, K, COUNT, S, IE, NE, L
      INTEGER*2 W
      LOGICAL YES
      EQUIVALENCE (IE, NE)
C     A loop closed by I + 1 and .LE.
      I = 0
      COUNT = 0
   10 COUNT = COUNT + 1
      I = I + 1
      IF (I .LE. 5) GO TO 10
      PRINT *, 'LE', I, COUNT
C     By I + 1 and .LT.
      I = 0
      COUNT = 0
   20 COUNT = COUNT + 1
      I = I + 1
      IF (I .LT. 5) GO TO 20
      PRINT *, 'LT', I, COUNT
C     The limit a variable, the comparison written the other way round.
      N = 3
      I = 0
      COUNT = 0
   30 COUNT = COUNT + 1
      I = I + 1
      IF (N .GE. I) GO TO 30
      PRINT *, 'GE', I, COUNT
C     The limit the index itself, by another name: after the step they're equal,
C     so the loop goes on until the other test ends it. A loop instruction reads
C     its limit first, and would see the index one less.
      IE = 0
      COUNT = 0
   40 COUNT = COUNT + 1
      IF (COUNT .GE. 4) GO TO 41
      IE = IE + 1
      IF (IE .LE. NE) GO TO 40
   41 PRINT *, 'SAME', IE, COUNT
C     Steps of constants, up and down, which go by the sign. Stopping at zero tells
C     .GE. from .GT., which no loop instruction here does for a step down.
      I = 1
      COUNT = 0
   50 COUNT = COUNT + 1
      I = I + 3
      IF (I .LE. 10) GO TO 50
      PRINT *, 'UP', I, COUNT
      I = 8
      COUNT = 0
   51 COUNT = COUNT + 1
      I = I - 4
      IF (I .GE. 0) GO TO 51
      PRINT *, 'DOWNGE', I, COUNT
      I = 8
      COUNT = 0
   52 COUNT = COUNT + 1
      I = I - 4
      IF (I .GT. 0) GO TO 52
      PRINT *, 'DOWNGT', I, COUNT
      I = 8
      COUNT = 0
   53 COUNT = COUNT + 1
      I = I - 1
      IF (I .GE. 6) GO TO 53
      PRINT *, 'DOWN1', I, COUNT
C     A shorter integer, which no loop instruction steps.
      W = 0
      COUNT = 0
   54 COUNT = COUNT + 1
      W = W + 1
      IF (W .LE. 3) GO TO 54
      PRINT *, 'WORD', W, COUNT
C     A dummy argument as the index and as the limit.
      I = 0
      COUNT = 7
      CALL UPTO(I, COUNT)
      PRINT *, 'DUMMY', I, COUNT
C     Loops too long for a byte branch, by DO and by GO TO.
      S = 0
      DO 60 I = 1, 3
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
         S = S + I
   60 CONTINUE
      PRINT *, 'FARDO', I, S
      I = 0
      S = 0
   61 S = S + 1
      S = S + 1
      S = S + 1
      S = S + 1
      S = S + 1
      S = S + 1
      S = S + 1
      S = S + 1
      S = S + 1
      S = S + 1
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      S = S + N
      I = I + 1
      IF (I .LE. 2) GO TO 61
      PRINT *, 'FARGOTO', I, S
C     An arithmetic IF right after a step, which reads what the step left in the
C     condition codes.
      K = 2
      N = 0
   70 K = K - 1
      IF (K) 73, 72, 71
   71 N = N + 1
      GO TO 70
   72 PRINT *, 'ZERO', K, N
      K = K - 1
      IF (K) 73, 74, 74
   73 PRINT *, 'NEGATIVE', K
      GO TO 75
   74 PRINT *, 'NOTNEG', K
   75 CONTINUE
C     Branches to branches, one in a condition, and a logical's low bit.
      K = 0
      GO TO 80
   81 GO TO 82
   80 GO TO 81
   82 IF (K .EQ. 0) GO TO 83
      PRINT *, 'NEVER1'
   83 GO TO 84
      PRINT *, 'NEVER2'
   84 YES = .TRUE.
      IF (YES) GO TO 85
      PRINT *, 'NEVER3'
   85 IF (.NOT. YES) GO TO 86
      PRINT *, 'CHAINS'
   86 CONTINUE
C     A statement that branches to itself, which nothing reaches.
      GO TO 91
   90 GO TO 90
   91 CONTINUE
C     Labels reached only through an ASSIGN, or only through a table.
      ASSIGN 94 TO L
      GO TO L, (93, 94)
   93 PRINT *, 'NEVER4'
   94 K = 2
      GO TO (95, 96, 95), K
   95 PRINT *, 'NEVER5'
   96 PRINT *, 'REACHED'
C     A branch to a RETURN, from either side of it.
      PRINT *, 'ISPOS', ISPOS(5), ISPOS(-5), ISPOS(0)
      END
C
      SUBROUTINE UPTO(I, N)
      INTEGER I, N
   10 I = I + 1
      IF (I .LE. N) GO TO 10
      RETURN
      END
C
      INTEGER FUNCTION ISPOS(N)
      INTEGER N
      ISPOS = 0
      IF (N .GT. 0) GO TO 10
      GO TO 20
   10 ISPOS = 1
      GO TO 20
      ISPOS = 2
   20 RETURN
      END
