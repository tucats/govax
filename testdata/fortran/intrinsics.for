C     The intrinsic functions written in line (Phase 78, subtask 8): ABS,
C     MAX, MIN, DIM, SIGN, and MOD of every kind, generic and specific;
C     conversions, NINT, and ZEXT, with footnote 10's default integer;
C     COMPLEX parts; the bit functions on words and longwords, ISHFTC by
C     FTNRTL; the character functions, of passed-length dummies too; and
C     the branching ones in a condition, a DO loop's limit, a subscript,
C     and a statement that frees a dynamic string.
      PROGRAM INTRIN
      INTEGER I, J, K, M, N(4)
      INTEGER*2 I2, J2
      BYTE BB
      REAL X, Y
      DOUBLE PRECISION D
      COMPLEX C
      COMPLEX*16 Z
      CHARACTER*12 S
      CHARACTER*1 T
      DATA N /3, -7, 12, 5/
C     Choosing, of INTEGERs, REALs, and words.
      I = -5
      J = 3
      PRINT *, ABS(I), IABS(J), MAX(I, J, N(3)), MIN(N(1), N(2), N(4))
      X = -2.5
      Y = 0.0
      PRINT *, ABS(X), SIGN(X, Y), SIGN(3.0, X), DIM(X, -4.0)
      PRINT *, DIM(-4.0, X), AMIN1(X, Y, -1.0), DMAX1(-1D0, 2D0)
      K = 17
      PRINT *, MOD(K, 5), MOD(-K, 5), MOD(K, -5), ISIGN(J, I)
      PRINT *, IDIM(J, I), IDIM(I, J), ISIGN(I, 0), SIGN(-4, J)
      I2 = -300
      J2 = 7
      PRINT *, ABS(I2), MAX(I2, J2, 100), MIN0(I2, J2), MOD(I2, J2),
     1         ISIGN(J2, I2)
C     Conversions, and footnote 10's results.
      X = 2.5
      D = -2.5D0
      PRINT *, INT(X), NINT(X), NINT(D), IDNINT(D), IFIX(-X), INT(I2)
      PRINT *, ININT(X), MAX1(X, 1.2), AMAX0(I, J), REAL(J), DBLE(X)
      I2 = -1
      BB = -2
      PRINT *, ZEXT(I2), IZEXT(BB), JZEXT(BB), NINT(-X)
C     COMPLEX parts.
      C = (3.0, -4.0)
      PRINT *, REAL(C), AIMAG(C), CONJG(C)
      PRINT *, CMPLX(X, 2), CMPLX(J)
      Z = DCMPLX(D, X)
      PRINT *, DREAL(Z), DIMAG(Z), DPROD(X, 3.0)
C     The bit functions.
      I = 12
      J = 10
      K = -1
      PRINT *, IAND(I, J), IOR(I, J), IEOR(I, J), NOT(I)
      PRINT *, ISHFT(I, 4), ISHFT(K, -28), ISHFT(I, -2), ISHFT(I, 32)
      PRINT *, IBITS(K, 4, 8), IBITS(I, 2, J-8), IBSET(I, 0),
     1         IBCLR(I, 3)
      I2 = 1
      J2 = -1
      PRINT *, ISHFT(I2, 15), ISHFT(J2, -12), IBSET(I2, 15),
     1         IBCLR(J2, 0), IBITS(J2, 0, 16)
      PRINT *, BTEST(I, 2), BTEST(I, 0), BTEST(J2, 15), BTEST(I2, 1)
      I = 6
      K = 70
      M = -2147483647
      PRINT *, ISHFTC(I, 1, 3), ISHFTC(I, -1, 3), ISHFTC(I, 2, 4),
     1         ISHFTC(K, 1, 3), ISHFTC(M, 1, 32), ISHFTC(M, -1, 32)
      PRINT *, ISHFTC(I2, -1, 16), ISHFTC(J2, 3, 8), ISHFTC(K, J, 32)
C     The character functions.
      S = 'HELLO, WORLD'
      PRINT *, LEN(S), INDEX(S, 'O'), INDEX(S, 'WORLD'), INDEX(S, 'XY'),
     1         ICHAR(S(8:8))
      I = 6
      PRINT *, INDEX(S(I:), 'O'), INDEX(S(I:), S(1:1)), LEN(S(I:)),
     1         LEN(S(I:I+2)//'AB'), INDEX(S, S(I+3:I+4))
      CALL SHOW('ABC')
      CALL SHOW(S(1:I))
      T = CHAR(I + 59)
      S = CHAR(72) // CHAR(I+99) // 'Y'
      PRINT *, T, S, '|'
      PRINT *, LLT('ABC', S), LGE(S, 'HI'), LGT(S(1:1), T), LLE(T, 'A'),
     1         LLT(S, 'H')
C     Choices in a condition, a DO loop's limit, and a subscript.
      J = 3
      IF (MAX(I, J) .GT. 5 .AND. MIN(I, J) .LT. 5) PRINT *, 'BOTH'
      DO 10 K = 1, MIN(J, 2)
         PRINT *, K, N(MAX(K, 2)), ABS(N(MIN(K+1, 4)))
   10 CONTINUE
      CALL SHORT(2.5)
      END

C     LEN, INDEX, and ICHAR of a passed-length dummy, and a statement whose
C     concatenation's dynamic string is freed after MAX's branches.
      SUBROUTINE SHOW(A)
      CHARACTER*(*) A
      PRINT *, LEN(A), INDEX(A, 'L'), ICHAR(A), ' ', A
      PRINT *, A // '!', MAX(LEN(A), 4), MIN(INDEX(A, 'B'), 7), '|'
      END

C     Footnote 10 under /NOI4: NINT, INT, and MAX1 are INTEGER*2.
      OPTIONS /NOI4
      SUBROUTINE SHORT(X)
      PRINT *, NINT(X), INT(X), MAX1(X, 0.0), IDNINT(-1.5D0)
      END
