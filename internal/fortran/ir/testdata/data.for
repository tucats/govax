C     Initial values in $LOCAL and in common blocks: gaps, repeats, floating
C     values in hexadecimal, text, and overlays by EQUIVALENCE.
      PROGRAM DATAS
      EXTERNAL BLKDAT
      INTEGER A(12), B(5), K/5/
      REAL X/1.5/, R(2)
      DOUBLE PRECISION D
      INTEGER*2 W
      LOGICAL*1 L
      CHARACTER*6 STARS
      CHARACTER*3 C
      INTEGER CI
      EQUIVALENCE (C, CI)
      DATA A /12*0/, B(2), B(4) /7, 8/, R /2*0.0/
      DATA D /1D0/, W /-2/, L /.TRUE./, STARS /'****'/, C(2:3) /'XY'/
      COMMON /SHARED/ I, J, Y
      DATA J /3/
      K = K + I
      PRINT *, K, X, B(2), Y
      END

      BLOCK DATA BLKDAT
      COMMON /SHARED/ I, J, Y
      COMMON /OTHER/ DBL(2)
      DOUBLE PRECISION DBL
      DATA Y /2.5/, DBL /2*0D0/
      END
