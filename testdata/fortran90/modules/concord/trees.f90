! TREES: a binary search tree of words, each with how often it was seen and
! a LIST of the lines it was seen on.
MODULE TREES
  USE WORDS
  USE LISTS
  IMPLICIT NONE
  TYPE TREE
    CHARACTER (LEN = WORD_LENGTH) :: WORD
    INTEGER :: COUNT
    TYPE (LIST) :: LINES
    TYPE (TREE), POINTER :: LEFT, RIGHT
  END TYPE TREE
  INTEGER :: TREE_NODES = 0
CONTAINS
  ! W seen on line LINE: counted at its node, made if it isn't there.
  RECURSIVE SUBROUTINE INSERT (T, W, LINE)
    TYPE (TREE), POINTER :: T
    CHARACTER (LEN = *), INTENT (IN) :: W
    INTEGER, INTENT (IN) :: LINE
    IF (.NOT. ASSOCIATED (T)) THEN
      ALLOCATE (T)
      TREE_NODES = TREE_NODES + 1
      T % WORD = W
      T % COUNT = 0
      CALL START (T % LINES)
      NULLIFY (T % LEFT, T % RIGHT)
    END IF
    IF (W < T % WORD) THEN
      CALL INSERT (T % LEFT, W, LINE)
    ELSE IF (W > T % WORD) THEN
      CALL INSERT (T % RIGHT, W, LINE)
    ELSE
      T % COUNT = T % COUNT + 1
      IF (LAST (T % LINES) /= LINE) CALL APPEND (T % LINES, LINE)
    END IF
  END SUBROUTINE INSERT

  ! The node holding W, or a disassociated pointer.
  RECURSIVE FUNCTION FIND (T, W) RESULT (HIT)
    TYPE (TREE), POINTER :: T, HIT
    CHARACTER (LEN = *), INTENT (IN) :: W
    IF (.NOT. ASSOCIATED (T)) THEN
      NULLIFY (HIT)
    ELSE IF (W < T % WORD) THEN
      HIT => FIND (T % LEFT, W)
    ELSE IF (W > T % WORD) THEN
      HIT => FIND (T % RIGHT, W)
    ELSE
      HIT => T
    END IF
  END FUNCTION FIND

  RECURSIVE INTEGER FUNCTION HEIGHT (T) RESULT (H)
    TYPE (TREE), POINTER :: T
    IF (ASSOCIATED (T)) THEN
      H = 1 + MAX (HEIGHT (T % LEFT), HEIGHT (T % RIGHT))
    ELSE
      H = 0
    END IF
  END FUNCTION HEIGHT

  ! Each word in order, with its count and lines.
  RECURSIVE SUBROUTINE SHOW (T)
    TYPE (TREE), POINTER :: T
    INTEGER :: LINES (20), N
    IF (.NOT. ASSOCIATED (T)) RETURN
    CALL SHOW (T % LEFT)
    CALL ITEMS (T % LINES, LINES, N)
    PRINT '(1X, A, I3, A, 20I3)', T % WORD, T % COUNT, ':', LINES (1:N)
    CALL SHOW (T % RIGHT)
  END SUBROUTINE SHOW

  ! The tree's shape: each node's word, then its subtrees in parentheses.
  RECURSIVE SUBROUTINE OUTLINE (T, TEXT, N)
    TYPE (TREE), POINTER :: T
    CHARACTER (LEN = *), INTENT (INOUT) :: TEXT
    INTEGER, INTENT (INOUT) :: N
    INTEGER :: K
    IF (.NOT. ASSOCIATED (T)) THEN
      CALL PUT ('-')
      RETURN
    END IF
    DO K = 1, WORD_LENGTH
      IF (T % WORD (K:K) == ' ') EXIT
      CALL PUT (T % WORD (K:K))
    END DO
    IF (ASSOCIATED (T % LEFT) .OR. ASSOCIATED (T % RIGHT)) THEN
      CALL PUT ('(')
      CALL OUTLINE (T % LEFT, TEXT, N)
      CALL PUT (' ')
      CALL OUTLINE (T % RIGHT, TEXT, N)
      CALL PUT (')')
    END IF
  CONTAINS
    SUBROUTINE PUT (C)
      CHARACTER, INTENT (IN) :: C
      N = N + 1
      IF (N <= LEN (TEXT)) TEXT (N:N) = C
    END SUBROUTINE PUT
  END SUBROUTINE OUTLINE

  ! W taken out of the tree: a node with one subtree or none is replaced by
  ! it; one with two takes the word, count, and lines of the least node of
  ! its right subtree, which is taken out in its place.
  RECURSIVE SUBROUTINE REMOVE (T, W)
    TYPE (TREE), POINTER :: T
    CHARACTER (LEN = *), INTENT (IN) :: W
    TYPE (TREE), POINTER :: GONE, LEAST
    IF (.NOT. ASSOCIATED (T)) RETURN
    IF (W < T % WORD) THEN
      CALL REMOVE (T % LEFT, W)
    ELSE IF (W > T % WORD) THEN
      CALL REMOVE (T % RIGHT, W)
    ELSE IF (ASSOCIATED (T % LEFT) .AND. ASSOCIATED (T % RIGHT)) THEN
      LEAST => T % RIGHT
      DO WHILE (ASSOCIATED (LEAST % LEFT))
        LEAST => LEAST % LEFT
      END DO
      CALL FREE (T % LINES)
      T % WORD = LEAST % WORD
      T % COUNT = LEAST % COUNT
      T % LINES = LEAST % LINES
      CALL START (LEAST % LINES)
      CALL REMOVE (T % RIGHT, T % WORD)
    ELSE
      GONE => T
      IF (ASSOCIATED (T % LEFT)) THEN
        T => T % LEFT
      ELSE
        T => T % RIGHT
      END IF
      CALL FREE (GONE % LINES)
      DEALLOCATE (GONE)
      TREE_NODES = TREE_NODES - 1
    END IF
  END SUBROUTINE REMOVE

  RECURSIVE SUBROUTINE CUT_DOWN (T)
    TYPE (TREE), POINTER :: T
    IF (.NOT. ASSOCIATED (T)) RETURN
    CALL CUT_DOWN (T % LEFT)
    CALL CUT_DOWN (T % RIGHT)
    CALL FREE (T % LINES)
    DEALLOCATE (T)
    TREE_NODES = TREE_NODES - 1
  END SUBROUTINE CUT_DOWN
END MODULE TREES
