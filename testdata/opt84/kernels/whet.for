      PROGRAM WHET
C     A floating point mix in the manner of a synthetic benchmark:
C     common block arithmetic, a loop calling MTH$, array work.
      REAL E1(4), T, T1, T2, X, Y, Z, X1, X2, X3, X4
      INTEGER I, J, N, ITER
      COMMON /CTL/ T, T1, T2, J
      T = 0.499975
      T1 = 0.50025
      T2 = 2.0
      DO 100 ITER = 1, 200
      X1 = 1.0
      X2 = -1.0
      X3 = -1.0
      X4 = -1.0
      DO 10 I = 1, 50
         X1 = (X1 + X2 + X3 - X4)*T
         X2 = (X1 + X2 - X3 + X4)*T
         X3 = (X1 - X2 + X3 + X4)*T
         X4 = (-X1 + X2 + X3 + X4)*T
   10 CONTINUE
      E1(1) = 1.0
      E1(2) = -1.0
      E1(3) = -1.0
      E1(4) = -1.0
      DO 20 I = 1, 50
         CALL PA(E1)
   20 CONTINUE
      X = 0.5
      Y = 0.5
      DO 30 I = 1, 5
         X = T*ATAN(T2*SIN(X)*COS(X)/(COS(X+Y)+COS(X-Y)-1.0))
         Y = T*ATAN(T2*SIN(Y)*COS(Y)/(COS(X+Y)+COS(X-Y)-1.0))
   30 CONTINUE
      Z = 1.0
      DO 40 I = 1, 50
         CALL P3(X, Y, Z)
   40 CONTINUE
  100 CONTINUE
      PRINT *, X1, E1(4), Z
      END
      SUBROUTINE PA(E)
      REAL E(4), T, T1, T2
      INTEGER J
      COMMON /CTL/ T, T1, T2, J
      J = 0
   10 E(1) = (E(1) + E(2) + E(3) - E(4))*T
      E(2) = (E(1) + E(2) - E(3) + E(4))*T
      E(3) = (E(1) - E(2) + E(3) + E(4))*T
      E(4) = (-E(1) + E(2) + E(3) + E(4))/T2
      J = J + 1
      IF (J .LT. 6) GO TO 10
      END
      SUBROUTINE P3(X, Y, Z)
      REAL X, Y, Z, T, T1, T2, X1, Y1
      INTEGER J
      COMMON /CTL/ T, T1, T2, J
      X1 = X
      Y1 = Y
      X1 = T*(X1 + Y1)
      Y1 = T*(X1 + Y1)
      Z = (X1 + Y1)/T2
      END
