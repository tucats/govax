! CELLS, the first of COLONY's modules (Phase 112, subtask 14): one square of
! the board and what may be done with it, every procedure PURE or ELEMENTAL so
! a FORALL or an array expression may call it. A CELL's components have
! default values, so every CELL made anywhere starts empty; text becomes cells
! through an elemental defined assignment, whose INTENT(OUT) dummy takes those
! defaults at each call; ALIVE_COUNT is a generic name over two ranks, its
! interface block closed with its name, and so is the assignment's.
MODULE CELLS
  IMPLICIT NONE
  PRIVATE
  PUBLIC :: CELL, ASSIGNMENT (=), ALIVE_COUNT, GLYPH, NEXT_STATE, NEIGHBORS, WIDTH

  ! A square of the board: alive or not, and for how many generations.
  TYPE CELL
    LOGICAL :: ALIVE = .FALSE.
    INTEGER :: AGE = 0
  END TYPE CELL

  INTERFACE ASSIGNMENT (=)
    MODULE PROCEDURE FROM_LETTER
  END INTERFACE ASSIGNMENT (=)

  INTERFACE ALIVE_COUNT
    MODULE PROCEDURE COUNT_ROW, COUNT_BOARD
  END INTERFACE ALIVE_COUNT

CONTAINS

  ! C = 'O' is a cell just born; any other letter an empty one. C is
  ! INTENT(OUT), so it arrives with the type's defaults, and only a live
  ! cell's components need setting.
  ELEMENTAL SUBROUTINE FROM_LETTER (C, LETTER)
    TYPE (CELL), INTENT (OUT) :: C
    CHARACTER, INTENT (IN) :: LETTER

    IF (LETTER == 'O') THEN
      C % ALIVE = .TRUE.
      C % AGE = 1
    END IF
  END SUBROUTINE FROM_LETTER

  ! How a cell is drawn: '.' if empty, its age if 1 to 9, '+' if older.
  ELEMENTAL CHARACTER FUNCTION GLYPH (C)
    TYPE (CELL), INTENT (IN) :: C

    IF (.NOT. C % ALIVE) THEN
      GLYPH = '.'
    ELSE IF (C % AGE > 9) THEN
      GLYPH = '+'
    ELSE
      GLYPH = ACHAR (IACHAR ('0') + C % AGE)
    END IF
  END FUNCTION GLYPH

  ! Life's rule: a live cell with two or three live neighbors lives on, one
  ! older; an empty one with exactly three is born; any other is empty: NEW,
  ! a function result, starts with the type's defaults.
  ELEMENTAL FUNCTION NEXT_STATE (C, N) RESULT (NEW)
    TYPE (CELL), INTENT (IN) :: C
    INTEGER, INTENT (IN) :: N
    TYPE (CELL) :: NEW

    IF (C % ALIVE .AND. (N == 2 .OR. N == 3)) THEN
      NEW = CELL (.TRUE., C % AGE + 1)
    ELSE IF (.NOT. C % ALIVE .AND. N == 3) THEN
      NEW = CELL (.TRUE., 1)
    END IF
  END FUNCTION NEXT_STATE

  ! The live neighbors of G(I, J), the board wrapped around at its edges.
  PURE INTEGER FUNCTION NEIGHBORS (G, I, J)
    TYPE (CELL), INTENT (IN) :: G(:, :)
    INTEGER, INTENT (IN) :: I, J
    INTEGER :: DI, DJ, R, C

    R = SIZE (G, 1)
    C = SIZE (G, 2)
    NEIGHBORS = 0
    DO DI = -1, 1
      DO DJ = -1, 1
        IF (DI == 0 .AND. DJ == 0) CYCLE
        IF (G(MODULO (I + DI - 1, R) + 1, MODULO (J + DJ - 1, C) + 1) % ALIVE) &
          NEIGHBORS = NEIGHBORS + 1
      END DO
    END DO
  END FUNCTION NEIGHBORS

  PURE INTEGER FUNCTION COUNT_ROW (ROW)
    TYPE (CELL), INTENT (IN) :: ROW(:)

    COUNT_ROW = COUNT (ROW % ALIVE)
  END FUNCTION COUNT_ROW

  PURE INTEGER FUNCTION COUNT_BOARD (G)
    TYPE (CELL), INTENT (IN) :: G(:, :)

    COUNT_BOARD = COUNT (G % ALIVE)
  END FUNCTION COUNT_BOARD

  ! The width of a drawn row of N cells, between its two bars: PURE, so a
  ! declaration's length may call it.
  PURE INTEGER FUNCTION WIDTH (N)
    INTEGER, INTENT (IN) :: N

    WIDTH = N + 2
  END FUNCTION WIDTH

END MODULE CELLS
