C     mixed.for in G_floating, each unit's OPTIONS statement saying so:
C     REAL*8 values with G's 15 digits, and COMPLEX*8 with REAL*8 a
C     COMPLEX*16 in G_floating.
      OPTIONS /G_FLOATING
      PROGRAM MIXED
      BYTE B
      LOGICAL*4 L
      INTEGER*2 I2, IV(5)
      INTEGER*4 I4, K, N
      REAL*4 R
      REAL*8 D, DV(5), POLYD
      REAL*16 Q
      COMPLEX*8 C, POLYC
      COMPLEX*16 Z
      DATA IV /1, 2, 3, 4, 5/
C     A value widened through every type.
      B = -7
      I2 = B
      I4 = I2
      R = I4
      D = R
      Q = D
      C = Q
      Z = C
      PRINT *, B, I2, I4, R
      PRINT *, D
      PRINT *, Q
      PRINT *, C, Z
C     And narrowed: reals round, integers truncate, a byte keeps the low
C     eight bits.
      Z = (1234.5678D0, -1D0)
      C = Z
      Q = Z
      D = Q
      R = Z
      I4 = R
      I2 = -I4*3
      B = I2
      PRINT *, C
      PRINT *, D, R
      PRINT *, I4, I2, B
C     The result has the higher operand's type.
      I2 = 3
      I4 = 100000
      R = 0.5
      D = 0.25D0
      Q = 3
      L = .TRUE.
      PRINT *, I2/2*R, I2*I4, I4/I2, R + D
      PRINT *, I2 + D, R*Q
      PRINT *, L + I2, L*I4, (I2 + R)*(0.0, 2.0)
      PRINT *, C*D, Z + Q
C     A REAL*4 constant keeps its digits in a REAL*8 expression; a REAL*4
C     variable has only its own.
      R = 0.3333333
      PRINT *, 1D0 + 0.3333333, 1D0 + R
C     Mixed types as subscripts, DO limits, and arguments.
      DO 10 K = 1, 5
         DV(K) = IV(6-K)*0.5
   10 CONTINUE
      R = 2.75
      PRINT *, IV(R), DV(IV(2)*R), DV(R+D)
      N = 0
      DO 20 K = 1, R*2
         N = N + IV(K)
   20 CONTINUE
      PRINT *, N
      CALL SUMS(IV, DV, 5, D)
      PRINT *, D
C     Mixed types in conditions.
      I2 = 2
      IF (I2 .EQ. 2.0D0) PRINT *, 'I2 EQ 2.0D0'
      IF (DV(5) .LT. I2/4) PRINT *, 'WRONG'
      IF (R .GT. 2.7499999D0 .AND. R .LT. 2.7500001Q0) PRINT *, 'R OK'
      IF ((2.0, 0.0) .EQ. I2) PRINT *, 'COMPLEX EQ'
C     One polynomial in three types: 2x**3 - 3x**2 + 4x - 5.
      PRINT *, POLYD(1.5D0), POLYD(-2D0)
      CALL POLYQ(0.1Q0, Q)
      PRINT *, Q
      PRINT *, POLYC((1.0, 1.0)), POLYC((0.0, -2.0))
      END
C     The sums of an INTEGER*2 array and a REAL*8 array, added into a
C     REAL*8.
      OPTIONS /G_FLOATING
      SUBROUTINE SUMS(IV, DV, N, TOTAL)
      INTEGER*2 IV(*)
      REAL*8 DV(*), TOTAL
      TOTAL = 0
      DO 10 K = 1, N
         TOTAL = TOTAL + IV(K) + DV(K)
   10 CONTINUE
      END
      OPTIONS /G_FLOATING
      REAL*8 FUNCTION POLYD(X)
      REAL*8 X
      POLYD = ((2*X - 3)*X + 4)*X - 5
      END
C     A REAL*16 function is Phase 79's, so a subroutine.
      OPTIONS /G_FLOATING
      SUBROUTINE POLYQ(X, P)
      REAL*16 X, P
      P = ((2*X - 3)*X + 4)*X - 5
      END
      OPTIONS /G_FLOATING
      COMPLEX FUNCTION POLYC(X)
      COMPLEX X
      POLYC = ((2*X - 3)*X + 4)*X - 5
      END
