C     funcs.for's COMPLEX*16 functions in G_floating, with a REAL*8 function
C     in registers beside them for comparison, and a value only G_floating's
C     range holds.
      OPTIONS /G_FLOATING
      PROGRAM FUNCSG
      COMPLEX*16 ZSQ, Z
      REAL*8 DTWICE, D
      Z = (1D0, 2D0)
      Z = ZSQ(Z)
      PRINT *, Z
C     2**128 is past D_floating's range; four divisions by 2**32 bring it back
      Z = (4294967296D0, 0D0)
      Z = ZSQ(ZSQ(Z))
      D = 4294967296D0
      PRINT *, DREAL(Z) / D / D / D / D
      PRINT *, DTWICE(1.5D0)
      END

      OPTIONS /G_FLOATING
      COMPLEX*16 FUNCTION ZSQ(X)
      COMPLEX*16 X
      ZSQ = (0D0, 0D0)
      ZSQ = X * X
      END

      OPTIONS /G_FLOATING
      REAL*8 FUNCTION DTWICE(X)
      REAL*8 X
      DTWICE = X * 2
      END
