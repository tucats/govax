      OPTIONS /F90
C     Arrays of derived types and components with rank (Phase 105,
C     subtask 5a): a component of every element, PATH%X; of a section,
C     PATH(2:6:2)%Y; a section of an array component, T%C(2:3); a vector
C     subscript of records, PATH(IV)%X; an array of a type whole and as a
C     section; each assignment's order: a section read before it is
C     written, components that are apart, components that may meet; a
C     structure constructor whose array component's value is an array.
      PROGRAM COMPS
      TYPE POINT
        REAL X, Y
      END TYPE POINT
      TYPE POLY
        INTEGER N
        REAL C(3)
      END TYPE POLY
      TYPE(POINT) PATH(6), Q(6), P
      TYPE(POLY) T
      INTEGER IV(2)
      REAL V(3)
      PATH%X = 1.0
      PATH(2:6:2)%Y = PATH(1:5:2)%X + 1.0
      T%C(2:3) = T%C(1:2)
      PATH(2:6)%X = PATH(1:5)%X
      PATH%X = PATH%Y
      PATH%Y = PATH(6:1:-1)%Y
      Q = PATH
      Q(2:3) = Q(1:2)
      Q = P
      Q(IV) = POINT(1.0, 2.0)
      T = POLY(3, V * 2.0)
      V(1:2) = PATH(IV)%X + T%C(2:3)
      END
C     A STRUCTURE's records: a section of an array field, and an array of
C     records whole, as before Fortran 90 but for the sections.
      SUBROUTINE RECS
      STRUCTURE /S/
        INTEGER A, B(4)
      END STRUCTURE
      RECORD /S/ R, RA(3)
      R.B(1:2) = R.B(3:4)
      RA.A = R.B(2:4)
      RA(2:3) = R
      END
