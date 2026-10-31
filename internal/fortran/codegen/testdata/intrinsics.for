C     In-line intrinsic functions: the ones that branch (ABS, MAX, MIN, DIM,
C     SIGN) through a temporary before the statement; MOD's division;
C     conversions, NINT, and ZEXT; COMPLEX parts; the bit functions on
C     words and longwords; ISHFTC by FTNRTL; the character functions.
      PROGRAM INTRINS
      INTEGER I, J, K, N(3)
      INTEGER*2 I2, J2
      LOGICAL L
      LOGICAL*2 L2
      REAL X, Y
      DOUBLE PRECISION D
      COMPLEX C
      CHARACTER*8 S
      CHARACTER*1 T
      I = ABS(I) + MAX(I, J, N(K)) - MIN0(J2, 5)
      X = SIGN(X, Y) + DIM(X, 1.0) + AMAX0(I, J)
      I = MOD(I, J) + IFIX(X) + NINT(D) + INT(I2)
      I2 = ININT(X) + IZEXT(I2) + ISIGN(I2, 3)
      X = AIMAG(C) + REAL(C) + DPROD(X, Y)
      C = CMPLX(X, I) + CONJG(C)
      I = IAND(I, 255) + IOR(I, J) + IEOR(I, -1) + NOT(J)
      I2 = IISHFT(I2, 3) + ISHFT(I2, -3) + IBSET(I2, 4)
      I = ISHFT(I, 5) + ISHFT(I, -5) + IBITS(I, J, 4) + IBITS(I2, 2, 3)
      I = IBCLR(I, J) + IBCLR(I2, J2) + ISHFTC(I, 3, 8)
      L = BTEST(I, 3)
      L2 = BTEST(I2, J2)
      I = LEN(S) + INDEX(S, 'AB') + ICHAR(S(I:I)) + INDEX(S(I:), S(J:K))
      T = CHAR(I)
      S = CHAR(J) // S(2:)
      L = LLT(S, 'M') .AND. LGE(S(I:J), T)
      END
