C     Arrays of records as arguments and dynamic arrays of records
C     (docs/planning/PHASE-105 - Fortran 90 derived types.md, subtask 5b),
C     the Fortran 90 forms derived types brought, for a STRUCTURE's records:
C     every record's field, RB.CODE, or a section of an array field,
C     RA.QTY(2), to an assumed-shape dummy by stride and to an old one by a
C     copy; arrays of records to assumed-shape and old dummies; an
C     allocatable array of records, and an automatic one. SIZE and SUM are
C     Fortran 90 intrinsics, which a VAX FORTRAN program declares INTRINSIC.
      PROGRAM RECDYN
      STRUCTURE /ITEM/
        INTEGER*2 CODE
        CHARACTER*6 NAME
        REAL QTY(3)
      END STRUCTURE
      RECORD /ITEM/ RA(4), RB(:)
      ALLOCATABLE RB
      INTEGER I, J
      REAL T
      DO I = 1, 4
        RA(I).CODE = 10 * I
        RA(I).NAME = 'PART'
        RA(I).QTY = (/ 1.0, 2.0, 3.0 /) * I
      END DO
      ALLOCATE (RB(3))
      RB = RA(2:4)
      CALL BUMP(RB.CODE)
      CALL BUMP(RA(1:4:3).CODE)
      CALL SCALE(RA.QTY(2), 10.0)
      CALL SCALE(RB(2).QTY, 0.5)
      CALL TOTAL(RA.QTY(3), 4, T)
      CALL TWICE(RB.QTY(1), 3)
      CALL TAG(RB, 'X')
      CALL TAG(RA(2:3), 'Y')
      CALL KEEP(RA(1:4:2), 2)
      CALL KEEP(RB(2:3), 2)
      CALL REVERSE(RA)
      WRITE (6, 10) RA.CODE, RB.CODE
      WRITE (6, 20) (RA(I).NAME, I = 1, 4), (RB(I).NAME, I = 1, 3)
      WRITE (6, 30) ((RA(I).QTY(J), J = 1, 3), I = 1, 4)
      WRITE (6, 30) ((RB(I).QTY(J), J = 1, 3), I = 1, 3)
      WRITE (6, 30) T
      DEALLOCATE (RB)
10    FORMAT (1X, 7I6)
20    FORMAT (1X, 7(A, 1X))
30    FORMAT (1X, 12F6.1)
      CONTAINS
      SUBROUTINE BUMP(M)
      INTEGER*2 M(:)
      M = M + 1
      END SUBROUTINE
      SUBROUTINE SCALE(V, F)
      REAL V(:), F
      V = V * F
      END SUBROUTINE
      SUBROUTINE TOTAL(V, N, T)
      INTEGER N
      REAL V(N), T
      INTRINSIC SUM
      T = SUM(V)
      END SUBROUTINE
      SUBROUTINE TWICE(V, N)
      INTEGER N
      REAL V(N)
      V = V * 2.0
      END SUBROUTINE
      SUBROUTINE TAG(R, C)
      RECORD /ITEM/ R(:)
      CHARACTER C
      INTEGER K
      INTRINSIC SIZE
      DO K = 1, SIZE(R)
        R(K).NAME(6:6) = C
      END DO
      R.CODE = R.CODE + 1000
      END SUBROUTINE
      SUBROUTINE KEEP(R, N)
      INTEGER N
      RECORD /ITEM/ R(N)
      R.CODE = -R.CODE
      END SUBROUTINE
      SUBROUTINE REVERSE(R)
      RECORD /ITEM/ R(:)
      INTRINSIC SIZE
      RECORD /ITEM/ W(SIZE(R))
      W = R
      R = W(SIZE(R):1:-1)
      END SUBROUTINE
      END PROGRAM
