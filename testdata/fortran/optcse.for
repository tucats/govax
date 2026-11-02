      PROGRAM OPTCS
C     What common subexpressions and finding a place once must get right (Phase
C     84's subtask 6). Every value here is unknown to the compiler, set through a
C     CALL, so that no earlier pass settles it.
      INTEGER I, J, K, L, M, N, IV(5), LV(20), KE, TK, CV
      INTEGER LA, LB, N1, N2, N3
      INTEGER*2 IW(3), HW(2)
      EQUIVALENCE (KE, HW)
      REAL A(3,3), R3(3), C, P, Q, X, Y, Z, W, S, R, VV, RS
      INTEGER RN
      EQUIVALENCE (R, RN)
      REAL*8 DV(3)
      CHARACTER*8 T(3)
      CHARACTER*2 S2, S3
      LOGICAL HIT
      VOLATILE VV
      COMMON /CC/ CV
  100 FORMAT (1X, A, 3I8)
  110 FORMAT (1X, A, 3F8.2)
      CALL SET(I, 2)
      CALL SET(J, 3)
C
C     One place, found once: each operator, either side, and those that can't.
      A(I,J) = 10.0
      A(I,J) = A(I,J) + 2.5
      PRINT 110, 'ADD', A(2,3)
      A(I,J) = 2.0*A(I,J)
      PRINT 110, 'MUL', A(2,3)
      A(I,J) = A(I,J) - 5.0
      PRINT 110, 'SUB', A(2,3)
      A(I,J) = A(I,J)/4.0
      PRINT 110, 'DIV', A(2,3)
      A(I,J) = 1.0 - A(I,J)
      PRINT 110, 'SUBR', A(2,3)
      A(I,J) = 20.0/A(I,J)
      PRINT 110, 'DIVR', A(2,3)
      A(J,I) = 4.0
      A(I,J) = 1.0
      A(I,J) = A(I,J) + A(J,I)
      PRINT 110, 'OTHER', A(2,3)
      A(I,J) = A(I,J) + A(I,J)
      PRINT 110, 'TWICE', A(2,3)
C
C     In integers: adding and taking away 1, the logical operators.
      IV(I) = 12
      IV(I) = IV(I) + 1
      PRINT 100, 'INC', IV(2)
      IV(I) = IV(I) - 1
      PRINT 100, 'DEC', IV(2)
      IV(I) = 3*IV(I)
      IV(I) = IV(I)/5
      PRINT 100, 'MULDIV', IV(2)
      IV(I) = IEOR(IV(I), 5)
      PRINT 100, 'XOR', IV(2)
      IV(I) = IV(I) .OR. 9
      PRINT 100, 'OR', IV(2)
      IV(I) = IV(I) .AND. 6
      PRINT 100, 'AND', IV(2)
C
C     Other lengths, and dummy arguments of each form.
      IW(I) = 7
      IW(I) = IW(I) + 1
      PRINT 100, 'WORD', IW(2)
      DV(I) = 1.5D0
      DV(I) = DV(I) + 0.5D0
      PRINT 110, 'DOUBLE', DV(2)
      R3(1) = 1.0
      R3(2) = 2.0
      R3(3) = 3.0
      CALL BUMP(R3, 3, I)
      PRINT 110, 'DUMMY', R3(1), R3(2), R3(3)
      CALL BUMPF(R3, I)
      PRINT 110, 'FIXED', R3(1), R3(2), R3(3)
C
C     One value used three times.
      CALL SETR(C, 3.0)
      A(I,J) = 10.0
      A(J,J) = 20.0
      X = A(I,J)*C + 1.0
      Y = A(I,J)*C - 1.0
      Z = A(I,J)*C + 2.0
      PRINT 110, 'MERGED', X, Y, Z
C
C     Not the same value once what it reads has changed: the index,
      X = A(I,J)*C + 1.0
      I = I + 1
      Y = A(I,J)*C - 1.0
      Z = A(I,J)*C + 2.0
      W = A(I,J)*C + 3.0
      PRINT 110, 'INDEX', X, Y, Z
      PRINT 110, 'INDEX', W
      I = I - 1
C     the element,
      X = A(I,J)*C + 1.0
      A(2,3) = 100.0
      Y = A(I,J)*C - 1.0
      Z = A(I,J)*C + 2.0
      PRINT 110, 'ELEMENT', X, Y, Z
C     another variable sharing its storage,
      CALL SETR(R, 2.0)
      CALL SETR(RS, 3.0)
      X = R*RS + 1.0
      RN = 0
      Y = R*RS - 1.0
      Z = R*RS + 2.0
      PRINT 110, 'SHARED', X, Y, Z
C     COMMON changed by a call,
      CV = 4
      N1 = CV*J + 1
      CALL TWICE
      N2 = CV*J - 1
      N3 = CV*J + 2
      PRINT 100, 'COMMON', N1, N2, N3
C     and a local whose address a call is given.
      CALL SET(TK, 5)
      N1 = TK*J + 1
      CALL ADD1(TK)
      N2 = TK*J - 1
      N3 = TK*J + 2
      PRINT 100, 'TAKEN', N1, N2, N3
C
C     A call leaves alone what it can't reach.
      LA = I*2
      LB = I + 5
      N1 = LA*LB + 1
      CALL SET(K, 0)
      N2 = LA*LB - 1
      N3 = LA*LB + 2
      PRINT 100, 'LOCAL', N1, N2, N3
C
C     A place that holds the value, for as long as it does.
      LV(17) = 0
      LV(18) = 0
      LV(1) = 0
      LV(9) = 0
      K = J*5 + I
      LV(J*5 + I) = 99
      PRINT 100, 'HOLDER', K, LV(17)
      K = J*5 + I
      K = K + 1
      LV(J*5 + I) = 77
      PRINT 100, 'CHANGED', K, LV(17), LV(18)
      KE = J*5 + I
      HW(1) = 1
      LV(J*5 + I) = 55
      PRINT 100, 'HALF', KE, LV(17), LV(1)
      CALL SET(K, 5)
      K = K + I
      LV(K + I) = 11
      PRINT 100, 'ITSELF', K, LV(9)
C
C     Operands the other way round, and those that aren't the same.
      CALL SETR(P, 2.0)
      CALL SETR(Q, 5.0)
      X = P*Q + 1.0
      Y = Q*P - 1.0
      Z = P*Q*3.0
      PRINT 110, 'COMMUTE', X, Y, Z
      X = (P - Q) + 1.0
      Y = (Q - P) + 1.0
      PRINT 110, 'NOT', X, Y
C
C     Values inside values.
      L = (I + J)*(I + J) + (I + J)*(I + J)
      PRINT 100, 'NESTED', L
C
C     In a loop: a new value each trip.
      A(1,3) = 1.0
      A(2,3) = 2.0
      A(3,3) = 3.0
      S = 0.0
      DO 20 L = 1, 3
         S = S + A(L,J)*C + A(L,J)*C
   20 CONTINUE
      PRINT 110, 'LOOP', S
C
C     A condition's second operand may not run, so it can't be computed first.
      CALL SET(N, 0)
      CALL SET(M, 7)
      HIT = .FALSE.
      IF (N .NE. 0 .AND. (M/N .GT. 2 .OR. M/N .LT. -2)) HIT = .TRUE.
      IF (HIT) PRINT 100, 'NEVER'
      PRINT 100, 'NOFAULT', M
C     But it can use one already computed.
      CALL SET(N, 2)
      L = (M/N + I)*2
      IF (N .NE. 0 .AND. M/N + I .GT. 2) PRINT 100, 'USED', L
C
C     VOLATILE stays as written.
      VV = 2.0
      X = VV*C + 1.0
      Y = VV*C - 1.0
      Z = VV*C + 2.0
      PRINT 110, 'VOLATILE', X, Y, Z
C
C     A format's expression and the list share a value, not a temporary.
      CALL SET(K, 2)
      L = K*K + K*K + K*K*K
      PRINT 120, K*K + K*K, L
  120 FORMAT (1X, 'VFE', I<K*K + K*K>, I4)
C
C     Character values: the same subscripts, and a store ending them.
      T(2) = 'ABCDEFGH'
      S2 = T(I)(J:J+1)
      S3 = T(I)(J+1:J+2)
      PRINT 130, 'SUBSTR', S2, S3
  130 FORMAT (1X, A, 1X, A, 1X, A)
      T(I)(J:J) = 'X'
      HIT = T(I)(J:J+1) .EQ. 'XD'
      S2 = T(I)(J:J+1)
      PRINT 130, 'STORED', S2, T(I)(J:J+1)
      IF (HIT) PRINT 100, 'EQUAL'
      END
      SUBROUTINE SET(N, V)
      INTEGER N, V
      N = V
      END
      SUBROUTINE SETR(X, V)
      REAL X, V
      X = V
      END
      SUBROUTINE TWICE
      INTEGER CV
      COMMON /CC/ CV
      CV = CV*2
      END
      SUBROUTINE ADD1(N)
      INTEGER N
      N = N + 1
      END
      SUBROUTINE BUMP(Q, N, M)
      INTEGER N, M
      REAL Q(N)
      Q(M) = Q(M) + 1.0
      Q(M+1) = 2.0*Q(M+1)
      END
      SUBROUTINE BUMPF(Q, M)
      INTEGER M
      REAL Q(3)
      Q(2) = Q(2)*2.0
      Q(M) = Q(M) - 1.0
      Q(M+1) = Q(M+1)/2.0
      END
