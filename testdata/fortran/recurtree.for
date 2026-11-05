C A tree of records walked by recursive routines (Phase 89, subtask 11): the
C nodes are records in an array in COMMON, linked by their subscripts. INSERT
C passes a node's link field on as the dummy it sets; WALK keeps a copy of its
C node and the line it prints in its own frame across the walk below it;
C MIRROR builds the tree's mirror image from a record in each frame, assigned
C whole, its links set from calls of itself; PATHS passes a concatenation in its
C frame down as the next activation's passed-length dummy, and uses it again
C after the first child's walk. HEIGHT, TOTAL, and DEPTH are functions with
C RESULT, given fields as arguments.
      PROGRAM TREES
      STRUCTURE /NODE/
         CHARACTER*8 KEY
         INTEGER COUNT, LEFT, RIGHT
      END STRUCTURE
      RECORD /NODE/ POOL(40)
      INTEGER NUSED, ROOT, MROOT, I, HEIGHT, MIRROR, TOTAL, DEPTH
      CHARACTER*8 WORDS(12)
      COMMON /TREE/ NUSED, POOL
      DATA WORDS /'PEAR', 'APPLE', 'FIG', 'PLUM', 'APPLE', 'KIWI',
     1   'DATE', 'FIG', 'APPLE', 'LIME', 'CHERRY', 'PEAR'/

      NUSED = 0
      ROOT = 0
      DO 10 I = 1, 12
         CALL INSERT(ROOT, WORDS(I))
   10 CONTINUE
      WRITE (6, 100) ROOT, NUSED, HEIGHT(ROOT), TOTAL(ROOT)
  100 FORMAT (' ROOT', I3, '   NODES', I3, '   HEIGHT', I3,
     1   '   WORDS', I3)
      CALL WALK(ROOT, 0)
      CALL PATHS(ROOT, ' ', 0)

      MROOT = MIRROR(ROOT)
      WRITE (6, 100) MROOT, NUSED, HEIGHT(MROOT), TOTAL(MROOT)
      CALL WALK(MROOT, 0)
      CALL PATHS(MROOT, ' ', 0)

      WRITE (6, 110) DEPTH(ROOT, 'KIWI', 0), DEPTH(ROOT, 'GRAPE', 0),
     1   DEPTH(MROOT, 'CHERRY', 0), DEPTH(MROOT, 'PEAR', 0)
  110 FORMAT (' DEPTHS', 4I3)
      END

C The link is the parent's field, set here when the word is new.
      RECURSIVE SUBROUTINE INSERT(I, WORD)
      STRUCTURE /NODE/
         CHARACTER*8 KEY
         INTEGER COUNT, LEFT, RIGHT
      END STRUCTURE
      RECORD /NODE/ POOL(40)
      COMMON /TREE/ NUSED, POOL
      INTEGER I
      CHARACTER*(*) WORD
      IF (I .EQ. 0) THEN
         NUSED = NUSED + 1
         I = NUSED
         POOL(I).KEY = WORD
         POOL(I).COUNT = 1
         POOL(I).LEFT = 0
         POOL(I).RIGHT = 0
      ELSE IF (WORD .LT. POOL(I).KEY) THEN
         CALL INSERT(POOL(I).LEFT, WORD)
      ELSE IF (WORD .GT. POOL(I).KEY) THEN
         CALL INSERT(POOL(I).RIGHT, WORD)
      ELSE
         POOL(I).COUNT = POOL(I).COUNT + 1
      END IF
      END

C In order, each word indented by its depth. HERE and LINE are made before the
C left subtree's walk and used after it.
      RECURSIVE SUBROUTINE WALK(I, D)
      STRUCTURE /NODE/
         CHARACTER*8 KEY
         INTEGER COUNT, LEFT, RIGHT
      END STRUCTURE
      RECORD /NODE/ POOL(40), HERE
      COMMON /TREE/ NUSED, POOL
      CHARACTER*20 LINE
      INTEGER I, D
      IF (I .EQ. 0) RETURN
      HERE = POOL(I)
      LINE = ' '
      LINE(2*D+1:) = HERE.KEY
      CALL WALK(HERE.LEFT, D + 1)
      WRITE (6, 100) LINE, HERE.COUNT
  100 FORMAT (1X, A, I2)
      CALL WALK(HERE.RIGHT, D + 1)
      END

