      PROGRAM OPTRG
C     What keeping values in registers must get right (Phase 84, subtask 9):
C     a variable's home written back on every way out of a loop, what a call
C     can and can't see, values of every width, and faults on registers.
      EXTERNAL TALLY
      INTEGER N, I, J, K, L, M, R, S, V, R2, R3, IC, NCOND, FSUM
      INTEGER IW, A, B, C, D, E, F, G, H, NW, MAT(2,3)
      INTEGER*2 HW(2), IS
      BYTE IB
      LOGICAL LG
      REAL X, RX, Y
      REAL*8 DV
      REAL*16 QV
      CHARACTER*5 LINE
      CHARACTER C1
      EQUIVALENCE (IW, HW)
      COMMON /V/ N
      COMMON /W/ IC
      COMMON /CONDS/ NCOND
  100 FORMAT (1X, A, 3I8)
  101 FORMAT (1X, A, I8, 1X, A)
  102 FORMAT (1X, A, F8.3)
  103 FORMAT (1X, A, F8.2, I8)
      CALL INIT
C
C     A way out by GO TO, with the values as they were there.
      DO 10 I = 1, N
         K = I*2
         IF (I .EQ. 3) GO TO 12
   10 CONTINUE
   12 PRINT 100, 'EXIT', I, K
C
C     The end of the loop, the DO variable one step past it.
      DO 14 I = 1, N
         K = K + I
   14 CONTINUE
      PRINT 100, 'END', I, K
C
C     A local kept from one call to the next, past a RETURN in a loop.
      CALL KEPT(2, R)
      CALL KEPT(5, R2)
      CALL KEPT(1, R3)
      PRINT 100, 'KEPT', R, R2, R3
C
C     Registers across a call of a routine that uses them all.
      CALL CALLER(R)
      PRINT 100, 'CALLER', R
C
C     COMMON changed by a routine called in the loop, and in a loop with no
C     call, then seen by a routine.
      IC = 0
      S = 0
      DO 16 I = 1, 3
         CALL BUMP
         S = S + IC
   16 CONTINUE
      DO 18 I = 1, 3
         IC = IC + 10
   18 CONTINUE
      CALL PEEK(K)
      PRINT 100, 'COMMON', S, K
C
C     Two names for one place.
      IW = 0
      DO 20 I = 1, 3
         IW = IW + 1
         HW(2) = I
   20 CONTINUE
      PRINT 100, 'EQUIV', IW
C
C     Dummy arguments: a constant that is only read, one written only if asked,
C     one not passed at all, one written on every trip.
      CALL RO(7, 3, R)
      PRINT 100, 'RO', R
      CALL MAYBE(5, 2, .FALSE., R)
      V = 5
      CALL MAYBE(V, 2, .TRUE., R2)
      PRINT 100, 'MAYBE', R, R2, V
      CALL OPT(0, , R)
      CALL OPT(1, 4, R2)
      PRINT 100, 'OPT', R, R2
      V = 0
      CALL OUT(V, 4)
      PRINT 100, 'OUT', V
C
C     A variable format expression reads what the loop sets.
      DO 22 I = 1, 2
         NW = I + 2
         PRINT 104, I
  104    FORMAT (1X, I<NW>)
   22 CONTINUE
C
C     More variables than registers.
      A = 0
      B = 0
      C = 0
      D = 0
      E = 0
      F = 0
      G = 0
      H = 0
      DO 24 I = 1, 3
         A = A + 1
         B = B + 2
         C = C + 3
         D = D + 4
         E = E + 5
         F = F + 6
         G = G + 7
         H = H + 8
   24 CONTINUE
      PRINT 100, 'EIGHT', A + B + C + D + E + F + G + H
C
C     Every width and type.
      DV = 1.5D0
      QV = 1.0Q0
      IS = 32000
      IB = 100
      LG = .FALSE.
      S = 0
      DO 26 I = 1, 3
         DV = DV*2.0D0
         QV = QV + QV*0.5Q0
         LG = .NOT. LG
         IF (LG) S = S + I
   26 CONTINUE
      DO 28 I = 1, 2
         IS = IS + 1000
   28 CONTINUE
      DO 29 I = 1, 1
         IB = IB + 100
   29 CONTINUE
      PRINT 102, 'DOUBLE', DV
      PRINT 102, 'QUAD', QV
      PRINT 100, 'WORD', IS
      PRINT 100, 'BYTE', IB
      PRINT 100, 'LOGICAL', S
C
C     One character, unsigned, in a loop; by address after it.
      LINE = 'ABCDE'
      LINE(2:2) = CHAR(200)
      M = 0
      DO 30 I = 1, N
         C1 = LINE(I:I)
         IF (C1 .GT. CHAR(127)) M = M + 1
   30 CONTINUE
      PRINT 101, 'CHARS', M, C1
C
C     Ways out of a computed GO TO and an arithmetic IF.
      M = 0
      DO 40 I = 1, N
         K = I*2
         GO TO (41, 42), I-2
         GO TO 40
   41    M = K + 1
         GO TO 40
   42    GO TO 45
   40 CONTINUE
   45 PRINT 100, 'SWITCH', I, K, M
      DO 50 I = 1, N
         K = I - 3
         IF (K) 50, 52, 50
   50 CONTINUE
   52 PRINT 100, 'SIGN', I, K
