C     Fortran 90's CASE construct lowered (Phase 101, subtask 4): each of
C     caseForm's forms, on both sides of its thresholds.
      PROGRAM F90SELECT
      INTEGER I, N
      INTEGER*2 K
      CHARACTER*8 WORD, WORDS(4)
      LOGICAL L
C     A table: 4 tests over 16 values (4 slots a test), DEFAULT for the gaps,
C     and the open range tested when CASEL finds the value outside it.
      SELECT CASE (I)
      CASE (-1)
        N = 1
      CASE (3, 14)
        N = 2
      CASE (100:)
        N = 4
      CASE DEFAULT
        N = 5
      END SELECT
C     A chain: 4 tests over 17 values, too sparse for a table.
      SELECT CASE (I)
      CASE (1)
        N = 1
      CASE (3)
        N = 2
      CASE (17)
        N = 3
      CASE (10)
        N = 4
      END SELECT
C     A chain: 3 tests, too few for a table however dense.
      SELECT CASE (I)
      CASE (1, 2, 3)
        N = 1
      END SELECT
C     A binary search: 8 tests, sparse.
      SELECT CASE (I)
      CASE (10, 20:29)
        N = 1
      CASE (40, 50, 60:)
        N = 2
      CASE (:0)
        N = 3
      CASE (35)
        N = 4
      END SELECT
C     A chain: 7 tests, sparse.
      SELECT CASE (I)
      CASE (10, 20:29)
        N = 1
      CASE (40, 50, 60:)
        N = 2
      CASE (:0)
        N = 3
      END SELECT
C     INTEGER*2: widened, the range clipped to what it holds, which needs
C     no test of its high end; nor does the one reaching its lowest value.
      SELECT CASE (K)
      CASE (30000:100000)
        N = 1
      CASE (-32768:-1)
        N = 3
      END SELECT
C     An expression, and a constant one.
      SELECT CASE (I + N)
      CASE (2)
        N = 1
      END SELECT
      SELECT CASE (3)
      CASE (1:2)
        N = 1
      CASE (3:)
        N = 2
      END SELECT
C     LOGICAL.
      SELECT CASE (L)
      CASE (.FALSE.)
        N = 1
      CASE DEFAULT
        N = 2
      END SELECT
C     CHARACTER: a chain, then a search, then a place found at run time.
      SELECT CASE (WORD)
      CASE ('A':'M', 'ZZ')
        N = 1
      CASE DEFAULT
        N = 2
      END SELECT
      SELECT CASE (WORD(1:2))
      CASE ('A', 'C', 'E', 'G')
        N = 1
      CASE ('I', 'K', 'M', 'O')
        N = 2
      END SELECT
      SELECT CASE (WORDS(I))
      CASE ('X')
        N = 1
      END SELECT
C     A jump to END SELECT from inside it.
      SELECT CASE (N)
      CASE (1)
        IF (L) GO TO 10
        N = 6
   10 END SELECT
      PRINT *, N
      END
C     A value only known at run time: a dynamic string, freed after the
C     comparisons.
      SUBROUTINE PICK(S, N)
      CHARACTER*(*) S
      SELECT CASE (S // 'X')
      CASE ('AX')
        N = 1
      CASE DEFAULT
        N = 2
      END SELECT
      END
