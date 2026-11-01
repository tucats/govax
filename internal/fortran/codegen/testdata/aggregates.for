C     Records as wholes: assignment, arguments, COMMON, unformatted I/O, and
C     whole array fields.
      PROGRAM AGG
      STRUCTURE /DATE/
        LOGICAL*1 DAY /1/
        INTEGER*2 YEAR
      END STRUCTURE
      STRUCTURE /ITEM/
        CHARACTER*3 TAG
        RECORD /DATE/ WHEN
        INTEGER*4 QTY(3)
        CHARACTER*2 NAMES(2)
      END STRUCTURE
      RECORD /DATE/ D1, D2
      RECORD /ITEM/ ONE, STOCK(3)
      COMMON /BLK/ D2
      I = 2
      D1 = D2
      ONE.WHEN = D1
      STOCK(I) = ONE
      STOCK(2).WHEN = STOCK(I).WHEN
      CALL TAKE(ONE, STOCK(I), STOCK, ONE.WHEN)
      CALL NUMS(ONE.QTY, STOCK(I).QTY, %REF(ONE), %DESCR(ONE.QTY))
      CALL TEXTS(ONE.NAMES)
      WRITE (1) ONE, STOCK, STOCK(I), ONE.QTY
      READ (1) D1
      PRINT *, ONE.QTY, STOCK(I).QTY
      PRINT *, ONE.NAMES, %LOC(STOCK(I))
      END

      SUBROUTINE TAKE(R, E, LIST, W)
      STRUCTURE /DATE/
        LOGICAL*1 DAY
        INTEGER*2 YEAR
      END STRUCTURE
      STRUCTURE /ITEM/
        CHARACTER*3 TAG
        RECORD /DATE/ WHEN
        INTEGER*4 QTY(3)
        CHARACTER*2 NAMES(2)
      END STRUCTURE
      RECORD /ITEM/ R, E, LIST(*)
      RECORD /DATE/ W
      E = R
      LIST(2) = E
      W.YEAR = R.WHEN.YEAR
      R.QTY(2) = LIST(3).QTY(1)
      PRINT *, R.QTY
      CALL FURTHER(R, R.WHEN)
      END

      SUBROUTINE ADJ(T, N, M)
      STRUCTURE /P/
        CHARACTER*3 C
        INTEGER*4 V(2)
      END STRUCTURE
      RECORD /P/ T(N, M), H
      H = T(N, M)
      T(1, 2) = H
      T(N, 1).V(2) = T(1, M).V(N)
      PRINT *, T(N, 1).V, SIZEOF(T)
      WRITE (1) T
      END
