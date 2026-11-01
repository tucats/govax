C     Records and scalar field references (Phase 82, subtask 3): records
C     with their structures' initial values, a union's two maps, fields of
C     an array of records with subscripts that aren't constant, an array
C     field, a substructure, substrings of a field, fields as arguments, a
C     COMPLEX field, and fields as an internal file and in I/O lists.
      PROGRAM RECORDS
      STRUCTURE /DATE/
        LOGICAL*1 DAY /1/, MONTH /1/
        INTEGER*2 YEAR /1988/
      END STRUCTURE
      STRUCTURE /ITEM/
        CHARACTER*5 NAME /'NONE'/
        RECORD /DATE/ WHEN
        INTEGER*4 QTY(3)
        UNION
          MAP
            INTEGER*4 WHOLE /-1/
          END MAP
          MAP
            INTEGER*2 LOW, HIGH
          END MAP
        END UNION
        STRUCTURE PRICE
          REAL*4 EACH
          COMPLEX*8 Z
        END STRUCTURE
      END STRUCTURE
      RECORD /DATE/ TODAY
      RECORD /ITEM/ ONE, STOCK(4)
      INTEGER I, J, TOTAL
      LATER(X) = X + TODAY.YEAR
C     Every record of a structure starts with its initial values.
      PRINT *, TODAY.DAY, TODAY.MONTH, TODAY.YEAR
      PRINT *, STOCK(3).NAME, '|', STOCK(3).WHEN.YEAR, STOCK(3).WHOLE
C     A union's maps share their bytes.
      ONE.WHOLE = 65538
      PRINT *, ONE.LOW, ONE.HIGH
      ONE.HIGH = 0
      PRINT *, ONE.WHOLE
C     Fields of an array of records, and an array field, by subscripts
C     that aren't constant.
      DO I = 1, 4
        STOCK(I).NAME = 'ITEM' // CHAR(ICHAR('0') + I)
        STOCK(I).WHEN.DAY = I
        STOCK(I).WHEN.YEAR = 1990 + I
        DO J = 1, 3
          STOCK(I).QTY(J) = 10*I + J
          ONE.QTY(J) = J*J
        END DO
        STOCK(I).PRICE.EACH = 1.5 * I
      END DO
      TOTAL = 0
      DO I = 1, 4
        DO J = 1, 3
          TOTAL = TOTAL + STOCK(I).QTY(J)
        END DO
      END DO
      PRINT *, TOTAL, ONE.QTY(1) + ONE.QTY(2) + ONE.QTY(3)
      PRINT *, (STOCK(I).NAME, STOCK(I).WHEN.YEAR, I = 1, 4)
      PRINT *, STOCK(2).PRICE.EACH * STOCK(4).QTY(3), STOCK(4).WHEN.DAY
C     Substrings of a field, and fields as arguments.
      STOCK(1).NAME(1:1) = 'J'
      PRINT *, STOCK(1).NAME, ' ', STOCK(2).NAME(5:5)
      CALL TWICE(STOCK(2).QTY(2))
      CALL RENAME(ONE.NAME)
      PRINT *, STOCK(2).QTY(2), ' ', ONE.NAME, '|'
C     A COMPLEX field.
      ONE.PRICE.Z = (1.0, 2.0)
      ONE.PRICE.Z = ONE.PRICE.Z * (0.0, 1.0)
      PRINT *, ONE.PRICE.Z
C     A field as an internal file, and read into.
      WRITE (ONE.NAME, '(I5)') 12345
      READ (ONE.NAME, '(I5)') STOCK(3).QTY(1)
      PRINT *, ONE.NAME, STOCK(3).QTY(1) + 1
C     A statement function's expression and argument; a COMPLEX field of
C     an array of records; a comparison; IOSTAT=; a DO limit.
      I = 2
      PRINT *, LATER(STOCK(I).PRICE.EACH)
      STOCK(I).PRICE.Z = (1.0, 2.0)
      STOCK(I).PRICE.Z = STOCK(I).PRICE.Z * 2.0
      PRINT *, STOCK(I).PRICE.Z
      IF (STOCK(I).NAME .EQ. 'ITEM2') PRINT *, 'FOUND'
      READ (ONE.NAME, '(I5)', IOSTAT=ONE.QTY(1)) ONE.QTY(2)
      PRINT *, ONE.QTY(1), ONE.QTY(2)
      TOTAL = 0
      DO J = 1, ONE.LOW
        TOTAL = TOTAL + 1
      END DO
      PRINT *, TOTAL
      END

      SUBROUTINE TWICE(N)
      N = 2*N
      END

      SUBROUTINE RENAME(S)
      CHARACTER*(*) S
      S = 'NEW'
      END
