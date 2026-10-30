C     Arguments, function values, and output items of every kind.
      PROGRAM CALLS
      INTEGER I, J, K, M(2,3), ISQ
      REAL X, Y, TOTAL
      LOGICAL OK
      I = 3
C     A function's value straight into its target, and through a
C     temporary when it is converted or part of an expression.
      J = ISQ(I)
      X = ISQ(I+1)
      K = ISQ(ISQ(2)) + ISQ(J)
C     A constant, an expression, a copy, a null argument, an element.
      CALL SHOW(5, I*2, (J), , M(2,K-1))
      CALL FILL(M, 6)
      Y = TOTAL(M, 6)
      M(1,1) = M(2,3) + M(I,J)
C     Output: constants, a dummy-free element, a logical, a function.
      OK = Y .GT. 10.0 .OR. I .EQ. 0
      PRINT *, 'M(1,1) IS', M(1,1), 1.5, OK, .TRUE., M(I,2), ISQ(4)
C     Loops: a REAL variable, a step that isn't constant, no trips.
      DO 10 X = 1.0, 2.0, 0.25
   10 CONTINUE
      DO 20 I = J, K, J - 1
         IF (I .GT. 10) GO TO 30
   20 CONTINUE
   30 DO 40 I = 5, 1
   40 CONTINUE
      IF (.NOT. OK) STOP 99
      IF (OK) CALL SHOW(1, 2, 3, 4, 5)
      END
C
      INTEGER FUNCTION ISQ(N)
      ISQ = N*N
      END
C
      SUBROUTINE FILL(A, N)
      INTEGER N, A(6)
      DO 10 I = 1, N
         A(I) = I
   10 CONTINUE
      PRINT *, N, A(N), A
      END
C
      SUBROUTINE SHOW(A, B, C, D, E)
      INTEGER A, B, C, D, E
      WRITE (*,*) A, B, C, D, E
      IF (A .LT. 0) RETURN
      STOP
      END
