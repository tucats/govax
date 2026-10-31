C     reals.for in G_floating, each unit's OPTIONS statement saying so,
C     and a value only G_floating's range holds.
      OPTIONS /G_FLOATING
      PROGRAM REALSG
      DOUBLE PRECISION D, E, THIRD, V(4), DSUM, HYPOT2
      REAL*16 Q, QTHIRD
      REAL X
      INTEGER I, N
C     A REAL*8 constant's digits, and a REAL*4's widened
      THIRD = 1D0/3D0
      PRINT *, THIRD
      X = 0.1
      D = X
      E = 0.1D0
      PRINT *, D, E
      X = E
      PRINT *, X
C     Mixed types: the integer division, then REAL*4, then REAL*8
      N = 7
      D = N/2 + X*N + E
      PRINT *, D
C     G editing's two forms at REAL*8's width
      PRINT *, 1D15, 1D16, 0.1D0
      PRINT *, 0.01D0, -12345.678D0, 0D0
C     REAL*16
      QTHIRD = 1Q0/3Q0
      PRINT *, QTHIRD
      Q = THIRD
      PRINT *, Q - QTHIRD
      Q = 2
      DO 10 I = 1, 4
         Q = Q*Q
   10 CONTINUE
      PRINT *, Q
C     A REAL*8 DO variable, an array, a sum
      DSUM = 0
      I = 0
      DO 20 D = 0.5D0, 2D0, 0.5D0
         I = I + 1
         V(I) = D*D
         DSUM = DSUM + V(I)
   20 CONTINUE
      PRINT *, V, DSUM
C     Conversion to an integer truncates
      D = -2.75D0
      I = D
      PRINT *, I
C     A REAL*8 function, and a subroutine changing a REAL*8 array
      PRINT *, HYPOT2(3D0, 4D0)
      CALL SCALE(V, 4, 10D0)
      PRINT *, V(1), V(4)
      IF (THIRD .LT. 0.3333333334D0 .AND. THIRD .GT. 0.3333333333D0)
     1   PRINT *, 'BETWEEN'
C     Expressions that need more registers than a statement has
      D = 0.5D0
      E = ((D+1)*(D+2) + (D+3)*(D+4))*((D+5)*(D+6) - (D+7)*(D+8))
      PRINT *, E
      Q = 0.5Q0
      Q = ((Q+1)*(Q+2) + (Q+3)*(Q+4))*((Q+5)*(Q+6) - (Q+7)*(Q+8))
      PRINT *, Q
      D = 1D300
      PRINT *, D*1D7, D/1D300
      END
      OPTIONS /G_FLOATING
      DOUBLE PRECISION FUNCTION HYPOT2(A, B)
      DOUBLE PRECISION A, B
      HYPOT2 = A*A + B*B
      END
      OPTIONS /G_FLOATING
      SUBROUTINE SCALE(V, N, F)
      DOUBLE PRECISION V(*), F
      DO 10 I = 1, N
   10 V(I) = V(I)*F
      END
