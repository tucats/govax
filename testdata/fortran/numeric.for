C     Numerical methods end to end (Phase 78, subtask 10): Newton's
C     method beside SQRT, a quadratic's roots real and complex, Simpson's
C     rule, pi from ATAN in three formats, degrees, logarithms, a mean
C     and standard deviation, compound interest, a linear system solved
C     by elimination, and Euclid's algorithm. Where a result's last digits
C     depend on rounding along the way, the program checks it against a
C     tolerance or prints it rounded, so its output is the mathematics'.
      PROGRAM NUMERIC
      REAL*8 X, XOLD, S, H, A(3,4), M, SOL(3)
      REAL*16 QPI
      REAL R, V(8), MEAN, SD
      COMPLEX ROOT1, ROOT2
      INTEGER I, J, K, N
      DATA V /2.0, 4.0, 4.0, 4.0, 5.0, 5.0, 7.0, 9.0/
      DATA A /2D0, -3D0, -2D0, 1D0, -1D0, 1D0, -1D0, 2D0, 2D0,
     1        8D0, -11D0, -3D0/
C     Newton's method for the square root of 2.
      X = 1
      N = 0
   10 XOLD = X
      X = (X + 2/X)/2
      N = N + 1
      IF (ABS(X - XOLD) .GT. 1D-15 .AND. N .LT. 20) GO TO 10
      PRINT *, N, X
      PRINT *, ABS(X - SQRT(2D0)) .LE. 2D-16
C     A quadratic's roots: real, then complex.
      CALL QUAD(1.0, -5.0, 6.0, ROOT1, ROOT2)
      PRINT *, ROOT1, ROOT2
      CALL QUAD(1.0, 2.0, 5.0, ROOT1, ROOT2)
      PRINT *, ROOT1, ROOT2
C     Simpson's rule for the integral of SIN from 0 to pi, which is 2.
      N = 100
      H = 4*ATAN(1D0)/N
      S = SIN(0D0) + SIN(N*H)
      DO 20 I = 1, N-1
         S = S + (2 + 2*MOD(I, 2))*SIN(I*H)
   20 CONTINUE
      S = S*H/3
      PRINT *, NINT(S*1D6), ABS(S - 2) .LT. 1D-7
C     Pi from ATAN(1), in F, D, and H.
      R = 4*ATAN(1.0)
      X = 4*DATAN(1D0)
      QPI = 4*ATAN(1Q0)
      PRINT *, R, X
      PRINT *, QPI
C     Degrees, logarithms, the hyperbolic identity, an absolute value.
      PRINT *, SIND(30.0), COSD(60.0), ATAN2D(1.0, 1.0), TAND(45.0)
      PRINT *, LOG10(1000D0), EXP(0.0), CABS((3.0, 4.0))
      X = 0.75D0
      PRINT *, ABS(COSH(X)**2 - SINH(X)**2 - 1) .LT. 1D-15, TANH(0.0)
C     The mean and standard deviation of eight values.
      MEAN = 0
      DO 30 I = 1, 8
         MEAN = MEAN + V(I)
   30 CONTINUE
      MEAN = MEAN/8
      SD = 0
      DO 40 I = 1, 8
         SD = SD + (V(I) - MEAN)**2
   40 CONTINUE
      SD = SQRT(SD/8)
      PRINT *, MEAN, SD
C     Compound interest: 5% for ten years, in hundredths.
      X = 100000*1.05D0**10
      PRINT *, NINT(X)
C     A system of three equations solved by elimination and back
C     substitution: x = 2, y = 3, z = -1.
      DO 60 K = 1, 2
         DO 60 I = K+1, 3
            M = A(I,K)/A(K,K)
            DO 50 J = K, 4
               A(I,J) = A(I,J) - M*A(K,J)
   50       CONTINUE
   60 CONTINUE
      DO 80 I = 3, 1, -1
         SOL(I) = A(I,4)
         DO 70 J = I+1, 3
            SOL(I) = SOL(I) - A(I,J)*SOL(J)
   70    CONTINUE
         SOL(I) = SOL(I)/A(I,I)
   80 CONTINUE
      PRINT *, SOL
C     Euclid's algorithm: greatest common divisors, a least common
C     multiple.
      PRINT *, IGCD(1071, 462), IGCD(17, 5), IGCD(0, 9)
      PRINT *, 1071/IGCD(1071, 462)*462
      END
C     QUAD finds the roots of A*x**2 + B*x + C, complex if they must be.
      SUBROUTINE QUAD(A, B, C, ROOT1, ROOT2)
      REAL A, B, C
      COMPLEX ROOT1, ROOT2, DISC
      DISC = CSQRT(CMPLX(B*B - 4*A*C))
      ROOT1 = (-B + DISC)/(2*A)
      ROOT2 = (-B - DISC)/(2*A)
      END
C     IGCD is the greatest common divisor of M and N, by Euclid.
      INTEGER FUNCTION IGCD(M, N)
      INTEGER M, N, I, J, K
      I = M
      J = N
   10 IF (J .EQ. 0) THEN
         IGCD = I
         RETURN
      END IF
      K = MOD(I, J)
      I = J
      J = K
      GO TO 10
      END
