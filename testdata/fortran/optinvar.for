      PROGRAM OPTIN
C     What moving values out of loops must get right (Phase 84, subtask 7). Each
C     loop has something that looks the same on every trip. Values come through
C     COMMON and a CALL, so that nothing earlier settles them, and then into
C     locals no call can reach.
      EXTERNAL NOTHIN
      INTEGER K, L, M, N, I, J, SUM, TRIPS, A(3,3), IA, IW, IC, IV(4)
      REAL X, Y, Z, RV(2), AX(3)
      INTEGER*2 HW(2)
      CHARACTER*2 C2(3)
      EQUIVALENCE (IW, HW)
      COMMON /V/ IV, RV, IC
  100 FORMAT (1X, A, I8)
  101 FORMAT (1X, A, I8, F8.2)
      CALL INIT
      K = IV(1)
      L = IV(2)
      M = IV(3)
      N = IV(4)
      Y = RV(1)
      Z = RV(2)
C
C     Subscripts in nested loops, whose parts leave by different amounts.
      SUM = 0
      DO 12 J = 1, 3
         DO 10 I = 1, 3
            A(I,J) = 10*J + I
   10    CONTINUE
   12 CONTINUE
      DO 16 J = 1, 3
         DO 14 I = 1, 3
            SUM = SUM + A(I,J)
   14    CONTINUE
   16 CONTINUE
      PRINT 100, 'NESTED', SUM
C
C     A division by zero in a loop that doesn't run.
      X = 5.0
      DO 20 I = 1, N
         X = Y/Z
   20 CONTINUE
      PRINT 101, 'NO TRIPS', N, X
C
C     Under a test that is never true.
      TRIPS = 0
      DO 22 I = 1, 3
         IF (Z .NE. 0.0) X = Y/Z
         TRIPS = TRIPS + 1
   22 CONTINUE
      PRINT 101, 'GUARDED', TRIPS, X
C
C     The second operand of a test that the first decides.
      TRIPS = 0
      DO 24 I = 1, 3
         IF (Z .NE. 0.0 .AND. Y/Z .GT. 1.0) X = 1.0
         TRIPS = TRIPS + 1
   24 CONTINUE
      PRINT 101, 'SHORT', TRIPS, X
C
C     Leaving before the division.
      TRIPS = 0
      DO 26 I = 1, 3
         TRIPS = TRIPS + 1
         IF (Z .EQ. 0.0) GO TO 28
         X = Y/Z
   26 CONTINUE
   28 PRINT 101, 'LEFT', TRIPS, X
C
C     A division that runs on the first trip, by something not zero, moved.
      SUM = 0
      DO 30 I = 1, 3
         X = Y/2.0
         AX(I) = X + I
   30 CONTINUE
      PRINT 101, 'SURE', I, AX(3)
C
C     COMMON, changed by the routine a loop calls.
      SUM = 0
      IC = 1
      DO 32 I = 1, 3
         SUM = SUM + IC*3
         CALL BUMP
   32 CONTINUE
      PRINT 100, 'COMMON', SUM
C
C     A local given to the routine a loop calls.
      SUM = 0
      IW = 1
      DO 34 I = 1, 3
         SUM = SUM + IW*2
         CALL BUMPM(IW)
   34 CONTINUE
      PRINT 100, 'PASSED', SUM
C
C     A local no call can reach, in a loop with a call.
      SUM = 0
      DO 36 I = 1, 3
         SUM = SUM + M*3
         CALL NOTHIN
   36 CONTINUE
      PRINT 100, 'KEPT', SUM
C
C     Half of a variable, changed under another name.
      SUM = 0
      IW = 5
      DO 38 I = 1, 3
         SUM = SUM + IW*2
         HW(1) = HW(1) + 1
   38 CONTINUE
      PRINT 100, 'EQUIVALENT', SUM
C
C     One element read, and its array written.
      SUM = 0
      DO 40 I = 1, 3
         SUM = SUM + A(1,1)*2
         A(I,1) = 0
   40 CONTINUE
      PRINT 100, 'ELEMENT', SUM
C
C     A loop made of GO TO.
      SUM = 0
      I = 0
   42 I = I + 1
      SUM = SUM + K*L + I
      IF (I .LT. 3) GO TO 42
      PRINT 100, 'GO TO', SUM
C
C     The extended range: part of the loop, far from it in the text.
      SUM = 0
      J = K
      DO 44 I = 1, 3
         IF (I .EQ. 2) GO TO 48
   46    SUM = SUM + J*L
   44 CONTINUE
      GO TO 49
   48 J = J + 1
      GO TO 46
   49 PRINT 100, 'EXTENDED', SUM
