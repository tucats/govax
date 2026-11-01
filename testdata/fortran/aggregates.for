C     Records as wholes (Phase 82, subtask 4): aggregate assignment, records
C     as arguments (one element, an array, an adjustable array, a substructure,
C     fields at odd offsets, ENTRY), %LOC and %REF, records in COMMON with
C     their initial values, records in unformatted I/O, whole array fields, and
C     a record too long for one MOVC3.
      PROGRAM AGGREG
      STRUCTURE /DATE/
        LOGICAL*1 DAY /1/, MONTH /2/
        INTEGER*2 YEAR /1988/
      END STRUCTURE
      STRUCTURE /ITEM/
        CHARACTER*3 TAG /'NEW'/
        RECORD /DATE/ WHEN
        INTEGER*4 QTY(3)
      END STRUCTURE
      STRUCTURE /BIG/
        INTEGER*4 FIRST
        CHARACTER*1000 MIDDLE(70)
        INTEGER*4 LAST
      END STRUCTURE
      STRUCTURE /GROUP/
        CHARACTER*2 NAMES(3)
        RECORD /DATE/ DATES(2)
      END STRUCTURE
      STRUCTURE /SHARED/
        INTEGER*4 COUNT /7/
        CHARACTER*4 LABEL /'BLK '/
      END STRUCTURE
      RECORD /DATE/ TODAY, THEN
      RECORD /ITEM/ ONE, TWO, THREE, HOLD, STOCK(3), TABLE(2,3)
      RECORD /BIG/ B1, B2
      RECORD /GROUP/ G
      RECORD /SHARED/ S
      COMMON /BLK/ S
      COMMON /PLAIN/ K1, K2
      DATA K1 /3/, K2 /4/
      INTEGER I, J, TOTAL, COUNTQ, SUMTAB
      INTEGER*4 ADDR1, ADDR2
      CHARACTER*6 TXT
C     Aggregate assignment: a record, an element, a substructure.
      THEN.YEAR = 2000
      THEN.DAY = 9
      TODAY = THEN
      PRINT *, TODAY.DAY, TODAY.MONTH, TODAY.YEAR
      ONE.WHEN = TODAY
      ONE.TAG = 'ONE'
      ONE.QTY(1) = 11
      ONE.QTY(2) = 12
      ONE.QTY(3) = 13
      TWO = ONE
      STOCK(2) = TWO
      I = 2
      STOCK(I+1) = STOCK(I)
      STOCK(3).QTY(3) = 99
      PRINT *, STOCK(3).TAG, STOCK(3).WHEN.YEAR, STOCK(3).QTY(1),
     1  STOCK(3).QTY(3), STOCK(2).QTY(3)
      PRINT *, STOCK(1).TAG, STOCK(1).WHEN.YEAR
      THEN = STOCK(3).WHEN
      PRINT *, THEN.YEAR
C     A record as an argument; its fields are at odd offsets (TAG is 3 bytes).
      CALL DOUBLE(ONE)
      PRINT *, ONE.QTY(1), ONE.QTY(2), ONE.QTY(3), ' ', ONE.TAG
      CALL DOUBLE(STOCK(2))
      PRINT *, STOCK(2).QTY(1)
C     A substructure, and an array of records by its first element.
      CALL NEXTYR(ONE.WHEN)
      PRINT *, ONE.WHEN.YEAR
      STOCK(1).QTY(1) = 5
      PRINT *, COUNTQ(STOCK, 3)
      PRINT *, COUNTQ(STOCK(2), 2)
C     An adjustable array of records of two dimensions.
      DO J = 1, 3
        DO I = 1, 2
          TABLE(I,J).QTY(2) = 10*I + J
        END DO
      END DO
      I = SUMTAB(TABLE, 2, 3, 2)
      PRINT *, I
C     An ENTRY's dummy record.
      CALL SETYR(TODAY, 1999)
      PRINT *, TODAY.YEAR
      CALL GETYR(I, TODAY)
      PRINT *, I
