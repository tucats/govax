C     Records and scalar field references (sections 2.2.5.2, 4.13): records
C     in $LOCAL with their structures' initial values (a substructure's, and
C     a union's later map's replacing the earlier one's bytes), fields of
C     every kind of place, constant and variable subscripts.
      PROGRAM RECS
      STRUCTURE /DATE/
        LOGICAL*1 DAY /1/, MONTH /1/
        INTEGER*2 YEAR /1988/
      END STRUCTURE
      STRUCTURE /ENTRY/
        CHARACTER*3 TAG /'ABC'/
        RECORD /DATE/ WHEN
        INTEGER*4 COUNTS(2)
        UNION
          MAP
            INTEGER*4 WHOLE /-1/
          END MAP
          MAP
            INTEGER*2 HALF /5/
          END MAP
        END UNION
        STRUCTURE PAIR(2)
          REAL*4 X
          COMPLEX*8 Z
        END STRUCTURE
      END STRUCTURE
      RECORD /DATE/ TODAY
      RECORD /ENTRY/ E, LIST(3)
      CHARACTER*2 S
      TODAY.YEAR = TODAY.YEAR + 1
      E.COUNTS(2) = TODAY.DAY
      E.PAIR(1).Z = (1.0, 2.0)
      E.PAIR(2).X = REAL(E.PAIR(1).Z)
      LIST(I).COUNTS(J) = I + J
      LIST(I).WHEN.YEAR = 2000
      LIST(2).PAIR(I).X = 0.5
      S = LIST(I).TAG(2:3)
      E.TAG(I:I) = 'Z'
      CALL SHOW(E.COUNTS(1), E.TAG, LIST(I).PAIR(J).Z)
      PRINT *, TODAY.YEAR, E.TAG, %LOC(E.WHEN.MONTH)
      END
