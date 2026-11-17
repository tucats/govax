C     Fortran 90's CASE construct (Phase 101, subtask 3), in fixed form,
C     where blanks don't count: CASEDEFAULTPICK is CASE DEFAULT PICK.
      PROGRAM F90SELECT
      INTEGER I, N
      INTEGER*2 K
      CHARACTER*8 WORD
      LOGICAL L
      PARAMETER (LIMIT = 10)
      N = 0
C     INTEGER: single values, ranges, open ranges, DEFAULT in the middle.
      PICK: SELECT CASE (I)
      CASE (1, 3:5) PICK
        N = 1
      CASE DEFAULT PICK
        N = 2
      CASE (:0, LIMIT:)
        N = 3
      END SELECT PICK
C     Another kind: values compared by value, one never matching.
      SELECT CASE (K)
      CASE (-1)
      CASE (100000)
      END SELECT
C     CHARACTER: ranges compared as text, the shorter padded with blanks.
      SELECT CASE (WORD(1:1))
      CASE ('A':'M', 'Z')
        N = 4
      CASE ('N':'Y')
        SELECT CASE (L)
        CASE (.TRUE.)
          N = 5
        CASE (.FALSE.)
          N = 7
        END SELECT
      END SELECT
C     A jump to END SELECT from a block inside it.
      SELECT CASE (N)
      CASE (1)
        IF (L) GO TO 10
        N = 6
   10 END SELECT
      PRINT *, N
      END
