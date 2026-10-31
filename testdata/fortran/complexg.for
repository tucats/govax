C     complex.for in G_floating, each unit's OPTIONS statement saying so,
C     and a product only G_floating's range holds.
      OPTIONS /G_FLOATING
      PROGRAM CMPLXG
      COMPLEX C, D, E, CA(3), CSQ
      COMPLEX*16 Z, W, ZA(2)
      REAL X
      DOUBLE PRECISION DX
      INTEGER I
C     Constants, and list-directed output
      C = (1.0, 2.0)
      D = (3.0, -1.5)
      PRINT *, C, D
C     Sums, differences, negation, and mixed types
      E = C + D
      PRINT *, E, C - D, -C
      X = 0.5
      PRINT *, C + X, C*2, 1 - C
C     A product in line, and a quotient by OTS$DIVC
      PRINT *, C*D
      PRINT *, C/D, D/(0.0, 1.5)
      C = C*C
      PRINT *, C
      PRINT *, (1.0, 2.0)/(3.0, 4.0)
      E = (1.0, 1.0)/3
      PRINT *, E
C     A COMPLEX value converted to a REAL or an INTEGER is its real part
      X = D
      I = D
      PRINT *, X, I
C     Equal or not
      IF (C .EQ. (-3.0, 4.0)) PRINT *, 'EQUAL'
      IF (C .NE. D) PRINT *, 'NOT EQUAL'
      IF (D .EQ. 3.0) PRINT *, 'WRONG'
      IF (D .NE. (3.0, -1.5)) PRINT *, 'WRONG'
C     An array: subscripts that aren't constant, the whole array printed
      DO 10 I = 1, 3
         CA(I) = I*(1.0, -1.0)
   10 CONTINUE
      PRINT *, CA
      I = 2
      CA(I+1) = CA(I)*CA(I-1)
      PRINT *, CA(3)
C     A COMPLEX function, and a subroutine changing a COMPLEX argument
      PRINT *, CSQ((0.0, 1.0)) + 2
      CALL CSCALE(C, 0.5)
      PRINT *, C
      CALL CSCALE(CA(I), 2.0)
      PRINT *, CA(2)
C     COMPLEX*16: a product and a quotient by OTS$
      Z = (1D0, 2D0)
      W = Z*Z/(0.5D0, 0D0)
      PRINT *, W
      Z = (1D0, 1D0)/3
      PRINT *, Z
C     COMPLEX*8 with REAL*8 is COMPLEX*16, the REAL*8 not rounded
      DX = 0.1D0
      W = DX + C
      PRINT *, W
      W = C*Z
      PRINT *, W
C     An element of a COMPLEX*16 array, a subscript not constant
      I = 2
      ZA(I) = (0.25D0, -0.5D0)
      ZA(I-1) = ZA(I)*ZA(I)
      PRINT *, ZA
      W = (1D200, 2D0)
      W = W*(1D100, 0D0)
      PRINT *, W
      END
      OPTIONS /G_FLOATING
      COMPLEX FUNCTION CSQ(A)
      COMPLEX A
      CSQ = A*A
      END
      OPTIONS /G_FLOATING
      SUBROUTINE CSCALE(A, F)
      COMPLEX A
      A = A*F
      END
