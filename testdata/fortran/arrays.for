      PROGRAM ARRAYS
C     Arrays (sections 2.2.3 and 4.5): lower bounds, several dimensions,
C     column-major order, and assumed-size dummy arguments.
      PARAMETER (N = 3)
      INTEGER M(N, 0:1), V(-2:2), S7(2,2,2,2,2,2,2), R(2.9)
      INTEGER GRID(-1:1, 2, 2)
C     The first subscript varies fastest.
      DO 10 J = 0, 1
      DO 10 I = 1, N
   10 M(I, J) = I + 10 * J
      PRINT *, M
      PRINT *, M(3, 0), M(1, 1)
C     Negative, zero, and positive subscripts.
      DO 20 I = -2, 2
   20 V(I) = I * I
      PRINT *, V
      PRINT *, V(-2), V(0), V(2)
C     A REAL subscript is truncated, and so is a REAL bound: R has 2.
      R(1) = 5
      R(2) = 6
      X = 2.9
      PRINT *, R(X), M(1.9, 0)
C     Three dimensions, through a one-dimensional dummy.
      DO 30 K = 1, 2
      DO 30 J = 1, 2
      DO 30 I = -1, 1
   30 GRID(I, J, K) = 100 * K + 10 * J + I
      PRINT *, GRID(-1, 1, 1), GRID(1, 2, 2)
      CALL LIST(GRID, 7, 12)
C     An assumed-size array with its own bounds sees the same storage.
      CALL SETTER(M, 99)
      PRINT *, M(2, 1), M(3, 1)
C     An array passed from an element starts there.
      CALL LIST(M(1, 1), 1, 3)
C     Seven dimensions: this element is number 85, from 0.
      DO 40 I = 1, 128
   40 CALL PUT(S7, I, 0)
      S7(2, 1, 2, 1, 2, 1, 2) = 7
      CALL FIND(S7, 128)
      END

      SUBROUTINE LIST(A, FIRST, LAST)
C     The elements from FIRST to LAST in storage order.
      INTEGER A(*), FIRST, LAST
      DO 10 I = FIRST, LAST
   10 PRINT *, I, A(I)
      END

      SUBROUTINE SETTER(A, VALUE)
C     Lower bounds on an assumed-size array: A(1, 0) is element number
C     1 + (0 + 1) * 3 = 4, from 0, which is its caller's M(2, 1).
      INTEGER A(0:2, -1:*), VALUE
      A(1, 0) = VALUE
      A(2, 0) = VALUE + 1
      END

      SUBROUTINE PUT(A, I, VALUE)
      INTEGER A(*), I, VALUE
      A(I) = VALUE
      END

      SUBROUTINE FIND(A, N)
      INTEGER A(*), N
      DO 10 I = 1, N
      IF (A(I) .NE. 0) PRINT *, I, A(I)
   10 CONTINUE
      END