C Every path from the root to a leaf.
      RECURSIVE SUBROUTINE PATHS(I, TRAIL, N)
      STRUCTURE /NODE/
         CHARACTER*8 KEY
         INTEGER COUNT, LEFT, RIGHT
      END STRUCTURE
      RECORD /NODE/ POOL(40)
      COMMON /TREE/ NUSED, POOL
      CHARACTER*(*) TRAIL
      CHARACTER*60 NEXT
      INTEGER I, N, M, LENTRM
      IF (I .EQ. 0) RETURN
      IF (N .EQ. 0) THEN
         NEXT = '/' // POOL(I).KEY
      ELSE
         NEXT = TRAIL(1:N) // '/' // POOL(I).KEY
      END IF
      M = N + 1 + LENTRM(POOL(I).KEY)
      IF (POOL(I).LEFT .EQ. 0 .AND. POOL(I).RIGHT .EQ. 0) THEN
         WRITE (6, 100) NEXT(1:M)
  100    FORMAT (' PATH ', A)
      ELSE
         CALL PATHS(POOL(I).LEFT, NEXT, M)
         CALL PATHS(POOL(I).RIGHT, NEXT, M)
      END IF
      END

C A new tree, its children made before it, so it is numbered in postorder.
      RECURSIVE INTEGER FUNCTION MIRROR(I) RESULT(M)
      STRUCTURE /NODE/
         CHARACTER*8 KEY
         INTEGER COUNT, LEFT, RIGHT
      END STRUCTURE
      RECORD /NODE/ POOL(40), NEW
      COMMON /TREE/ NUSED, POOL
      INTEGER I
      IF (I .EQ. 0) THEN
         M = 0
      ELSE
         NEW = POOL(I)
         NEW.LEFT = MIRROR(POOL(I).RIGHT)
         NEW.RIGHT = MIRROR(POOL(I).LEFT)
         NUSED = NUSED + 1
         POOL(NUSED) = NEW
         M = NUSED
      END IF
      END

C The number of nodes on the longest path down.
      RECURSIVE INTEGER FUNCTION HEIGHT(I) RESULT(H)
      STRUCTURE /NODE/
         CHARACTER*8 KEY
         INTEGER COUNT, LEFT, RIGHT
      END STRUCTURE
      RECORD /NODE/ POOL(40)
      COMMON /TREE/ NUSED, POOL
      INTEGER I
      IF (I .EQ. 0) THEN
         H = 0
      ELSE
         H = 1 + MAX(HEIGHT(POOL(I).LEFT), HEIGHT(POOL(I).RIGHT))
      END IF
      END

C The words counted, duplicates and all.
      RECURSIVE INTEGER FUNCTION TOTAL(I) RESULT(T)
      STRUCTURE /NODE/
         CHARACTER*8 KEY
         INTEGER COUNT, LEFT, RIGHT
      END STRUCTURE
      RECORD /NODE/ POOL(40)
      COMMON /TREE/ NUSED, POOL
      INTEGER I
      IF (I .EQ. 0) THEN
         T = 0
      ELSE
         T = TOTAL(POOL(I).LEFT) + POOL(I).COUNT + TOTAL(POOL(I).RIGHT)
      END IF
      END

C A word's depth by a search of both subtrees (a mirror's order is reversed),
C or -1.
      RECURSIVE INTEGER FUNCTION DEPTH(I, WORD, D) RESULT(R)
      STRUCTURE /NODE/
         CHARACTER*8 KEY
         INTEGER COUNT, LEFT, RIGHT
      END STRUCTURE
      RECORD /NODE/ POOL(40)
      COMMON /TREE/ NUSED, POOL
      INTEGER I, D
      CHARACTER*(*) WORD
      IF (I .EQ. 0) THEN
         R = -1
      ELSE IF (POOL(I).KEY .EQ. WORD) THEN
         R = D
      ELSE
         R = DEPTH(POOL(I).LEFT, WORD, D + 1)
         IF (R .LT. 0) R = DEPTH(POOL(I).RIGHT, WORD, D + 1)
      END IF
      END

C The length of a text without its trailing blanks.
      INTEGER FUNCTION LENTRM(S)
      CHARACTER*(*) S
      DO 10 LENTRM = LEN(S), 1, -1
         IF (S(LENTRM:LENTRM) .NE. ' ') RETURN
   10 CONTINUE
      LENTRM = 0
      END
