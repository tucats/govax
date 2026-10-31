C     NML80E.FOR - written by testdata/nml80/gen.go: NML80E.MAR as FORTRAN.
      PROGRAM NML80E
      INTEGER I, A(3), B(2,2), LONGNAME1
      INTEGER*2 J
      REAL R
      COMPLEX C
      CHARACTER*5 T
      CHARACTER*3 TA(3)
      NAMELIST /GRP/ I, J, R, C, T, A, B, TA, LONGNAME1
      DATA T /' '/, TA /3*' '/
      READ (5, GRP)
      END
