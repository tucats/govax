      PROGRAM ARGS
C     Subroutines and functions, arguments by reference: an actual
C     argument changed, an expression's value passed, an array passed
C     whole and from an element on, and a function in an expression.
      INTEGER K, V(5)
      REAL X
      K = 5
      CALL TWICE(K)
      PRINT *, 'After TWICE(K), K is', K
      CALL TWICE(K + 1)
      CALL TWICE((K))
      PRINT *, 'After TWICE(K+1) and TWICE((K)), K is', K
      DO 10 I = 1, 5
   10 V(I) = I * 10
      PRINT *, 'ISUM(V,5) is', ISUM(V, 5), ', ISUM(V(3),3) is',
     1   ISUM(V(3), 3)
      CALL SCALE(V, 5, 3)
      PRINT *, V
      X = 1.5
      PRINT *, CUBE(X), CUBE(X + 1.0), CUBE(-2.0)
      PRINT *, 'ICLAMP:', ICLAMP(-4), ICLAMP(7), ICLAMP(20)
      PRINT *, 2 * ISUM(V, 2) + ICLAMP(K) - 1
      END
C
      SUBROUTINE TWICE(N)
      N = N * 2
      END
C
      INTEGER FUNCTION ISUM(A, N)
      INTEGER A(*), N
      ISUM = 0
      DO 10 I = 1, N
   10 ISUM = ISUM + A(I)
      END
C
      SUBROUTINE SCALE(A, N, F)
      INTEGER A(*), N, F
      DO 10 I = 1, N
   10 A(I) = A(I) * F
      RETURN
      END
C
      REAL FUNCTION CUBE(Y)
      CUBE = Y * Y * Y
      END
C
C     ICLAMP returns its argument held to 0 to 10, by a RETURN in
C     the middle of the function.
      FUNCTION ICLAMP(N)
      ICLAMP = 0
      IF (N .LT. 0) RETURN
      ICLAMP = 10
      IF (N .GT. 10) RETURN
      ICLAMP = N
      END
