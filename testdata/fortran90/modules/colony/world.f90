! WORLD, COLONY's second module: a BOARD of CELLs on a torus, the generation
! it has reached, and its cells through a pointer component that starts
! disassociated, `=> NULL()'. MAKE fills a board from rows of text by a FORALL
! of the elemental assignment; COUNTS is each cell's live neighbors by a FORALL
! of a PURE function, SHIFTED the same by CSHIFT, so each checks the other;
! ADVANCE is one generation, an elemental function over the whole board;
! CENSUS says what the next generation will do to each cell by a WHERE inside a
! WHERE and a masked ELSEWHERE; OLDEST finds each row's oldest cell by MAXLOC
! and MAXVAL with DIM=.
MODULE WORLD
  USE CELLS
  IMPLICIT NONE

  TYPE BOARD
    INTEGER :: ROWS = 0, COLS = 0, GENERATION = 0
    TYPE (CELL), POINTER :: AT(:, :) => NULL()
  END TYPE BOARD

CONTAINS

  ! B is INTENT(OUT): its generation is 0 and its cells disassociated on entry.
  SUBROUTINE MAKE (B, PICTURE)
    TYPE (BOARD), INTENT (OUT) :: B
    CHARACTER (LEN = *), INTENT (IN) :: PICTURE(:)
    INTEGER :: I, J

    B % ROWS = SIZE (PICTURE)
    B % COLS = LEN (PICTURE)
    ALLOCATE (B % AT(B % ROWS, B % COLS))
    FORALL (I = 1:B % ROWS, J = 1:B % COLS) B % AT(I, J) = PICTURE(I)(J:J)
  END SUBROUTINE MAKE

  FUNCTION COUNTS (B) RESULT (N)
    TYPE (BOARD), INTENT (IN) :: B
    INTEGER :: N(B % ROWS, B % COLS)
    INTEGER :: I, J

    FORALL (I = 1:B % ROWS, J = 1:B % COLS) N(I, J) = NEIGHBORS (B % AT, I, J)
  END FUNCTION COUNTS

  FUNCTION SHIFTED (B) RESULT (N)
    TYPE (BOARD), INTENT (IN) :: B
    INTEGER :: N(B % ROWS, B % COLS)
    LOGICAL :: LIVE(B % ROWS, B % COLS)
    INTEGER :: DI, DJ

    LIVE = B % AT % ALIVE
    N = 0
    DO DI = -1, 1
      DO DJ = -1, 1
        IF (DI /= 0 .OR. DJ /= 0) N = N + MERGE (1, 0, CSHIFT (CSHIFT (LIVE, DI, 1), DJ, 2))
      END DO
    END DO
  END FUNCTION SHIFTED

  SUBROUTINE ADVANCE (B)
    TYPE (BOARD), INTENT (INOUT) :: B

    B % AT = NEXT_STATE (B % AT, COUNTS (B))
    B % GENERATION = B % GENERATION + 1
  END SUBROUTINE ADVANCE

  ! A map of what the next generation does: x a cell that dies, s one that
  ! survives, b one born, '.' one that stays empty.
  SUBROUTINE CENSUS (B)
    TYPE (BOARD), INTENT (IN) :: B
    CHARACTER :: MAP(B % ROWS, B % COLS)
    INTEGER :: N(B % ROWS, B % COLS)
    INTEGER :: I

    N = COUNTS (B)
    WHERE (B % AT % ALIVE)
      WHERE (N < 2 .OR. N > 3)
        MAP = 'x'
      ELSEWHERE
        MAP = 's'
      END WHERE
    ELSEWHERE (N == 3)
      MAP = 'b'
    ELSEWHERE
      MAP = '.'
    END WHERE
    DO I = 1, B % ROWS
      PRINT '(1X, A, 1X, 40A1)', 'next', MAP(I, :)
    END DO
  END SUBROUTINE CENSUS

  ! Each row's oldest cell, its column and age; a row of no live cells gives
  ! column 1 and age 0, the first of equals.
  SUBROUTINE OLDEST (B)
    TYPE (BOARD), INTENT (IN) :: B
    INTEGER :: PLACE(B % ROWS), AGE(B % ROWS)
    INTEGER :: I

    PLACE = MAXLOC (B % AT % AGE, DIM = 2)
    AGE = MAXVAL (B % AT % AGE, DIM = 2)
    PRINT '(1X, A, 40(1X, I0, ":", I0))', 'oldest', (PLACE(I), AGE(I), I = 1, B % ROWS)
  END SUBROUTINE OLDEST

END MODULE WORLD