C     %LOC and %REF of a whole record.
      ADDR1 = %LOC(ONE)
      ADDR2 = %LOC(ONE.WHEN)
      PRINT *, ADDR2 - ADDR1
      ADDR2 = %LOC(STOCK(2))
      ADDR1 = %LOC(STOCK)
      PRINT *, ADDR2 - ADDR1
      CALL DOUBLE(%REF(TWO))
      PRINT *, TWO.QTY(2)
      PRINT *, SIZEOF(ONE), SIZEOF(STOCK), SIZEOF(ONE.WHEN),
     1  SIZEOF(STOCK(1)), SIZEOF(ONE.QTY), SIZEOF(G.DATES)
C     A record in COMMON, with its structure's initial values, which BUMP's
C     declaration repeats, and DATA it repeats too.
      PRINT *, S.COUNT, S.LABEL, '|'
      CALL BUMP
      PRINT *, S.COUNT, S.LABEL, '|'
      PRINT *, K1, K2
C     Unformatted I/O: a record, an element, a substructure, an array of
C     records, and an array field, in one I/O record.
      OPEN (UNIT=1, FILE='AGG.DAT', STATUS='NEW', FORM='UNFORMATTED')
      WRITE (1) ONE, STOCK(3), TODAY, STOCK, ONE.QTY
      WRITE (1) THEN
      REWIND (1)
      TWO.TAG = 'OLD'
      READ (1) TWO, STOCK(1), THEN
      PRINT *, TWO.TAG, TWO.QTY(1), STOCK(1).QTY(3), THEN.YEAR
      READ (1) TODAY
      PRINT *, TODAY.YEAR
      CLOSE (UNIT=1, STATUS='DELETE')
C     Whole array fields: an argument, in a list, read into, CHARACTER too,
C     and an internal file.
      PRINT *, ISUM(ONE.QTY, 3)
      PRINT *, ONE.QTY
      TXT = ' 1 2 3'
      READ (TXT, '(3I2)') ONE.QTY
      PRINT *, ONE.QTY
      PRINT *, ISUM(STOCK(2).QTY, 3)
      G.NAMES(1) = 'AB'
      G.NAMES(2) = 'CD'
      G.NAMES(3) = 'EF'
      PRINT *, G.NAMES
      CALL SHOW(G.NAMES)
      WRITE (G.NAMES, '(A2)') 'X1', 'X2', 'X3'
      PRINT *, G.NAMES(1), G.NAMES(3)
      G.DATES(2).YEAR = 1234
      CALL NEXTYR(G.DATES(2))
      PRINT *, G.DATES(2).YEAR, G.DATES(1).YEAR
      PRINT *, MAXYR(G.DATES, 2)
C     Whole array fields of an element chosen when the program runs, and of
C     a dummy record; a record passed on; a dummy assigned; an adjustable
C     array in an unformatted list, with its size.
      I = 2
      PRINT *, STOCK(I).QTY
      READ (TXT, '(3I2)') STOCK(I).QTY
      PRINT *, ISUM(STOCK(I).QTY, 3)
      THREE = TWO
      CALL PASSON(THREE, HOLD, 3)
      PRINT *, HOLD.TAG, HOLD.WHEN.YEAR, HOLD.QTY(3)
      CALL SAVEIT(STOCK, 3)
