C     The intrinsic functions MTHRTL computes (Phase 78, subtask 9), called
C     with their arguments by reference: REAL*4 and REAL*8 values in R0
C     and R1, REAL*16 and COMPLEX*16 values through a hidden argument;
C     generic names and specific ones; ISHFT by a count that isn't
C     constant; functions of functions, in subscripts and conditions.
      PROGRAM MATHFN
      REAL X, Y, A(3)
      DOUBLE PRECISION D
      REAL*16 Q
      COMPLEX C
      COMPLEX*16 Z
      INTEGER I, J
      INTEGER*2 I2, J2
C     Square roots, exponentials, and logarithms in F, D, and H
      X = 2.0
      PRINT *, SQRT(X), EXP(1.0), ALOG(10.0), LOG10(X)
      D = 2D0
      PRINT *, SQRT(D), EXP(1D0), LOG(10D0)
      PRINT *, DLOG10(D), DSQRT(0.25D0), DEXP(-1D0)
      Q = 2
      PRINT *, SQRT(Q)
      PRINT *, QEXP(1Q0)
      PRINT *, LOG(10Q0)
      PRINT *, LOG10(Q)
C     Trigonometric functions in radians and in degrees
      PRINT *, SIN(1.0), COS(1.0), TAN(1.0), ATAN(1.0)
      PRINT *, SIND(30.0), COSD(60.0), TAND(45.0), ATAND(1.0)
      PRINT *, ASIN(0.5), ACOS(0.5), ASIND(0.5), ACOSD(0.5)
      PRINT *, ATAN2(1.0, -1.0), ATAN2D(-1.0, -1.0)
      PRINT *, DSIN(1D0), DCOS(1D0), DTAN(1D0)
      PRINT *, DATAN(1D0)*4, DSIND(30D0), DATAN2D(1D0, 1D0)
      PRINT *, QSIN(1Q0)
      PRINT *, QATAN(1Q0)*4
C     Hyperbolic functions
      PRINT *, SINH(1.0), COSH(1.0), TANH(1.0)
      PRINT *, DSINH(1D0), DCOSH(1D0), DTANH(1D0)
      PRINT *, QTANH(1Q0)
C     AINT, ANINT, and MOD of REALs
      PRINT *, AINT(2.7), AINT(-2.7), ANINT(2.5), ANINT(-2.5)
      PRINT *, DINT(-7.5D0), DNINT(7.5D0), QNINT(-0.5Q0)
      PRINT *, AMOD(7.5, 2.0), AMOD(-7.5, 2.0), DMOD(10D0, 3D0)
      PRINT *, MOD(10Q0, 3Q0)
C     Complex functions: F values in R0 and R1, D values through the
C     hidden argument
      C = (3.0, 4.0)
      PRINT *, CABS(C), CSQRT((-4.0, 0.0)), CEXP((0.0, 0.0))
      PRINT *, CLOG((-1.0, 0.0)), SQRT(C)
      PRINT *, CSIN(C)
      PRINT *, CCOS(C)
      Z = (3D0, 4D0)
      PRINT *, CDABS(Z), ABS(Z)
      PRINT *, SQRT(Z)
      PRINT *, CDLOG((0D0, 1D0))
      Z = CDEXP(Z)
      PRINT *, Z
C     ISHFT by a count that isn't constant: MTH$JISHFT and MTH$IISHFT
      I = 1
      J = 4
      PRINT *, ISHFT(I, J), ISHFT(-1, -J), ISHFT(I, 32), ISHFT(I, -J)
      I2 = 1
      J2 = 15
      PRINT *, ISHFT(I2, J2), ISHFT(-1, -J2), ISHFT(I2, J2 + 1)
C     A function of a function, in a subscript, and in a condition
      A(1) = 10.0
      A(2) = 20.0
      A(3) = 30.0
      PRINT *, SQRT(SQRT(16.0)), A(INT(SQRT(9.0))), EXP(LOG(1.0))
      IF (SIN(X) .GT. 0.9 .AND. COS(X) .LT. 0.0) PRINT *, 'BOTH'
      END
