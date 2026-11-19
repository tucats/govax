C     Arrays of records and record fields with rank (docs/planning/PHASE-105 -
C     Fortran 90 derived types.md, subtask 5a), the Fortran 90 array forms
C     that derived types brought, in a VAX FORTRAN program: every record's
C     field, RA.A; a section of an array field, R.B(2:4); an array of
C     records assigned whole, as a section, and from one record; and a
C     CHARACTER field's substring, which a field's list still is.
      PROGRAM RECARR
      STRUCTURE /ITEM/
        INTEGER*2 CODE
        CHARACTER*6 NAME
        REAL QTY(3)
      END STRUCTURE
      RECORD /ITEM/ R, RA(4), RB(4)
      INTEGER I
      RA.CODE = (/ (10 * I, I = 1, 4) /)
      RA.NAME = 'BOLT'
      RA(3).NAME(5:6) = 'S2'
      R.QTY = (/ 1.5, 2.5, 3.5 /)
      R.QTY(2:3) = R.QTY(1:2) + 10.0
      R.CODE = 99
      R.NAME = 'NUT'
      RB = RA
      RB(2:3) = RB(1:2)
      RB(4) = R
      DO I = 1, 4
        RA(I).QTY = I
      END DO
      RA.QTY(2) = RA.QTY(1) * 2.0
      WRITE (6, 10) RA.CODE, RB.CODE
      WRITE (6, 20) (RB(I).NAME, I = 1, 4), RA(3).NAME(3:6)
      WRITE (6, 30) R.QTY, RA.QTY(2)
10    FORMAT (1X, 8I4)
20    FORMAT (1X, 4(A, 1X), A)
30    FORMAT (1X, 7F6.1)
      END