C
C     An assigned GO TO to a loop's top from inside it.
      SUM = 0
      I = 0
      ASSIGN 50 TO IA
   50 I = I + 1
      SUM = SUM + K*L
      IF (I .GE. 3) GO TO 52
      GO TO IA, (50)
   52 PRINT 100, 'ASSIGNED', SUM
C
C     And from outside, into a loop's top.
      SUM = 0
      I = 0
      ASSIGN 54 TO IA
      GO TO IA, (54)
   54 I = I + 1
      SUM = SUM + K*L + I
      IF (I .LT. 3) GO TO 54
      PRINT 100, 'ASSIGNED TOP', SUM
C
C     A dummy argument that wasn't passed, read only under a test.
      CALL OMIT(3, )
C
C     A routine that is nothing but a loop.
      SUM = 0
      CALL TOP(K, L, SUM)
      PRINT 100, 'ROUTINE', SUM
C
C     An entry point at a loop's top.
      SUM = 0
      CALL ENT(K, L, SUM)
      PRINT 100, 'FROM THE TOP', SUM
      CALL ENT2(K, L, SUM)
      PRINT 100, 'FROM ENTRY', SUM
C
C     Adjustable arrays, moved along by one, and given to a routine.
      DO 56 I = 1, 3
         AX(I) = I
   56 CONTINUE
      CALL SHIFT(AX, 3, K, L)
      PRINT 101, 'ADJUSTABLE', 3, AX(1)
      PRINT 101, 'ADJUSTABLE', 3, AX(2)
      PRINT 101, 'ADJUSTABLE', 3, AX(3)
      C2(1) = 'AB'
      C2(2) = 'CD'
      C2(3) = 'EF'
      CALL TEXTS(C2, 3)
      PRINT '(1X, 3(A, 1X))', C2
C
C     A division by zero after a call that never comes back.
      DO 60 I = 1, 3
         CALL GIVEUP(I)
         X = Y/Z
   60 CONTINUE
      PRINT 100, 'NOT REACHED'
      END
C
      SUBROUTINE INIT
      INTEGER IV(4), IC
      REAL RV(2)
      COMMON /V/ IV, RV, IC
      IV(1) = 2
      IV(2) = 3
      IV(3) = 4
      IV(4) = 0
      RV(1) = 1.0
      RV(2) = 0.0
      END
C
      SUBROUTINE BUMP
      INTEGER IV(4), IC
      REAL RV(2)
      COMMON /V/ IV, RV, IC
      IC = IC + 1
      END
C
      SUBROUTINE BUMPM(M)
      INTEGER M
      M = M + 1
      END
C
      SUBROUTINE NOTHIN
      END
C
      SUBROUTINE OMIT(N, IB)
      INTEGER N, IB, I, SUM
      SUM = 0
      DO 10 I = 1, N
         IF (N .GT. 5) SUM = SUM + IB*2
         SUM = SUM + 1
   10 CONTINUE
      PRINT '(1X, A, I8)', 'OMITTED', SUM
      END
C
      SUBROUTINE TOP(K, L, SUM)
      INTEGER K, L, SUM, I
   10 I = SUM/(K*L)
      SUM = SUM + K*L
      IF (I .LT. 2) GO TO 10
      END
C
      SUBROUTINE ENT(K, L, SUM)
      INTEGER K, L, SUM
      SUM = 100
      ENTRY ENT2(K, L, SUM)
   10 SUM = SUM + K*L
      IF (SUM .LT. 118 .AND. SUM .NE. 6 .AND. SUM .NE. 12) GO TO 10
      END
C
      SUBROUTINE SHIFT(B, N, K, L)
      INTEGER N, K, L, J
      REAL B(N)
      DO 10 J = 1, N-1
         B(J) = B(J+1) + K*L
   10 CONTINUE
      CALL ADD1(B(N))
      END
C
      SUBROUTINE ADD1(X)
      REAL X
      X = X + 1.0
      END
C
      SUBROUTINE TEXTS(C, N)
      INTEGER N, J
      CHARACTER*(*) C(N)
      CHARACTER*2 T
      DO 10 J = 1, N-1
         T = C(J)
         C(J) = C(J+1)
         C(J+1) = T
   10 CONTINUE
      END
C
      SUBROUTINE GIVEUP(I)
      INTEGER I
      PRINT '(1X, A, I8)', 'GIVING UP', I
      STOP
      END
