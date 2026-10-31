C     FOR80E.FOR - written by testdata/for80/gen.go: FOR80E.MAR as
C     FORTRAN, for govax. The part is the foreign command's text.
      PROGRAM FOR80E
      CHARACTER*8 PART
      CALL LIB$GET_FOREIGN(PART)
      IF (PART(1:1) .EQ. 'A') CALL PARTA
      IF (PART(1:1) .EQ. 'B') CALL PARTB
      IF (PART(1:1) .EQ. 'C') CALL PARTC
      IF (PART(1:1) .EQ. 'D') CALL PARTD
      IF (PART(1:1) .EQ. 'E') CALL PARTE
      IF (PART(1:1) .EQ. 'F') CALL PARTF
      IF (PART(1:1) .EQ. 'G') CALL PARTG
      IF (PART(1:1) .EQ. 'H') CALL PARTH
      IF (PART(1:1) .EQ. 'I') CALL PARTI
      IF (PART(1:1) .EQ. 'J') CALL PARTJ
      CALL LIB$PUT_OUTPUT('FOR80E: the statement returned')
      END

      SUBROUTINE PARTA
      WRITE (6, '(1X,I2)') 3244
      END

      SUBROUTINE PARTB
      REAL V1
      V1 = 3.25
      WRITE (6, '(1X,I5)') V1
      END

      SUBROUTINE PARTC
      WRITE (6, '(1X,J3)') 1
      END

      SUBROUTINE PARTD
      WRITE (6, '(1X,''A'')') 1
      END

      SUBROUTINE PARTE
      INTEGER V2
      CHARACTER*3 TABC
      TABC = 'ABC'
      READ (TABC, '(I3)') V2
      END

      SUBROUTINE PARTF
      INTEGER V2
      READ (5, '(I3)') V2
      END

      SUBROUTINE PARTG
      INTEGER V2
      READ (5, '(I3)') V2
      END

      SUBROUTINE PARTH
      CHARACTER*2 T2
      WRITE (T2, '(1X,I5)') 1
      END

      SUBROUTINE PARTI
      WRITE (1000, '(1X,I5)') 1
      END

      SUBROUTINE PARTJ
      INTEGER INNER
      WRITE (6, '(1X,I5)') INNER(6)
      END

      INTEGER FUNCTION INNER(IU)
      WRITE (IU, '(1X,I5)') 1
      INNER = 1
      END