C
C     A function's value, and a routine with ENTRY statements.
      PRINT 100, 'FSUM', FSUM(N)
      CALL E1(2, R)
      CALL E2(3, R2)
      PRINT 100, 'ENTRY', R, R2
C
C     An adjustable array, and a dummy's element by a constant subscript.
      CALL MATRIX(MAT, 2, 3, R, R2)
      PRINT 100, 'MATRIX', R, R2
C
C     A CHARACTER dummy's text, its pointer in a register.
      CALL TEXT('AXX', R)
      PRINT 100, 'TEXT', R
C
C     A reserved operand in a register: LIB$FIXUP_FLT changes the register.
      CALL LIB$ESTABLISH(TALLY)
      RX = 0.0
      DO 60 I = 1, 3
         X = '00008000'X
         RX = RX + X
   60 CONTINUE
      CALL LIB$REVERT
      PRINT 103, 'FIXUP', RX, NCOND
C
C     A reserved operand in a variable the loop doesn't use: copying it to a
C     register and back must not fault.
      X = '00008000'X
      K = 0
      DO 70 I = 1, N
         IF (I .GT. 100) Y = X + 1.0
         K = K + 1
   70 CONTINUE
      PRINT 100, 'BITS', K
      END
C
      SUBROUTINE INIT
      INTEGER N
      COMMON /V/ N
      N = 5
      END
C
      SUBROUTINE KEPT(N, R)
      INTEGER N, R, L, I
      DATA L /100/
      DO 10 I = 1, N
         L = L + 1
         IF (MOD(L, 4) .EQ. 0) THEN
            R = L
            RETURN
         END IF
   10 CONTINUE
      R = L
      END
C
      SUBROUTINE CALLER(R)
      INTEGER R, I, J, A, B, C, D, S
      A = 0
      B = 0
      C = 0
      D = 0
      S = 0
      DO 10 I = 1, 4
         J = I*10
         A = A + 1
         B = B + 2
         C = C + 3
         D = D + 4
         CALL BUSY
         S = S + J + A + B + C + D
   10 CONTINUE
      R = S
      END
C
      SUBROUTINE BUSY
      INTEGER K, P, Q, U, V, W, X, Y, Z
      DO 10 K = 1, 5
         P = P + 1
         Q = Q + 2
         U = U + 3
         V = V + 4
         W = W + 5
         X = X + 6
         Y = Y + 7
         Z = Z + 8
   10 CONTINUE
      END
C
      SUBROUTINE BUMP
      INTEGER IC
      COMMON /W/ IC
      IC = IC + 1
      END
C
      SUBROUTINE PEEK(K)
      INTEGER K, IC
      COMMON /W/ IC
      K = IC
      END
C
      SUBROUTINE RO(K, M, R)
      INTEGER K, M, R, S, I
      S = 0
      DO 10 I = 1, M
         S = S + K
   10 CONTINUE
      R = S
      END
C
      SUBROUTINE MAYBE(K, M, W, R)
      INTEGER K, M, R, S, I
      LOGICAL W
      S = 0
      DO 10 I = 1, M
         IF (W) K = K + 1
         S = S + K
   10 CONTINUE
      R = S
      END
C
      SUBROUTINE OPT(N, K, R)
      INTEGER N, K, R, S, I
      S = 0
      DO 10 I = 1, 3
         IF (N .GT. 0) S = S + K
   10 CONTINUE
      R = S
      END
C
      SUBROUTINE OUT(K, M)
      INTEGER K, M, I
      DO 10 I = 1, M
         K = I
   10 CONTINUE
      END
C
      INTEGER FUNCTION FSUM(N)
      INTEGER N, I
      FSUM = 0
      DO 10 I = 1, N
         FSUM = FSUM + I
   10 CONTINUE
      END
C
      SUBROUTINE E1(K, R)
      INTEGER K, R, L, S, I
      L = K
      ENTRY E2(K, R)
      S = 0
      DO 10 I = 1, K
         S = S + L
   10 CONTINUE
      R = S
      END
C
      SUBROUTINE MATRIX(AR, N, M, R, R2)
      INTEGER N, M, AR(N,M), R, R2, S, I, J
      DO 20 J = 1, M
         DO 10 I = 1, N
            AR(I,J) = 10*J + I
   10    CONTINUE
   20 CONTINUE
      S = 0
      DO 40 J = 1, M
         DO 30 I = 1, N
            S = S + AR(I,J)
   30    CONTINUE
         R2 = AR(2,1)
   40 CONTINUE
      R = S
      END
C
      SUBROUTINE TEXT(T, R)
      CHARACTER*(*) T
      INTEGER R, N, I
      N = 0
      DO 10 I = 1, 3
         IF (T(I:I) .EQ. 'X') N = N + 1
   10 CONTINUE
      R = N
      END
C
      INTEGER FUNCTION TALLY(SIGARGS, MECHARGS)
      INTEGER SIGARGS(*), MECHARGS(*), NCOND, LIB$FIXUP_FLT
      COMMON /CONDS/ NCOND
      NCOND = NCOND + 1
      TALLY = LIB$FIXUP_FLT(SIGARGS, MECHARGS, 1.5)
      END
