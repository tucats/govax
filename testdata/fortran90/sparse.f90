! A sparse matrix (Phase 107, subtask 8): each row a linked list of its
! nonzero entries in column order, the rows an array pointer component of the
! matrix; a module whose procedures set an entry (inserting, replacing, or
! unlinking and freeing it when the value is zero), find one (a pointer
! function whose target the caller changes), copy, transpose, add (merging
! two rows' lists), multiply by a vector (operator *, its value's extent the
! matrix's row count) and by a matrix (operator *, each entry of A scaling a
! row of B into the product's row), make a diagonal matrix from a vector
! (assignment, beside the type's own; given an array constant and an array
! expression), count, show, and free a matrix; a matrix
! function's value given its rows by ALLOCATE; intrinsic assignment copying
! the rows' pointer, so two matrices share their entries, and a deep copy
! that doesn't; the main program walking a row by a pointer of its own.
! Every value is a whole number, so every sum is exact.
MODULE SPARSE_MATRICES
  IMPLICIT NONE
  TYPE ENTRY
    INTEGER :: COL
    REAL :: VAL
    TYPE (ENTRY), POINTER :: NEXT
  END TYPE ENTRY
  TYPE ROW
    TYPE (ENTRY), POINTER :: FIRST
  END TYPE ROW
  TYPE MATRIX
    INTEGER :: N, M
    TYPE (ROW), POINTER :: ROWS (:)
  END TYPE MATRIX
  INTEGER :: LIVE = 0
  INTERFACE OPERATOR (+)
    MODULE PROCEDURE PLUS
  END INTERFACE
  INTERFACE OPERATOR (*)
    MODULE PROCEDURE TIMES_VECTOR, TIMES_MATRIX
  END INTERFACE
  INTERFACE ASSIGNMENT (=)
    MODULE PROCEDURE DIAGONAL
  END INTERFACE
CONTAINS
  SUBROUTINE CREATE (A, N, M)
    TYPE (MATRIX), INTENT (OUT) :: A
    INTEGER, INTENT (IN) :: N, M
    INTEGER :: I
    A % N = N
    A % M = M
    ALLOCATE (A % ROWS (N))
    DO I = 1, N
      NULLIFY (A % ROWS (I) % FIRST)
    END DO
  END SUBROUTINE CREATE

  SUBROUTINE SET (A, I, J, V)
    TYPE (MATRIX), INTENT (INOUT) :: A
    INTEGER, INTENT (IN) :: I, J
    REAL, INTENT (IN) :: V
    TYPE (ENTRY), POINTER :: P, PREV, NEW
    NULLIFY (PREV)
    P => A % ROWS (I) % FIRST
    DO WHILE (ASSOCIATED (P))
      IF (P % COL >= J) EXIT
      PREV => P
      P => P % NEXT
    END DO
    IF (ASSOCIATED (P)) THEN
      IF (P % COL == J) THEN
        IF (V /= 0.0) THEN
          P % VAL = V
        ELSE
          IF (ASSOCIATED (PREV)) THEN
            PREV % NEXT => P % NEXT
          ELSE
            A % ROWS (I) % FIRST => P % NEXT
          END IF
          DEALLOCATE (P)
          LIVE = LIVE - 1
        END IF
        RETURN
      END IF
    END IF
    IF (V == 0.0) RETURN
    ALLOCATE (NEW)
    LIVE = LIVE + 1
    NEW % COL = J
    NEW % VAL = V
    NEW % NEXT => P
    IF (ASSOCIATED (PREV)) THEN
      PREV % NEXT => NEW
    ELSE
      A % ROWS (I) % FIRST => NEW
    END IF
  END SUBROUTINE SET

  SUBROUTINE DIAGONAL (A, X)
    TYPE (MATRIX), INTENT (OUT) :: A
    REAL, INTENT (IN) :: X (:)
    INTEGER :: K
    CALL CREATE (A, SIZE (X), SIZE (X))
    DO K = 1, SIZE (X)
      CALL SET (A, K, K, X (K))
    END DO
  END SUBROUTINE DIAGONAL

  FUNCTION FIND (A, I, J) RESULT (P)
    TYPE (MATRIX), INTENT (IN) :: A
    INTEGER, INTENT (IN) :: I, J
    TYPE (ENTRY), POINTER :: P
    P => A % ROWS (I) % FIRST
    DO WHILE (ASSOCIATED (P))
      IF (P % COL == J) RETURN
      IF (P % COL > J) EXIT
      P => P % NEXT
    END DO
    NULLIFY (P)
  END FUNCTION FIND

  REAL FUNCTION GET (A, I, J)
    TYPE (MATRIX), INTENT (IN) :: A
    INTEGER, INTENT (IN) :: I, J
    TYPE (ENTRY), POINTER :: P
    P => FIND (A, I, J)
    GET = 0.0
    IF (ASSOCIATED (P)) GET = P % VAL
  END FUNCTION GET

  INTEGER FUNCTION NONZEROS (A)
    TYPE (MATRIX), INTENT (IN) :: A
    TYPE (ENTRY), POINTER :: P
    INTEGER :: I
    NONZEROS = 0
    DO I = 1, A % N
      P => A % ROWS (I) % FIRST
      DO WHILE (ASSOCIATED (P))
        NONZEROS = NONZEROS + 1
        P => P % NEXT
      END DO
    END DO
  END FUNCTION NONZEROS

  SUBROUTINE SHOW (A, TITLE)
    TYPE (MATRIX), INTENT (IN) :: A
    CHARACTER (LEN = *), INTENT (IN) :: TITLE
    INTEGER :: I, J
    PRINT '(1X, A, I3, A, I2, A, I2)', TITLE, NONZEROS (A), ' of', A % N, ' by', A % M
    DO I = 1, A % N
      PRINT '(3X, 8F6.0)', (GET (A, I, J), J = 1, A % M)
    END DO
  END SUBROUTINE SHOW

  FUNCTION COPY (A) RESULT (C)
    TYPE (MATRIX), INTENT (IN) :: A
    TYPE (MATRIX) :: C
    TYPE (ENTRY), POINTER :: P
    INTEGER :: I
    CALL CREATE (C, A % N, A % M)
    DO I = 1, A % N
      P => A % ROWS (I) % FIRST
      DO WHILE (ASSOCIATED (P))
        CALL SET (C, I, P % COL, P % VAL)
        P => P % NEXT
      END DO
    END DO
  END FUNCTION COPY

  FUNCTION TRANSPOSED (A) RESULT (T)
    TYPE (MATRIX), INTENT (IN) :: A
    TYPE (MATRIX) :: T
    TYPE (ENTRY), POINTER :: P
    INTEGER :: I
    CALL CREATE (T, A % M, A % N)
    DO I = 1, A % N
      P => A % ROWS (I) % FIRST
      DO WHILE (ASSOCIATED (P))
        CALL SET (T, P % COL, I, P % VAL)
        P => P % NEXT
      END DO
    END DO
  END FUNCTION TRANSPOSED

  ! Each row of the sum merges the two rows' lists, appending at a tail.
  FUNCTION PLUS (A, B) RESULT (C)
    TYPE (MATRIX), INTENT (IN) :: A, B
    TYPE (MATRIX) :: C
    TYPE (ENTRY), POINTER :: P, Q, TAIL
    INTEGER :: I, COL
    REAL :: V
    CALL CREATE (C, A % N, A % M)
    DO I = 1, A % N
      P => A % ROWS (I) % FIRST
      Q => B % ROWS (I) % FIRST
      NULLIFY (TAIL)
      DO WHILE (ASSOCIATED (P) .OR. ASSOCIATED (Q))
        IF (.NOT. ASSOCIATED (Q)) THEN
          COL = P % COL
          V = P % VAL
          P => P % NEXT
        ELSE IF (.NOT. ASSOCIATED (P)) THEN
          COL = Q % COL
          V = Q % VAL
          Q => Q % NEXT
        ELSE IF (P % COL < Q % COL) THEN
          COL = P % COL
          V = P % VAL
          P => P % NEXT
        ELSE IF (Q % COL < P % COL) THEN
          COL = Q % COL
          V = Q % VAL
          Q => Q % NEXT
        ELSE
          COL = P % COL
          V = P % VAL + Q % VAL
          P => P % NEXT
          Q => Q % NEXT
        END IF
        IF (V == 0.0) CYCLE
        IF (ASSOCIATED (TAIL)) THEN
          ALLOCATE (TAIL % NEXT)
          TAIL => TAIL % NEXT
        ELSE
          ALLOCATE (C % ROWS (I) % FIRST)
          TAIL => C % ROWS (I) % FIRST
        END IF
        LIVE = LIVE + 1
        TAIL % COL = COL
        TAIL % VAL = V
        NULLIFY (TAIL % NEXT)
      END DO
    END DO
  END FUNCTION PLUS

  FUNCTION TIMES_VECTOR (A, X) RESULT (Y)
    TYPE (MATRIX), INTENT (IN) :: A
    REAL, INTENT (IN) :: X (:)
    REAL :: Y (A % N)
    TYPE (ENTRY), POINTER :: P
    INTEGER :: I
    DO I = 1, A % N
      Y (I) = 0.0
      P => A % ROWS (I) % FIRST
      DO WHILE (ASSOCIATED (P))
        Y (I) = Y (I) + P % VAL * X (P % COL)
        P => P % NEXT
      END DO
    END DO
  END FUNCTION TIMES_VECTOR

  FUNCTION TIMES_MATRIX (A, B) RESULT (C)
    TYPE (MATRIX), INTENT (IN) :: A, B
    TYPE (MATRIX) :: C
    TYPE (ENTRY), POINTER :: P, Q
    INTEGER :: I
    CALL CREATE (C, A % N, B % M)
    DO I = 1, A % N
      P => A % ROWS (I) % FIRST
      DO WHILE (ASSOCIATED (P))
        Q => B % ROWS (P % COL) % FIRST
        DO WHILE (ASSOCIATED (Q))
          CALL SET (C, I, Q % COL, GET (C, I, Q % COL) + P % VAL * Q % VAL)
          Q => Q % NEXT
        END DO
        P => P % NEXT
      END DO
    END DO
  END FUNCTION TIMES_MATRIX

  SUBROUTINE DESTROY (A)
    TYPE (MATRIX), INTENT (INOUT) :: A
    TYPE (ENTRY), POINTER :: P
    INTEGER :: I
    DO I = 1, A % N
      DO WHILE (ASSOCIATED (A % ROWS (I) % FIRST))
        P => A % ROWS (I) % FIRST
        A % ROWS (I) % FIRST => P % NEXT
        DEALLOCATE (P)
        LIVE = LIVE - 1
      END DO
    END DO
    DEALLOCATE (A % ROWS)
  END SUBROUTINE DESTROY
END MODULE SPARSE_MATRICES

PROGRAM SPARSE
  USE SPARSE_MATRICES
  IMPLICIT NONE
  TYPE (MATRIX) :: A, B, C, D, G, S, L, CT, R, NEG, RN
  TYPE (ENTRY), POINTER :: E
  REAL, POINTER :: Y (:)
  REAL :: X (5)
  INTEGER :: I

  ! A tridiagonal matrix, set out of order, one entry set twice.
  CALL CREATE (A, 5, 5)
  DO I = 5, 1, -1
    CALL SET (A, I, I, 2.0)
    IF (I > 1) CALL SET (A, I, I - 1, -1.0)
    IF (I < 5) CALL SET (A, I, I + 1, -1.0)
  END DO
  CALL SET (A, 1, 5, 7.0)
  CALL SET (A, 3, 3, 4.0)
  CALL SHOW (A, 'A:')
  PRINT '(1X, A, I3)', 'live:', LIVE

  ! Entries unlinked from a row's head, middle, and end, and one never there.
  CALL SET (A, 2, 1, 0.0)
  CALL SET (A, 3, 3, 0.0)
  CALL SET (A, 1, 5, 0.0)
  CALL SET (A, 4, 1, 0.0)
  PRINT '(1X, A, 2I3, 3F5.0)', 'unlinked:', NONZEROS (A), LIVE, GET (A, 2, 1), GET (A, 3, 3), GET (A, 3, 4)

  ! An entry changed through the pointer FIND gives back.
  E => FIND (A, 4, 5)
  E % VAL = 9.0
  PRINT '(1X, A, F5.0, L2)', 'found:', GET (A, 4, 5), ASSOCIATED (FIND (A, 4, 1))

  ! A row walked by the program's own pointer.
  E => A % ROWS (3) % FIRST
  DO WHILE (ASSOCIATED (E))
    PRINT '(1X, A, I2, F5.0)', 'row 3:', E % COL, E % VAL
    E => E % NEXT
  END DO

  ! A times a vector, into a pointer's target.
  X = (/ 1.0, 2.0, 3.0, 4.0, 5.0 /)
  ALLOCATE (Y (5))
  Y = A * X
  PRINT '(1X, A, 5F6.0)', 'A x:', Y
  DEALLOCATE (Y)

  ! Diagonal matrices by the defined assignment, from a constant (its zero
  ! not stored) and from an expression.
  G = (/ 3.0, 0.0, 1.0 /)
  CALL SHOW (G, 'G = diagonal:')
  CALL DESTROY (G)
  G = X * 2.0 - 1.0
  PRINT '(1X, A, I3, 5F6.0)', 'G:', NONZEROS (G), G * (X - X + 1.0)
  CALL DESTROY (G)

  ! The transpose, and A plus it: a symmetric matrix.
  B = TRANSPOSED (A)
  CALL SHOW (B, 'B = transposed A:')
  S = A + B
  CALL SHOW (S, 'S = A + B:')
  PRINT '(1X, A, I3)', 'live:', LIVE

  ! A copy shares A's entries; a deep copy doesn't.
  C = A
  D = COPY (A)
  CALL SET (C, 1, 1, 50.0)
  CALL SET (D, 1, 1, 60.0)
  PRINT '(1X, A, 3F5.0, L2)', 'shared:', GET (A, 1, 1), GET (C, 1, 1), GET (D, 1, 1), &
        ASSOCIATED (C % ROWS, A % ROWS)
  CALL SET (A, 1, 1, 2.0)
  CALL DESTROY (D)
  NULLIFY (C % ROWS)

  ! A rectangular product: A times a 5 by 3 matrix.
  CALL CREATE (C, 5, 3)
  DO I = 1, 5
    CALL SET (C, I, MOD (I, 3) + 1, REAL (I))
  END DO
  CALL SET (C, 3, 3, -1.0)
  D = A * C
  CALL SHOW (D, 'D = A C:')
  PRINT '(1X, A, 5F6.0)', 'D (1, 1, 1):', D * (/ 1.0, 1.0, 1.0 /)

  ! A sum that cancels: (A C)' less C' A', every entry unlinked.
  CALL DESTROY (S)
  L = TRANSPOSED (D)
  CT = TRANSPOSED (C)
  R = CT * B
  NEG = MINUS_ONE (3)
  RN = NEG * R
  S = L + RN
  PRINT '(1X, A, I3, I3)', '(A C)'' - C'' A'':', NONZEROS (S), NONZEROS (L)

  CALL DESTROY (A)
  CALL DESTROY (B)
  CALL DESTROY (C)
  CALL DESTROY (D)
  CALL DESTROY (S)
  CALL DESTROY (L)
  CALL DESTROY (CT)
  CALL DESTROY (R)
  CALL DESTROY (NEG)
  CALL DESTROY (RN)
  PRINT '(1X, A, I3)', 'live at the end:', LIVE
CONTAINS
  ! The identity of order N times -1, so that a product can be subtracted.
  FUNCTION MINUS_ONE (N) RESULT (M)
    INTEGER, INTENT (IN) :: N
    TYPE (MATRIX) :: M
    INTEGER :: K
    CALL CREATE (M, N, N)
    DO K = 1, N
      CALL SET (M, K, K, -1.0)
    END DO
  END FUNCTION MINUS_ONE
END PROGRAM SPARSE
