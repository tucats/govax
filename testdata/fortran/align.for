C     /ALIGN's groups (docs/PHASE-82, Decision 8). Outside a group, as in V5.0,
C     everything is packed. Inside one, each field or common name goes to its
C     natural boundary, and a structure's length is a multiple of its own.
      PROGRAM ALIGNED
      STRUCTURE /PACKED/
        LOGICAL*1 F
        INTEGER*4 N
      END STRUCTURE
CDEC$ OPTIONS /ALIGN=RECORDS=NATURAL
C     F 0, N 4, G 8, D 16, H 24: 26 bytes, rounded up to the REAL*8's 8.
      STRUCTURE /NAT/
        LOGICAL*1 F
        INTEGER*4 N
        LOGICAL*1 G
        REAL*8 D
        INTEGER*2 H /7/
      END STRUCTURE
C     A COMPLEX is on an octaword, so the structure is too: 27 bytes, then 32.
      STRUCTURE /CX/
        LOGICAL*1 F
        COMPLEX C
        CHARACTER*3 T
      END STRUCTURE
C     The union's largest field is the INTEGER*4, so it starts at 4, and each
C     map's fields are placed from there: A 4, B 8, C 4. L is after its 8 bytes.
      STRUCTURE /UN/
        LOGICAL*1 F
        UNION
          MAP
            LOGICAL*1 A
            INTEGER*4 B /11/
          END MAP
          MAP
            INTEGER*2 C
          END MAP
        END UNION
        LOGICAL*1 L
      END STRUCTURE
C     A substructure goes on its structure's boundary, a packed one's too (N's 4),
C     though its own fields stay where they are.
      STRUCTURE /OUTER/
        LOGICAL*1 F
        RECORD /PACKED/ P
        LOGICAL*1 G
        RECORD /NAT/ Q(2)
      END STRUCTURE
CDEC$ END OPTIONS
C     Declared after the group, so packed again.
      STRUCTURE /AFTER/
        LOGICAL*1 F
        INTEGER*4 N
      END STRUCTURE
      RECORD /PACKED/ RP
      RECORD /NAT/ RN(3)
      RECORD /CX/ RC
      RECORD /UN/ RU
      RECORD /OUTER/ RO
      RECORD /AFTER/ RA
      INTEGER I5, I6
      LOGICAL*1 B1, B2, B3, B4
      INTEGER*4 I4
      CHARACTER C1
      REAL*8 D8, D2
      COMPLEX CX, CY
CDEC$ OPTIONS /ALIGN=COMMONS=NATURAL
      COMMON /CN/ B1, I4, C1, D8, CX
CDEC$ OPTIONS /ALIGN=COMMONS=STANDARD
      COMMON /CS/ B2, D2, CY
CDEC$ END OPTIONS
      COMMON /CN/ B3, I5
CDEC$ END OPTIONS
      COMMON /CP/ B4, I6
C     Each line's number is what the comment above says.
      PRINT *, 'PACKED', SIZEOF(RP), %LOC(RP.N) - %LOC(RP)
      PRINT *, 'NAT', SIZEOF(RN(1)), %LOC(RN(1).D) - %LOC(RN(1))
      PRINT *, 'STRIDE', %LOC(RN(2)) - %LOC(RN(1))
      PRINT *, 'BOUND', MOD(%LOC(RN(2).D), 8)
      IV = RN(3).H
      PRINT *, 'INIT', IV
      PRINT *, 'CX', SIZEOF(RC), %LOC(RC.C) - %LOC(RC)
      PRINT *, 'CXMOD', MOD(%LOC(RC), 16)
      PRINT *, 'UN', SIZEOF(RU), %LOC(RU.A) - %LOC(RU)
      PRINT *, 'UNC', %LOC(RU.C) - %LOC(RU)
      PRINT *, 'UNB', %LOC(RU.B) - %LOC(RU), %LOC(RU.L) - %LOC(RU), RU.B
      PRINT *, 'OUTER', SIZEOF(RO), %LOC(RO.P) - %LOC(RO)
      PRINT *, 'OUTERP', %LOC(RO.P.N) - %LOC(RO.P)
      PRINT *, 'OUTERG', %LOC(RO.G) - %LOC(RO)
      IV = RO.Q(2).H
      PRINT *, 'OUTERQ', %LOC(RO.Q(2).D) - %LOC(RO), IV
      PRINT *, 'AFTER', SIZEOF(RA)
      PRINT *, 'CN', %LOC(I4) - %LOC(B1), %LOC(C1) - %LOC(B1)
      PRINT *, 'CND', %LOC(D8) - %LOC(B1), MOD(%LOC(D8), 8)
      PRINT *, 'CNX', %LOC(CX) - %LOC(B1), MOD(%LOC(CX), 16)
      PRINT *, 'CNI', %LOC(I5) - %LOC(B1)
      PRINT *, 'CS', %LOC(D2) - %LOC(B2), %LOC(CY) - %LOC(B2)
      PRINT *, 'CP', %LOC(I6) - %LOC(B4)
      RN(2).D = 2.5
      RU.B = 300
      RC.T = 'ABC'
      PRINT *, 'VALUES', INT(RN(2).D * 2.), RU.B, RC.T
      CALL SECOND(RN(2))
      CALL THIRD
      END

C     This routine's $LOCAL follows the main program's in one section, so only
C     a boundary before them makes its offsets an address's.
      SUBROUTINE SECOND(R)
CDEC$ OPTIONS /ALIGN
      STRUCTURE /NAT/
        LOGICAL*1 F
        INTEGER*4 N
        LOGICAL*1 G
        REAL*8 D
        INTEGER*2 H
      END STRUCTURE
      STRUCTURE /CX/
        LOGICAL*1 F
        COMPLEX C
      END STRUCTURE
CDEC$ END OPTIONS
      RECORD /NAT/ R
      RECORD /CX/ RC
      RC.C = (1., 2.)
      PRINT *, 'SECOND', INT(R.D * 2.), MOD(%LOC(RC), 16),
     1  INT(REAL(RC.C))
      END

C     A routine with nothing aligned, in the same section.
      SUBROUTINE THIRD
      INTEGER I
      I = 3
      PRINT *, 'THIRD', I
      END
