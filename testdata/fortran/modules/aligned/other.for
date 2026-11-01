C     The same block's first names, under the same rule, at the same places.
      SUBROUTINE OTHER
CDEC$ OPTIONS /ALIGN=COMMONS=NATURAL
      COMMON /BLK/ B, D
      LOGICAL*1 B
      REAL*8 D
CDEC$ END OPTIONS
      PRINT *, 'OTHER', INT(D * 2.)
      D = 3.5
      END
