C     The floating types' instructions (Phase 78, subtask 3).
      PROGRAM FLOATS
      DOUBLE PRECISION D, E, DV(3), DF
      REAL*16 Q, QV(2)
      INTEGER*2 I2
      D = 0.1D0
      E = 0.1
      X = D
      D = X
      D = D + X*2
      E = (D + E)*(D - E)/(E + 1D0)
      Q = D
      Q = Q*Q + 1Q0
      Q = (Q + Q)*(Q - Q)
      Q = (Q + QV(I))*(Q - QV(I+1))
      D = Q
      I = D
      I2 = Q
      D = I2
      DV(I) = -DV(I+1)
      QV(I) = 0
      IF (D .LT. E) D = 2.5D0
      IF (Q .EQ. 0) Q = 1
      D = DF(D)
      CALL SUB(D, Q, 1D0, DV(I))
      PRINT *, D, Q, DV, QV(1), 3.5D0
      DO 10 D = 1, 2, 0.5D0
   10 CONTINUE
      END
      DOUBLE PRECISION FUNCTION DF(A)
      DOUBLE PRECISION A
      DF = A*A
      END
      OPTIONS /G_FLOATING
      SUBROUTINE SUB(A, B, C, V)
      REAL*8 A, C, V(*)
      REAL*16 B
      A = A + C*0.1D0 + V(2)
      B = A
      PRINT *, A
      END
