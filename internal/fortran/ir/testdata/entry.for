C     ENTRY statements: a subroutine's dummies in two orders, one only an
C     ENTRY names, an array and a passed length through their cells; a
C     function whose entry points' values are of two types; a CHARACTER
C     function, whose hidden argument moves every list along.
      SUBROUTINE SUB(A, B)
      REAL A, B, V(*), C
      CHARACTER*(*) S
      A = B
      ENTRY BUS(B, A)
      B = A + 1.0
      RETURN
      ENTRY ARR(I, V)
      V(I) = 2.0
      C = V(I + 1)
      ENTRY TXT(S)
      S(1:1) = 'X'
      I = LEN(S)
      END

      REAL FUNCTION F(X)
      REAL*8 G
      INTEGER H
      F = X
      IF (X .GT. 0.0) RETURN
      ENTRY H(X)
      H = 1
      RETURN
      ENTRY G
      G = 2.0D0
      END

      REAL FUNCTION P(X)
      ENTRY Q(Y, X)
      Q = X
      END

      CHARACTER*4 FUNCTION CF(N)
      CHARACTER*4 CG
      CF = 'AB'
      RETURN
      ENTRY CG(M, N)
      CG = 'CD'
      END

      PROGRAM CALLER
      REAL*8 G
      INTEGER H
      CHARACTER*4 CG
      CALL BUS(X, Y)
      Y = G()
      J = H(X)
      PRINT *, CG(1, 2)
      END
