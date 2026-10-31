C     Each length's instructions (Phase 78, subtask 2).
      PROGRAM SHORTS
      BYTE B, BV(3)
      LOGICAL*2 L2
      INTEGER*2 I2, J2, IV(3)
      INTEGER I4
      B = 255
      I2 = 0
      J2 = I2 + 1
      J2 = J2 + 1
      J2 = J2*I2
      I4 = I2
      I2 = I4
      X = I2
      J2 = X
      B = B .AND. L2
      L2 = L2 .XOR. L2
      L2 = L2 .EQV. L2
      I2 = .NOT. I2
      J2 = -I2
      I4 = IV(I4)
      IV(J2) = I2
      BV(I4) = B
      IF (B) I2 = 1
      IF (BV(I4)) I2 = 2
      IF (.NOT. L2) I2 = 3
      IF (B .AND. L2) I2 = 4
      IF (I2 .LT. 7) I2 = 5
      CALL SUB(I2, IV(I4), BV(I4), 5)
      PRINT *, B, L2, I2, IV(I4), IV
      DO 10 I2 = 1, J2
   10 CONTINUE
      END
      SUBROUTINE SUB(A, W, C, K)
      INTEGER*2 W(*)
      BYTE C
      W(K) = W(K+1)
      C = C
      END
