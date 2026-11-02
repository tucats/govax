      PROGRAM OPTFL
C     Control flow that isn't a plain DO loop, and code outside the loop's blocks
C     that reads its variables (Phase 84's Decisions 1 and 7): each breaks an
C     optimizer that sees only DO statements or only one way in.
      INTEGER I, K, L, T, V, W, S, N1, N2, N3, OFFS, IF1, VALUE, COUNT
      CHARACTER*6 LINE
      NAMELIST /GROUP/ VALUE
      IF1(I) = I + OFFS
  100 FORMAT (1X, A, 3I8)
C
C     An ENTRY point: a second way in, with variables the first left.
      T = 0
      CALL WALK(2, T)
      CALL AGAIN(2, T)
      PRINT 100, 'ENTRY', T
C
C     An assigned GO TO closing a loop, which only the variable says is one.
      V = 0
      K = 0
      ASSIGN 110 TO L
  105 K = K + 1
      GO TO L, (110, 120)
  110 ASSIGN 120 TO L
      V = V + 1
      GO TO 105
  120 V = V + 10
      PRINT 100, 'ASSIGNED', K, V
C
C     The extended range of a DO: out of the loop and back in.
      S = 0
      DO 200 I = 1, 3
         GO TO 210
  205    S = S + I
  200 CONTINUE
      GO TO 220
  210 S = S + 100
      GO TO 205
  220 PRINT 100, 'EXTENDED', S, I
C
C     A loop made of GO TOs, with the test at its top.
      S = 0
      I = 1
  230 IF (I .GT. 4) GO TO 240
      S = S + I*I
      I = I + 1
      GO TO 230
  240 PRINT 100, 'GO TO LOOP', S, I
C
C     Alternate returns from a call in a loop.
      N1 = 0
      N2 = 0
      N3 = 0
      DO 330 K = 1, 3
         CALL PICK(K, *310, *320)
         N3 = N3 + 10
         GO TO 330
  310    N1 = N1 + 1
         GO TO 330
  320    N2 = N2 + 5
  330 CONTINUE
      PRINT 100, 'ALTERNATE', N1, N2, N3
C
C     A variable format expression, which FTNRTL calls back for, read while the
C     statement runs: it must see the loop's variable.
      DO 400 W = 1, 3
         PRINT 410, W*W
  400 CONTINUE
  410 FORMAT (1X, I<W+1>)
C
C     A namelist group, whose variable FTNRTL finds by a descriptor.
      OPEN (UNIT=1, STATUS='SCRATCH')
      DO 500 K = 1, 2
         VALUE = K*7
         WRITE (1, GROUP)
  500 CONTINUE
      REWIND (1)
      VALUE = 0
      READ (1, GROUP)
      N1 = VALUE
      READ (1, GROUP)
      N2 = VALUE
      CLOSE (1)
      PRINT 100, 'NAMELIST', N1, N2
C
C     A statement function, whose variable changes between references.
      S = 0
      DO 600 K = 1, 3
         OFFS = K
         S = S + IF1(10)
  600 CONTINUE
      PRINT 100, 'STATEMENT FN', S
C
C     One character at a time, compared and moved, in a loop.
      LINE = 'ABCABC'
      COUNT = 0
      DO 700 K = 1, 6
         IF (LINE(K:K) .EQ. 'B') THEN
            COUNT = COUNT + 1
            LINE(K:K) = 'X'
         END IF
  700 CONTINUE
      PRINT 100, LINE, COUNT
      END
C
      SUBROUTINE WALK(N, T)
      INTEGER N, T, I, P
      P = 10
      DO 10 I = 1, N
         T = T + P
   10 CONTINUE
      RETURN
      ENTRY AGAIN(N, T)
      DO 20 I = 1, N
         T = T + P + I
   20 CONTINUE
      END
C
      SUBROUTINE PICK(K, *, *)
      INTEGER K
      IF (K .EQ. 1) RETURN 1
      IF (K .EQ. 2) RETURN 2
      END
