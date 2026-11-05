      PROGRAM OPTREC
C     What the optimizer must get right in a RECURSIVE unit (Phase 89's subtask
C     7): a call may come back into the unit and change any of its static
C     locals (SAVE'd, or given initial values), so nothing known of one lasts
C     through a call; its locals in the frame are this activation's own, so
C     what is known of them does. Each case is built so that a pass that kept a
C     static local's value through a call would print something else.
      INTEGER TOTAL, K, N, M
      INTEGER WALK, FWD, CSE, HOIST, STEPS, TRIPS, FRAME
  100 FORMAT (1X, A, 3I8)
C
C     A static local held in a register for a loop with a call in it.
      TOTAL = WALK(3)
      PRINT 100, 'REGISTER', TOTAL
C
C     A constant stored in a static local, forwarded past a call.
      PRINT 100, 'FORWARD', FWD(1), FWD(1)
C
C     The same expression of a static local before and after a call.
      K = 0
      CALL CSE2(K, N)
      PRINT 100, 'COMMON', K, N, CSE(2)
C
C     An expression of a static local hoisted out of a loop with a call.
      PRINT 100, 'INVARIANT', HOIST(2)
C
C     A static local stepped in a loop with a call.
      PRINT 100, 'STEPPED', STEPS(2)
C
C     A DO loop whose limit is a static local the call changes: the loop
C     runs the count it had when it began.
      PRINT 100, 'TRIPS', TRIPS(3)
C
C     The frame's locals keep their values through calls, at every depth.
      M = FRAME(4)
      PRINT 100, 'FRAME', M
      END
C
C     WALK: a SAVE'd count in a loop that recurses.
      RECURSIVE INTEGER FUNCTION WALK(N)
      INTEGER N, I, SUB, TOTAL
      INTEGER COUNT
      SAVE COUNT
      DATA COUNT /0/
      TOTAL = 0
      DO I = 1, N
         COUNT = COUNT + 1
         IF (N .GT. 1) THEN
            SUB = WALK(N-1)
            TOTAL = TOTAL + SUB
         END IF
         COUNT = COUNT + 1
      END DO
      WALK = TOTAL + COUNT
      END
C
C     FWD: V is 7 before the call, which changes it to 8.
      RECURSIVE INTEGER FUNCTION FWD(K)
      INTEGER K, V, W, J
      SAVE V
      IF (K .EQ. 2) THEN
         V = V + 1
         FWD = 0
         RETURN
      END IF
      V = 7
      W = 8
      J = FWD(2)
      FWD = V * 10 + W
      END
C
C     CSE: V * 3 before and after a call that doubles V.
      RECURSIVE INTEGER FUNCTION CSE(K)
      INTEGER K, V, X, Y, J
      DATA V /5/
      IF (K .EQ. 1) THEN
         V = V * 2
         CSE = 0
         RETURN
      END IF
      X = V * 3
      J = CSE(1)
      Y = V * 3
      CSE = X * 1000 + Y
      END
C
C     CSE2: the same through a subroutine, results through dummies.
      RECURSIVE SUBROUTINE CSE2(K, N)
      INTEGER K, N, V, J, L
      SAVE V
      DATA V /4/
      IF (K .EQ. -1) THEN
         V = V + 100
         RETURN
      END IF
      K = V + V
      CALL CSE2(-1, J)
      L = V + V
      N = L
      END
C
C     HOIST: V * 2 in a loop whose call, the first time, adds 10 to V.
      RECURSIVE INTEGER FUNCTION HOIST(K)
      INTEGER K, V, I, T, J
      DATA V /1/
      IF (K .EQ. 1) THEN
         V = V + 10
         HOIST = 0
         RETURN
      END IF
      T = 0
      DO I = 1, 3
         T = T + V * 2
         IF (I .EQ. 1) J = HOIST(1)
      END DO
      HOIST = T
      END
C
C     STEPS: L stepped by 3, and the call adds 100 to it once.
      RECURSIVE INTEGER FUNCTION STEPS(K)
      INTEGER K, L, I, A(20), T, J
      SAVE L
      DATA A /20*1/
      IF (K .EQ. 1) THEN
         L = L + 100
         STEPS = 0
         RETURN
      END IF
      L = 0
      T = 0
      DO I = 1, 4
         L = L + 3
         IF (L .LE. 20) T = T + A(L) * L
         IF (I .EQ. 2) J = STEPS(1)
      END DO
      STEPS = T * 1000 + L
      END
C
C     TRIPS: the limit LIM is 4 when the loop begins; each deeper call sets it
C     to 1. The count runs from the value at the start.
      RECURSIVE INTEGER FUNCTION TRIPS(K)
      INTEGER K, LIM, I, N
      SAVE LIM
      LIM = 4
      N = 0
      DO I = 1, LIM
         N = N + 1
         IF (K .GT. 1 .AND. I .EQ. 1) N = N + TRIPS(K-1) * 10
         LIM = 1
      END DO
      TRIPS = N
      END
C
C     FRAME: locals in the frame, forwarded and in registers, through a call
C     that recurses to depth K.
      RECURSIVE INTEGER FUNCTION FRAME(K)
      INTEGER K, A, B, I, S
      A = K * 10
      B = A + 1
      S = 0
      DO I = 1, K
         S = S + A
         IF (I .EQ. 1 .AND. K .GT. 1) S = S + FRAME(K-1)
         S = S + B
      END DO
      FRAME = S + A + B
      END
