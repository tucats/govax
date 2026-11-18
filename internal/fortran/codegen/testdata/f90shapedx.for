      OPTIONS /F90
C     Fortran 90's assumed-shape dummies (Phase 104, subtask 5b): actual
C     arguments computed into a temporary of the statement's (an expression, a
C     vector-subscripted section, an array in parentheses, one of a size known
C     only when the program runs), a substring of a section described where it
C     is, an OPTIONAL dummy passed on (absent if it was), and a generic name
C     whose specific has an assumed-shape dummy. The entry accepts a class A
C     descriptor as well as an NCA one.
      SUBROUTINE TAKE(V, C, O)
      REAL V(:)
      CHARACTER*(*) C(:)
      REAL, OPTIONAL :: O(:)
      PRINT *, SIZE(V), C(1), PRESENT(O)
      END
      SUBROUTINE GIVE(A, N, IV, S, O)
      INTEGER N, IV(3)
      REAL A(N)
      CHARACTER*4 S(5)
      REAL, OPTIONAL :: O(:)
      INTERFACE
        SUBROUTINE TAKE(V, C, O)
        REAL V(:)
        CHARACTER*(*) C(:)
        REAL, OPTIONAL :: O(:)
        END SUBROUTINE
      END INTERFACE
      INTERFACE G
        SUBROUTINE TAKEG(V)
        REAL V(:)
        END SUBROUTINE
      END INTERFACE
      CALL TAKE(A(IV), S(2:4)(2:3), O)
      CALL TAKE(A * 2.0, S(IV), (A))
      CALL TAKE((/ 1.0, 2.0 /), S // 'X')
      CALL G(A(1:N:2) + 1.0)
      END
