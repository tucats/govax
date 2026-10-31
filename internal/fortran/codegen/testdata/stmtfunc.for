C     Statement functions (section 6.2.1): dummy arguments named for other
C     entities, every kind of actual argument, one function using another.
      PROGRAM SF
      REAL A(3), AREA, RING, R, H
      COMPLEX Z, SWAP, W
      CHARACTER*10 CSF, S, T
      CHARACTER*4 SHORT
      INTEGER*2 N, NEXT
      LOGICAL BIG
      AREA(A) = 3.0*A*A
      RING(R, H) = AREA(R) - AREA(H)
      SWAP(W) = CMPLX(AIMAG(W), REAL(W))
      CSF(S, T) = S(6:10)//T(1:5)
      SHORT(S) = S
      NEXT(N) = N + 1
      BIG(H) = H .GT. 2.0
      A(1) = 2.0
      A(2) = AREA(A(1)) + RING(A(1), 1.0)
      Y = AREA(A(1) + 1.0)
      Z = SWAP(Z)
      PRINT *, CSF('ABCDEFGHIJ', 'KL'), SHORT(S)
      N = NEXT(N) + NEXT(3)
      IF (BIG(Y)) Y = AREA(AREA(Y))
      END
