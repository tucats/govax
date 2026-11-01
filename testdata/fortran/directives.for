CDEC$ IDENT 'DIRECTIVES V1.2'
CDEC$ IDENT 'IGNORED, AS THE SECOND'
CDEC$ TITLE 'THE GENERAL DIRECTIVES'
C     Section 10.3's directives: the attributes of common blocks' program
C     sections, seen in where the blocks land and what may be done to them.
C     A directive may stand before a unit, or anywhere in one, even before
C     the block's COMMON statement.
CDEC$ PSECT /B2/ ALIGN=9
      PROGRAM DIRECTIVES
      BYTE B1, B2, B3, B4, B5, B6
      INTEGER TABLE(3), LOCAL, BLANK
      COMMON /B1/ B1 /B2/ B2 /B3/ B3 /B4/ B4 /B5/ B5 /B6/ B6
      COMMON /FIXED/ TABLE
      COMMON /PRIV/ LOCAL
      COMMON BLANK
C     Each ALIGN=0 block starts right after the one before it, and the
C     others on their boundaries, past what the bytes before them leave.
CDEC$ PSECT /B1/ ALIGN=0
CDEC$ PSECT /B3/ ALIGN = 0
CDEC$ PSECT /B4/ ALIGN=4, GBL, WRT, SHR
CDEC$ PSECT /B5/ ALIGN=0
CDEC$ PSECT /B6/ ALIGN=3
CDEC$ PSECT /FIXED/ NOWRT, NOSHR
CDEC$ PSECT /PRIV/ LCL, NOSHR
CDEC$ PSECT // ALIGN=1
CDEC$ SUBTITLE 'PLACES'
      PRINT *, 'PAGE', MOD(%LOC(B2), 512)
      PRINT *, 'BYTE', %LOC(B3) - %LOC(B2)
      PRINT *, 'OCTA', MOD(%LOC(B4), 16)
      PRINT *, 'BYTE', %LOC(B5) - %LOC(B4)
      PRINT *, 'QUAD', MOD(%LOC(B6), 8)
      PRINT *, 'WORD', MOD(%LOC(BLANK), 2)
      DO 10 I = 1, 2
CDEC$ SUBTITLE 'A DIRECTIVE IN A LOOP'
         IF (I .EQ. 2) THEN
CDEC$ TITLE
            CALL READS
         END IF
10    CONTINUE
      LOCAL = 7
      B2 = 9
      CALL NOTES
      PRINT *, 'FIXED', TABLE
      PRINT '(A,Z8)', ' STORE AT ', %LOC(TABLE(2))
      CALL CHANGE
      PRINT *, 'NOT REACHED'
      END

C     Before the unit or in it, in any order: the same attributes either way.
CDEC$ PSECT /FIXED/ NOSHR, NOWRT
      SUBROUTINE READS
      INTEGER TABLE(3)
      COMMON /FIXED/ TABLE
      PRINT *, 'READ', TABLE(1) + TABLE(2) + TABLE(3)
      END

C     Writing a default out agrees with leaving it alone.
      SUBROUTINE NOTES
      BYTE B2
      STRUCTURE /S/
         INTEGER A
CDEC$ PSECT /B2/ ALIGN=9, GBL, WRT
         UNION
CDEC$ SUBTITLE 'A DIRECTIVE IN A UNION'
            MAP
               INTEGER B
            END MAP
         END UNION
      END STRUCTURE
      RECORD /S/ R
      COMMON /B2/ B2
      COMMON /PRIV/ LOCAL
CDEC$ PSECT /PRIV/ NOSHR, LCL
      R.A = LOCAL
      PRINT *, 'NOTES', R.A, B2
      END

      BLOCK DATA TABLES
      INTEGER TABLE(3)
CDEC$ PSECT /FIXED/ NOWRT, NOSHR
      COMMON /FIXED/ TABLE
      DATA TABLE /10, 20, 30/
      END

C     NOWRT means what it says: this store is an access violation.
      SUBROUTINE CHANGE
      INTEGER TABLE(3)
      COMMON /FIXED/ TABLE
CDEC$ PSECT /FIXED/ NOWRT, NOSHR
      TABLE(2) = 0
      END
CDEC$ TITLE 'AFTER THE LAST UNIT'
CDEC$ IDENT 'IGNORED TOO'
