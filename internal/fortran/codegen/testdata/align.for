      PROGRAM ALIGNED
CDEC$ OPTIONS /ALIGN=RECORDS=NATURAL
      STRUCTURE /NAT/
        LOGICAL*1 F
        INTEGER*4 N
        REAL*16 Q
        COMPLEX C /(1.,2.)/
        UNION
          MAP
            BYTE A
            REAL*8 D /2.5/
          END MAP
          MAP
            UNION
              MAP
                BYTE B
                INTEGER*2 W /3/
              END MAP
            END UNION
          END MAP
        END UNION
        CHARACTER*3 T
      END STRUCTURE
CDEC$ END OPTIONS
      STRUCTURE /PAK/
        BYTE F
        INTEGER*4 N
      END STRUCTURE
CDEC$ OPTIONS /ALIGN
      STRUCTURE /SUB/
        BYTE F
        RECORD /PAK/ P(2)
        BYTE G
      END STRUCTURE
      COMMON /N/ B1, I1, C1, Z1
      COMMON /OCTA/ RN
      LOGICAL*1 B1
      CHARACTER C1
      COMPLEX Z1
      RECORD /NAT/ RN, RL(2)
      RECORD /SUB/ RS
CDEC$ OPTIONS /ALIGN=COMMONS=STANDARD
      COMMON /S/ B2, D2, Z2
      LOGICAL*1 B2
      REAL*8 D2
      COMPLEX Z2
CDEC$ END OPTIONS
CDEC$ END OPTIONS
      COMMON /P/ B3, I3
      LOGICAL*1 B3
      END
      SUBROUTINE PLAIN
      X = 1
      END
