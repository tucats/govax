C     Every type and length a declaration can give (Table 2-1), and
C     each variable's place in $LOCAL at its natural alignment.
      SUBROUTINE TYPES(C, N)
      IMPLICIT LOGICAL*1 (B), DOUBLE PRECISION (D)
      IMPLICIT CHARACTER*8 (T-V)
      CHARACTER*(*) C
      BYTE B1
      INTEGER*2 I2
      INTEGER*4 I4
      LOGICAL*2 L2
      REAL*8 R8, R16*16
      COMPLEX X8
      COMPLEX*16 X16
      DOUBLE COMPLEX DC
      CHARACTER*5 S5, S7*7, SA(3)*2
      CHARACTER SC
      CHARACTER*(2+3) S
      INTEGER*2 IVEC*4(10)
      DIMENSION B2(3), DX(2), TV(2)
      SQ(Y) = Y*Y
      K = N
      END
C     IMPLICIT NONE, with everything declared.
      PROGRAM NONE
      IMPLICIT NONE
      INTEGER I
      REAL X
      I = 2
      X = I
      PRINT *, X
      END