C     A record too long for one MOVC3.
      B1.FIRST = 1
      B1.MIDDLE(1) = 'START'
      B1.MIDDLE(70)(999:1000) = 'ZZ'
      B1.LAST = 2
      B2 = B1
      PRINT *, B2.FIRST, B2.LAST, B2.MIDDLE(1)(1:5),
     1  B2.MIDDLE(70)(999:1000)
      END

      SUBROUTINE DOUBLE(R)
      STRUCTURE /D/
        LOGICAL*1 D, M
        INTEGER*2 Y
      END STRUCTURE
      STRUCTURE /I/
        CHARACTER*3 T
        RECORD /D/ W
        INTEGER*4 Q(3)
      END STRUCTURE
      RECORD /I/ R
      DO K = 1, 3
        R.Q(K) = 2*R.Q(K)
      END DO
      R.T = 'DBL'
      END

      SUBROUTINE NEXTYR(W)
      STRUCTURE /D/
        LOGICAL*1 D, M
        INTEGER*2 Y
      END STRUCTURE
      RECORD /D/ W
      W.Y = W.Y + 1
      END

      INTEGER FUNCTION COUNTQ(LIST, N)
      STRUCTURE /D/
        LOGICAL*1 D, M
        INTEGER*2 Y
      END STRUCTURE
      STRUCTURE /I/
        CHARACTER*3 T
        RECORD /D/ W
        INTEGER*4 Q(3)
      END STRUCTURE
      RECORD /I/ LIST(*)
      COUNTQ = 0
      DO K = 1, N
        COUNTQ = COUNTQ + LIST(K).Q(1)
      END DO
      END

      INTEGER FUNCTION SUMTAB(T, N, M, K)
      STRUCTURE /D/
        LOGICAL*1 D, M
        INTEGER*2 Y
      END STRUCTURE
      STRUCTURE /I/
        CHARACTER*3 T
        RECORD /D/ W
        INTEGER*4 Q(3)
      END STRUCTURE
      RECORD /I/ T(N, M), HOLD
      SUMTAB = 0
      DO J = 1, M
        DO I = 1, N
          HOLD = T(I, J)
          SUMTAB = SUMTAB + HOLD.Q(K)
          T(I, J).Q(1) = I + 100*J
        END DO
      END DO
C     The field of an element whose place is computed from the descriptor,
C     and its array field as a whole.
      PRINT *, T(2, 3).Q(1), T(N, M).Q(K), ISUM(T(N, 1).Q, 2)
      END

      SUBROUTINE SETYR(W, Y)
      STRUCTURE /D/
        LOGICAL*1 D, M
        INTEGER*2 Y
      END STRUCTURE
      RECORD /D/ W
      INTEGER Y, V
      W.Y = Y
      RETURN
      ENTRY GETYR(V, W)
      V = W.Y
      END

      SUBROUTINE BUMP
      STRUCTURE /QQ/
        INTEGER*4 N /7/
        CHARACTER*4 L /'BLK '/
      END STRUCTURE
      RECORD /QQ/ Q
      COMMON /BLK/ Q
      COMMON /PLAIN/ K1, K2
      DATA K1 /3/, K2 /4/
      Q.N = Q.N + 1
      Q.L = 'BUMP'
      END

      INTEGER FUNCTION ISUM(A, N)
      INTEGER A(N)
      ISUM = 0
      DO K = 1, N
        ISUM = ISUM + A(K)
      END DO
      END

      SUBROUTINE SHOW(N)
      CHARACTER*(*) N(3)
      PRINT *, N(2), LEN(N(2))
      END

      INTEGER FUNCTION MAXYR(LIST, N)
      STRUCTURE /D/
        LOGICAL*1 D, M
        INTEGER*2 Y
      END STRUCTURE
      RECORD /D/ LIST(N)
      MAXYR = 0
      DO K = 1, N
        IF (LIST(K).Y .GT. MAXYR) MAXYR = LIST(K).Y
      END DO
      END

      SUBROUTINE PASSON(R, KEPT, N)
      STRUCTURE /D/
        LOGICAL*1 D, M
        INTEGER*2 Y
      END STRUCTURE
      STRUCTURE /I/
        CHARACTER*3 T
        RECORD /D/ W
        INTEGER*4 Q(3)
      END STRUCTURE
      RECORD /I/ R, KEPT
      CALL DOUBLE(R)
      CALL NEXTYR(R.W)
      PRINT *, R.Q
      R.Q(N) = 7
      KEPT = R
      END

      SUBROUTINE SAVEIT(LIST, N)
      STRUCTURE /D/
        LOGICAL*1 D, M
        INTEGER*2 Y
      END STRUCTURE
      STRUCTURE /I/
        CHARACTER*3 T
        RECORD /D/ W
        INTEGER*4 Q(3)
      END STRUCTURE
      RECORD /I/ LIST(N), BACK
      OPEN (UNIT=2, FILE='AGG2.DAT', STATUS='NEW', FORM='UNFORMATTED')
      WRITE (2) LIST
      REWIND (2)
      READ (2) BACK
      CLOSE (UNIT=2, STATUS='DELETE')
      PRINT *, SIZEOF(LIST), BACK.Q(1), BACK.T
      END
