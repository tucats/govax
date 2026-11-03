C     What gfortran's build of DUNGEON gets in place of VAX FORTRAN's: the same
C     fixed moment as ../fixtime.for, and RAN as MTH$RANDOM's manual gives it,
C     SEED = 69069 * SEED + 1, the result the seed's high 24 bits as a fraction.
C
      SUBROUTINE XIDATE(I, J, K)
      INTEGER I, J, K
      I = 3
      J = 14
      K = 86
      END
C
      SUBROUTINE XITIME(H, M, S)
      INTEGER H, M, S
      H = 12
      M = 30
      S = 15
      END
C
      REAL FUNCTION XRAN(SEED)
      INTEGER SEED, I
      SEED = SEED * 69069 + 1
      I = ISHFT(SEED, -8)
      XRAN = REAL(I) * 2.0 ** (-24)
      END
