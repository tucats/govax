C     Intrinsic functions MTHRTL computes, called with their arguments by
C     reference: a value in R0, in R0 and R1, or through a hidden first
C     argument for REAL*16 and COMPLEX*16; ISHFT by a count that isn't
C     constant; intrinsic functions' names passed as arguments, MTHRTL's
C     routines and FTNRTL's; and a G_floating unit's routines.
      PROGRAM MATHS
      INTRINSIC SIN, DSQRT, IABS, LEN, CHAR, QEXT
      INTEGER I, J
      INTEGER*2 I2, J2
      REAL X, Y
      DOUBLE PRECISION D
      REAL*16 Q
      COMPLEX C
      COMPLEX*16 Z
      X = SQRT(X) + SIN(X + 1.0) * ATAN2D(Y, X)
      D = DLOG10(D) + MOD(D, 2.0D0) + ABS(C)
      Q = EXP(Q) + QMAX1(Q, 1Q0)
      C = CSQRT(C) * CEXP(C)
      Z = CDLOG(Z)
      X = CABS(C) + ABS(Z) + AINT(X) + ANINT(Y)
      I = ISHFT(I, J) + ISHFT(I2, J2)
      CALL APPLY(SIN, DSQRT, IABS, LEN, CHAR, QEXT)
      CALL GUNIT
      END
C
      OPTIONS /G_FLOATING
      SUBROUTINE GUNIT
      INTRINSIC DSIN, DBLEQ
      DOUBLE PRECISION D
      COMPLEX*16 Z
      D = DSIN(D) + DATAN2D(D, 1D0) + CDABS(Z)
      Z = SQRT(Z)
      CALL APPLY(DSIN, DBLEQ)
      END
