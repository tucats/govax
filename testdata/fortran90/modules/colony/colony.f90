! COLONY (Phase 112, subtask 14): Fortran 95's additions together, in Conway's
! game of Life on a board whose edges wrap around. Each colony is described by
! a NAMELIST group read from the terminal (colony.in), comments and all: the
! pattern's name, how many generations to run, and how often to draw it. The
! board is a derived type with default values and a pointer component starting
! `=> NULL()'; its cells are made, counted, advanced, and drawn by PURE and
! ELEMENTAL procedures (CELLS and WORLD), through FORALLs, a WHERE inside a
! WHERE, and an elemental defined assignment. Here: each drawing's lines sized
! by a PURE function in a declaration; the statistics in I0 and F0.2 fields;
! the percentage alive by CEILING with KIND=; a named FORALL construct with a
! mask counting how long each square was alive; and CPU_TIME.
PROGRAM COLONY
  USE WORLD
  IMPLICIT NONE
  CHARACTER (LEN = 6), PARAMETER :: GLIDER(6) = (/ '.O....', '..O...', 'OOO...', &
                                                  '......', '......', '......' /)
  CHARACTER (LEN = 5), PARAMETER :: BLINKER(5) = (/ '.....', '.....', '.OOO.', '.....', '.....' /)
  CHARACTER (LEN = 4), PARAMETER :: BLOCK(4) = (/ '....', '.OO.', '.OO.', '....' /)
  INTEGER :: GENERATIONS = 4, EVERY = 1
  CHARACTER (LEN = 10) :: PATTERN = 'GLIDER'
  NAMELIST /SETUP/ PATTERN, GENERATIONS, EVERY
  TYPE (BOARD) :: B
  INTEGER, ALLOCATABLE :: HEAT(:, :)
  INTEGER :: G, I, J, COLONIES
  REAL :: START, FINISH

  CALL CPU_TIME (START)
  PRINT '(1X, A, L2, I2)', 'Before any colony:', ASSOCIATED (B % AT), B % GENERATION
  COLONIES = 0
  DO
    READ (*, NML = SETUP, END = 10)
    SELECT CASE (PATTERN)
    CASE ('GLIDER')
      CALL MAKE (B, GLIDER)
    CASE ('BLINKER')
      CALL MAKE (B, BLINKER)
    CASE ('BLOCK')
      CALL MAKE (B, BLOCK)
    CASE DEFAULT
      PRINT '(1X, 3A)', 'No pattern ', TRIM (PATTERN), '.'
      CYCLE
    END SELECT
    COLONIES = COLONIES + 1
    PRINT '(/1X, A, 1X, I0, "x", I0, ",", 1X, I0, 1X, A, 1X, I0)', TRIM (PATTERN), B % ROWS, &
          B % COLS, GENERATIONS, 'generations, drawn every', EVERY
    ALLOCATE (HEAT(B % ROWS, B % COLS))
    HEAT = 0
    DO G = 0, GENERATIONS
      IF (G > 0) CALL ADVANCE (B)
      ALIVE: FORALL (I = 1:B % ROWS, J = 1:B % COLS, B % AT(I, J) % ALIVE)
        HEAT(I, J) = HEAT(I, J) + 1
      END FORALL ALIVE
      IF (MODULO (G, EVERY) == 0) CALL DRAW (B)
    END DO
    IF (ANY (COUNTS (B) /= SHIFTED (B))) PRINT '(1X, A)', 'The two counts differ.'
    CALL CENSUS (B)
    CALL OLDEST (B)
    PRINT '(1X, A)', 'Generations alive:'
    DO I = 1, B % ROWS
      PRINT '(1X, 40I3)', HEAT(I, :)
    END DO
    DEALLOCATE (HEAT, B % AT)
    B % AT => NULL ()
  END DO
10 CALL CPU_TIME (FINISH)
  PRINT '(/1X, I0, A, L2, 1X, I0)', COLONIES, ' colonies; the last board:', ASSOCIATED (B % AT), &
        B % GENERATION
  PRINT '(1X, A, L2)', 'CPU time went forward:', FINISH >= START

CONTAINS

  ! The board between bars, then its generation, population, the fraction of
  ! the board alive, and the percentage rounded up.
  SUBROUTINE DRAW (B)
    TYPE (BOARD), INTENT (IN) :: B
    CHARACTER (LEN = WIDTH (B % COLS)) :: LINE
    INTEGER :: I, LIVE

    DO I = 1, B % ROWS
      WRITE (LINE, '(42A)') '|', GLYPH (B % AT(I, :)), '|'
      PRINT '(1X, A)', LINE
    END DO
    LIVE = ALIVE_COUNT (B % AT)
    PRINT '(1X, A, I0, A, I0, A, F0.2, A, I0, A, I0)', 'generation ', B % GENERATION, ': ', LIVE, &
          ' alive, ', REAL (LIVE) / SIZE (B % AT), ', ', &
          CEILING (100.0 * LIVE / SIZE (B % AT), KIND = 2), '%; top row ', ALIVE_COUNT (B % AT(1, :))
  END SUBROUTINE DRAW

END PROGRAM COLONY
