! A concordance (Phase 107, subtask 8): the words of the lines typed, each
! with how often it was seen and on which lines, from a binary search tree of
! words (TREES) whose nodes hold queues of line numbers (LISTS), each a
! module of its own, all of their nodes made by ALLOCATE and found through
! pointer components; the tree's shape, its height, words found and not by a
! recursive pointer function, and words taken out (a leaf, a node with one
! subtree, one with two, and the root), every node given back at the end.
PROGRAM CONCORD
  USE TREES
  IMPLICIT NONE
  TYPE (TREE), POINTER :: ROOT, HIT
  CHARACTER (LEN = 80) :: LINE
  CHARACTER (LEN = WORD_LENGTH) :: WORD
  CHARACTER (LEN = 200) :: SHAPE
  INTEGER :: LINES, POS, SEEN, N

  NULLIFY (ROOT)
  LINES = 0
  SEEN = 0
  DO
    READ (*, '(A)', END = 10) LINE
    LINES = LINES + 1
    POS = 1
    DO
      CALL NEXT_WORD (LINE, POS, WORD)
      IF (WORD == ' ') EXIT
      SEEN = SEEN + 1
      CALL INSERT (ROOT, WORD, LINES)
    END DO
  END DO
10 PRINT '(1X, I2, A, I3, A, I3, A, I3, A)', LINES, ' lines,', SEEN, ' words,', TREE_NODES, &
         ' different,', NODES, ' line numbers'
  CALL SHOW (ROOT)
  CALL REPORT ('As read:')

  HIT => FIND (ROOT, 'OVER')
  PRINT '(1X, A, L2, 1X, A, I3)', 'OVER:', ASSOCIATED (HIT), HIT % WORD, HIT % COUNT
  HIT % COUNT = HIT % COUNT * 100
  NULLIFY (HIT)
  HIT => FIND (ROOT, 'OVER')
  PRINT '(1X, A, I5, L2)', 'OVER now:', HIT % COUNT, ASSOCIATED (FIND (ROOT, 'CAT'))

  CALL REMOVE (ROOT, 'SUN')
  CALL REPORT ('Without SUN, a leaf:')
  CALL REMOVE (ROOT, 'LAZY')
  CALL REPORT ('Without LAZY, one subtree:')
  CALL REMOVE (ROOT, 'QUICK')
  CALL REPORT ('Without QUICK, two subtrees:')
  CALL REMOVE (ROOT, ROOT % WORD)
  CALL REPORT ('Without the root, one subtree:')
  CALL REMOVE (ROOT, 'CAT')
  CALL SHOW (ROOT)

  CALL CUT_DOWN (ROOT)
  PRINT '(1X, A, I3, I3, L2)', 'Left:', TREE_NODES, NODES, ASSOCIATED (ROOT)
CONTAINS
  SUBROUTINE REPORT (TITLE)
    CHARACTER (LEN = *), INTENT (IN) :: TITLE
    SHAPE = ' '
    N = 0
    CALL OUTLINE (ROOT, SHAPE, N)
    PRINT '(1X, A, I3, A, I2, A, I3, A)', TITLE, TREE_NODES, ' words, height', HEIGHT (ROOT), &
          ',', NODES, ' line numbers'
    PRINT '(1X, A)', SHAPE (1:N)
  END SUBROUTINE REPORT
END PROGRAM CONCORD
