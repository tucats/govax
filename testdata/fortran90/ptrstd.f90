! The standard's examples of pointers (Phase 107, subtask 8), in its capitals,
! made to run, each in an internal subroutine of its own so the names can be
! the standard's: 4.4.1's REFERENCE with a pointer to its abstract and NODE
! linked into a list; 5.1.2.7's and 5.1.2.8's declarations; 6.3.1's ALLOCATE
! with STAT= of a pointer and an allocatable array with bounds of their own,
! and 6.3.3's DEALLOCATE of both; 6.3.2's NULLIFY; 7.5.2's six pointer
! assignments (a tree node's component, a component of a component, a row and
! a window of a matrix, a pointer function's value, every other element);
! 13.13.13's ASSOCIATED, of a section that is the whole array and one that
! isn't; C.5.2's DYNAM_ITER, reading its shape from the terminal, and C.5.3's
! ITER, swapping IN and OUT (each in a subroutine named for its section, the
! construct ITER sharing its name); and C.12.8's BEST, which leaves its caller's
! pointer at the best element of an array passed as a pointer.
MODULE STD_TYPES
  IMPLICIT NONE
  TYPE REFERENCE
    INTEGER :: VOLUME, YEAR, PAGE
    CHARACTER (LEN = 50) :: TITLE
    CHARACTER, DIMENSION (:), POINTER :: ABSTRACT
  END TYPE REFERENCE
  TYPE NODE
    INTEGER :: VALUE
    TYPE (NODE), POINTER :: NEXT_NODE
  END TYPE
  TYPE TREE
    INTEGER :: KEY
    TYPE (TREE), POINTER :: LEFT, RIGHT
  END TYPE TREE
  TYPE INNER
    INTEGER :: COMPONENT
  END TYPE INNER
  TYPE OUTER
    INTEGER :: TAG
    TYPE (INNER) :: SUBSTRUCT
  END TYPE OUTER
CONTAINS
  FUNCTION POINTER_FUNCTION (ARG_1, ARG_2)
    REAL, DIMENSION (:), POINTER :: ARG_1
    INTEGER, INTENT (IN) :: ARG_2
    REAL, POINTER :: POINTER_FUNCTION
    POINTER_FUNCTION => ARG_1 (ARG_2)
  END FUNCTION POINTER_FUNCTION

  SUBROUTINE BEST (P, A)
    REAL, POINTER :: P
    REAL, POINTER :: A (:)
    INTEGER :: I (1)
    I = MAXLOC (A)
    P => A (I (1))                      ! Find the best element, A(I)
    RETURN
  END SUBROUTINE BEST
END MODULE STD_TYPES

PROGRAM PTRSTD
  USE STD_TYPES
  IMPLICIT NONE
  CALL EXAMPLE_4_4_1
  CALL EXAMPLE_5_1_2
  CALL EXAMPLE_6_3
  CALL EXAMPLE_7_5_2
  CALL EXAMPLE_13_13_13
  CALL EXAMPLE_C_5_2
  CALL EXAMPLE_C_5_3
  CALL EXAMPLE_C_12_8
CONTAINS
  SUBROUTINE EXAMPLE_4_4_1
    TYPE (REFERENCE) :: R
    TYPE (NODE), POINTER :: LIST, P
    CHARACTER (LEN = 60) :: TEXT
    INTEGER :: I, TOTAL
    TEXT = 'Pointers in Fortran 90.'
    R = REFERENCE (12, 1991, 207, 'Programming Notes', NULL_TEXT ())
    ALLOCATE (R % ABSTRACT (INDEX (TEXT, '.')))
    DO I = 1, SIZE (R % ABSTRACT)
      R % ABSTRACT (I) = TEXT (I:I)
    END DO
    PRINT '(1X, A, I3, I5, I4, 1X, A)', '4.4.1 ref', R % VOLUME, R % YEAR, R % PAGE, R % TITLE (1:17)
    PRINT '(1X, A, I3, 1X, 60A1)', '4.4.1 abstract', SIZE (R % ABSTRACT), R % ABSTRACT
    R % ABSTRACT (1:8) = R % ABSTRACT (8:1:-1)
    PRINT '(1X, A, 1X, 60A1)', '4.4.1 reversed', R % ABSTRACT (1:8)
    DEALLOCATE (R % ABSTRACT)
    ! A list of objects of type NODE, built at its head.
    NULLIFY (LIST)
    DO I = 1, 5
      ALLOCATE (P)
      P % VALUE = I * I
      P % NEXT_NODE => LIST
      LIST => P
    END DO
    TOTAL = 0
    P => LIST
    DO WHILE (ASSOCIATED (P))
      TOTAL = TOTAL + P % VALUE
      P => P % NEXT_NODE
    END DO
    PRINT '(1X, A, I4, I4, I4)', '4.4.1 list', LIST % VALUE, LIST % NEXT_NODE % NEXT_NODE % VALUE, TOTAL
    DO WHILE (ASSOCIATED (LIST))
      P => LIST
      LIST => LIST % NEXT_NODE
      DEALLOCATE (P)
    END DO
  END SUBROUTINE EXAMPLE_4_4_1

  FUNCTION NULL_TEXT ()
    CHARACTER, DIMENSION (:), POINTER :: NULL_TEXT
    NULLIFY (NULL_TEXT)
  END FUNCTION NULL_TEXT

  SUBROUTINE EXAMPLE_5_1_2
    TYPE (NODE), POINTER :: CURRENT, TAIL
    REAL, DIMENSION (:, :), POINTER :: IN, OUT, SWAP
    TYPE (NODE), TARGET :: HEAD
    REAL, DIMENSION (10, 10), TARGET :: A, B
    INTEGER :: I, J
    HEAD % VALUE = 0
    NULLIFY (HEAD % NEXT_NODE)
    TAIL => HEAD
    DO I = 1, 3
      ALLOCATE (CURRENT)
      CURRENT % VALUE = 10 * I
      NULLIFY (CURRENT % NEXT_NODE)
      TAIL % NEXT_NODE => CURRENT
      TAIL => CURRENT
    END DO
    PRINT '(1X, A, 4I4)', '5.1.2 chain', HEAD % VALUE, HEAD % NEXT_NODE % VALUE, &
          HEAD % NEXT_NODE % NEXT_NODE % VALUE, TAIL % VALUE
    A = RESHAPE ((/ ((REAL (10 * I + J), I = 1, 10), J = 1, 10) /), (/ 10, 10 /))
    B = -A
    IN => A
    OUT => B (2:4, 3:5)
    PRINT '(1X, A, 2I4, 2F7.1)', '5.1.2 in, out', SIZE (IN), SIZE (OUT), IN (10, 10), OUT (1, 1)
    SWAP => IN; IN => OUT; OUT => SWAP
    PRINT '(1X, A, 2I4, 2F7.1)', '5.1.2 swapped', SIZE (IN), SIZE (OUT), IN (1, 1), OUT (10, 10)
    CURRENT => HEAD % NEXT_NODE
    DO WHILE (ASSOCIATED (CURRENT))
      TAIL => CURRENT % NEXT_NODE
      DEALLOCATE (CURRENT)
      CURRENT => TAIL
    END DO
  END SUBROUTINE EXAMPLE_5_1_2

  SUBROUTINE EXAMPLE_6_3
    REAL, POINTER :: X (:)
    INTEGER, ALLOCATABLE :: B (:, :)
    INTEGER :: N, M, IERR_ALLOC, I, J
    N = 4
    M = 2
    ALLOCATE (X (N), B (-3 : M, 0:9), STAT = IERR_ALLOC)
    X = (/ (1.5 * I, I = 1, N) /)
    DO J = 0, 9
      DO I = -3, M
        B (I, J) = 10 * I + J
      END DO
    END DO
    PRINT '(1X, A, I2, 2(I3, I3), I4)', '6.3.1 allocated', IERR_ALLOC, LBOUND (B), UBOUND (B), SIZE (B)
    PRINT '(1X, A, 4F5.1, 3I4)', '6.3.1 values', X, B (-3, 0), B (M, 9), SUM (B (:, 9))
    DEALLOCATE (X, B)
    PRINT '(1X, A, 2L2)', '6.3.3 deallocated', ASSOCIATED (X), ALLOCATED (B)
    ALLOCATE (X (0), STAT = IERR_ALLOC)
    PRINT '(1X, A, I2, I3, L2)', '6.3.1 zero-sized', IERR_ALLOC, SIZE (X), ASSOCIATED (X)
    DEALLOCATE (X)
  END SUBROUTINE EXAMPLE_6_3

  SUBROUTINE EXAMPLE_7_5_2
    TYPE (TREE), POINTER :: NEW_NODE, CURRENT_NODE
    TYPE (OUTER), TARGET :: STRUCTURE
    INTEGER, POINTER :: SIMPLE_NAME
    REAL, TARGET :: MAT2D (5, 5)
    REAL, POINTER :: ROW (:), WINDOW (:, :), EVERY_OTHER (:), POINTER_OBJECT, ARG_1 (:)
    REAL, TARGET :: VECTOR (9)
    INTEGER :: I, J, N, ARG_2
    ALLOCATE (CURRENT_NODE, NEW_NODE)
    CURRENT_NODE % KEY = 4
    NULLIFY (CURRENT_NODE % LEFT, CURRENT_NODE % RIGHT)
    NEW_NODE % KEY = 7
    NULLIFY (NEW_NODE % RIGHT)
    NEW_NODE % LEFT => CURRENT_NODE
    PRINT '(1X, A, 2I3, 2L2)', '7.5.2 left', NEW_NODE % KEY, NEW_NODE % LEFT % KEY, &
          ASSOCIATED (NEW_NODE % RIGHT), ASSOCIATED (NEW_NODE % LEFT % LEFT)
    DEALLOCATE (NEW_NODE % LEFT)
    DEALLOCATE (NEW_NODE)

    STRUCTURE = OUTER (1, INNER (25))
    SIMPLE_NAME => STRUCTURE % SUBSTRUCT % COMPONENT
    SIMPLE_NAME = SIMPLE_NAME + 1
    PRINT '(1X, A, 2I4)', '7.5.2 component', STRUCTURE % SUBSTRUCT % COMPONENT, SIMPLE_NAME

    MAT2D = RESHAPE ((/ ((REAL (10 * I + J), J = 1, 5), I = 1, 5) /), (/ 5, 5 /), ORDER = (/ 2, 1 /))
    N = 3
    ROW => MAT2D (N, :)
    PRINT '(1X, A, I2, 5F5.0)', '7.5.2 row', SIZE (ROW), ROW
    I = 2
    J = 4
    WINDOW => MAT2D (I-1:I+1, J-1:J+1)
    PRINT '(1X, A, 3F5.0, 2X, 3F5.0, 2X, 3F5.0)', '7.5.2 window', (WINDOW (I, :), I = 1, 3)
    WINDOW = 0.0
    PRINT '(1X, A, F7.0)', '7.5.2 window cleared', SUM (MAT2D)

    VECTOR = (/ (REAL (I), I = 1, 9) /)
    ARG_1 => VECTOR
    ARG_2 = 6
    POINTER_OBJECT => POINTER_FUNCTION (ARG_1, ARG_2)
    POINTER_OBJECT = -POINTER_OBJECT
    PRINT '(1X, A, F5.0, L2)', '7.5.2 function', VECTOR (6), ASSOCIATED (POINTER_OBJECT, VECTOR (6))
    N = 9
    EVERY_OTHER => VECTOR (1:N:2)
    PRINT '(1X, A, I2, 5F5.0)', '7.5.2 every other', SIZE (EVERY_OTHER), EVERY_OTHER
    EVERY_OTHER = EVERY_OTHER * 10.0
    PRINT '(1X, A, 9F5.0)', '7.5.2 vector', VECTOR
  END SUBROUTINE EXAMPLE_7_5_2

  SUBROUTINE EXAMPLE_13_13_13
    TYPE (NODE), POINTER :: CURRENT, CUR, TOP
    TYPE (NODE), TARGET :: HEAD
    INTEGER, TARGET :: A (8)
    INTEGER, POINTER :: A_PART (:)
    INTEGER :: N
    CURRENT => HEAD
    PRINT '(1X, A, L2)', '13.13.13 current, head', ASSOCIATED (CURRENT, HEAD)
    A = 0
    N = UBOUND (A, DIM = 1)
    A_PART => A (:N)
    PRINT '(1X, A, I2, L2)', '13.13.13 a_part', N, ASSOCIATED (A_PART, A)
    N = 5
    A_PART => A (:N)
    PRINT '(1X, A, I2, L2)', '13.13.13 a_part', N, ASSOCIATED (A_PART, A)
    CUR => HEAD
    TOP => CUR
    PRINT '(1X, A, L2)', '13.13.13 cur, top', ASSOCIATED (CUR, TOP)
    NULLIFY (CUR); NULLIFY (TOP)
    PRINT '(1X, A, L2)', '13.13.13 nullified', ASSOCIATED (CUR, TOP)
  END SUBROUTINE EXAMPLE_13_13_13

  ! Each pass moves every value half way to I * J, exactly in binary, until no
  ! value moves by 0.01 or more.
  SUBROUTINE TRANSFORM (FROM, TO, CONVERGED)
    REAL, DIMENSION (:, :) :: FROM, TO
    LOGICAL :: CONVERGED
    INTEGER :: I, J
    DO J = 1, SIZE (FROM, 2)
      DO I = 1, SIZE (FROM, 1)
        TO (I, J) = (FROM (I, J) + REAL (I * J)) / 2.0
      END DO
    END DO
    CONVERGED = MAXVAL (ABS (TO - FROM)) < 0.01
  END SUBROUTINE TRANSFORM

  SUBROUTINE EXAMPLE_C_5_2
    REAL, DIMENSION (:, :), POINTER :: A, B, SWAP   ! Declare pointers
    INTEGER :: N, M, PASSES, I
    LOGICAL :: CONVERGED
    READ (*, *) N, M
    ALLOCATE (A (N, M), B (N, M))                   ! Allocate target arrays
    ! Read values into A
    READ (*, *) A
    PASSES = 0
    ITER: DO
      ! Apply transformation of values in A to produce values in B
      CALL TRANSFORM (A, B, CONVERGED)
      PASSES = PASSES + 1
      IF (CONVERGED) EXIT ITER
      ! Swap A and B
      SWAP => A; A => B; B => SWAP
    END DO ITER
    PRINT '(1X, A, 3I3)', 'C.5.2 DYNAM_ITER', N, M, PASSES
    DO I = 1, N
      PRINT '(1X, A, 6F8.3)', 'C.5.2', B (I, :)
    END DO
    DEALLOCATE (A, B)
  END SUBROUTINE EXAMPLE_C_5_2

  SUBROUTINE EXAMPLE_C_5_3
    REAL, DIMENSION (3, 4), TARGET :: A, B
    REAL, DIMENSION (:, :), POINTER :: IN, OUT, SWAP
    INTEGER :: PASSES
    LOGICAL :: CONVERGED
    ! Read values into A
    A = 0.0
    A (2, 3) = 64.0
    IN => A                     ! Associate IN with target A
    OUT => B                    ! Associate OUT with target B
    PASSES = 0
    ITER: DO
      ! Apply transformation of IN values to produce OUT
      CALL TRANSFORM (IN, OUT, CONVERGED)
      PASSES = PASSES + 1
      IF (CONVERGED) EXIT ITER
      ! Swap IN and OUT
      SWAP => IN; IN => OUT; OUT => SWAP
    END DO ITER
    PRINT '(1X, A, I3, 2L2, 2F8.3)', 'C.5.3 ITER', PASSES, ASSOCIATED (OUT, A), ASSOCIATED (OUT, B), &
          OUT (2, 3), OUT (3, 4)
  END SUBROUTINE EXAMPLE_C_5_3

  SUBROUTINE EXAMPLE_C_12_8
    REAL, POINTER :: PB (:), PBEST
    REAL, TARGET :: B (10000)
    INTEGER :: I
    PB => B
    ! Calculate B
    DO I = 1, SIZE (B)
      B (I) = REAL (MOD (37 * I, 1009))
    END DO
    CALL BEST (PBEST, PB)
    PRINT '(1X, A, F7.0, L2)', 'C.12.8 best', PBEST, ASSOCIATED (PBEST, B (709))
    PBEST = 0.0
    CALL BEST (PBEST, PB)
    PRINT '(1X, A, F7.0, L2)', 'C.12.8 next', PBEST, ASSOCIATED (PBEST, B (1718))
  END SUBROUTINE EXAMPLE_C_12_8
END PROGRAM PTRSTD
