C     mathfns.for's REAL*8 and COMPLEX*16 functions in G_floating, the
C     unit's OPTIONS statement saying so: MTH$GSQRT, MTH$CGSQRT, and their
C     kind, by generic and by specific names, and values past
C     D_floating's range.
      OPTIONS /G_FLOATING
      PROGRAM MATHFG
      DOUBLE PRECISION D, E
      COMPLEX*16 Z
      D = 2D0
      PRINT *, SQRT(D), EXP(1D0), LOG(10D0)
      PRINT *, DLOG10(D), DSQRT(0.25D0), DEXP(-1D0)
      PRINT *, DSIN(1D0), DCOS(1D0), DTAN(1D0)
      PRINT *, DATAN(1D0)*4, DSIND(30D0), DATAN2D(1D0, 1D0)
      PRINT *, DASIN(0.5D0), DACOSD(0.5D0), DATAN2(1D0, -1D0)
      PRINT *, DSINH(1D0), DCOSH(1D0), DTANH(1D0)
      PRINT *, DINT(-7.5D0), DNINT(7.5D0), DMOD(10D0, 3D0)
C     Past D_floating's range, which ends near 1.7D38
      E = 1D300
      PRINT *, LOG(E), SQRT(E), EXP(700D0)
C     COMPLEX*16
      Z = (3D0, 4D0)
      PRINT *, CDABS(Z), ABS(Z)
      PRINT *, SQRT(Z)
      PRINT *, CDLOG((0D0, 1D0))
      Z = CDEXP(Z)
      PRINT *, Z
      PRINT *, CDSIN((1D0, 0D0))
      END
