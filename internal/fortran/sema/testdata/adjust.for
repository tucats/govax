C     Adjustable arrays: bounds computed at each entry, into a descriptor
C     cell; an element reached through its A0.
      FUNCTION SUM(A, M, N)
      DIMENSION A(M, N)
      SUM = 0.0
      DO 10 J = 1, N
      DO 10 I = 1, M
10    SUM = SUM + A(I, J)
      END

C     Both bounds computed, a constant dimension, and a bound in COMMON.
      SUBROUTINE SUB(X, I, J, Y, Z)
      COMMON /SHAPE/ K
      DIMENSION X(-I/2:I/2, J), Y(2, 0:K), Z(3, K, *)
      X(I/2, J) = 999
      Y(1, I + 1) = 1.0
      Z(1, 2, J) = 2.0
      PRINT *, X, Y, SIZEOF(X)
      CALL OTHER(X, Y(1, 1))
      END

C     CHARACTER arrays, of a declared length and of a passed one.
      SUBROUTINE CHARS(A, B, N)
      CHARACTER*8 A(N)
      CHARACTER*(*) B(0:N)
      A(N) = B(N - 1)
      PRINT *, A, B
      END

C     An ENTRY's bounds are computed at its entry point, not by falling into
C     it; an entry point that doesn't name N doesn't compute them.
      SUBROUTINE E(A, N)
      COMPLEX*16 A(N)
      N = 1
      ENTRY E1(N, A)
      A(N) = (0.0D0, 1.0D0)
      RETURN
      ENTRY E2(A)
      END
