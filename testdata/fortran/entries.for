C     ENTRY statements (section 6.2.4; Phase 79, subtask 6): one body with
C     several ways in. Arguments in another order through the same names; a
C     dummy only an ENTRY names; control falling past an ENTRY from above; an
C     ENTRY before the declarations; arrays and passed lengths through cells.
C     Functions: the manual's hyperbolic functions; a value of a different type
C     for each entry point, returned as that entry point's type; names that
C     share a value; CHARACTER, passed-length, and group 2 values through a
C     hidden argument, which moves every entry point's arguments along.
      PROGRAM ENTRYS
      INTEGER N, K, A(3)
      REAL X, MYTANH, MYSINH, MYCOSH, RVAL
      INTEGER IVAL, I1, I2
      INTEGER*2 WVAL
      REAL*8 DVAL
      COMPLEX CVAL
      LOGICAL LVAL
      CHARACTER*5 UPPER, LOWER
      CHARACTER*8 BAR, DOTS
      REAL*16 QVAL
      COMPLEX*16 ZVAL
C     One subroutine's dummies, in two orders
      CALL SUBT(10, 3)
      CALL BUST(10, 3)
C     From the top through two ENTRY statements, from the second, and a
C     dummy only the last entry point names
      N = 0
      CALL STEPS(N)
      PRINT *, N
      CALL STEP2(N)
      PRINT *, N
      CALL GETN(K)
      PRINT *, K
C     An ENTRY before the declarations
      X = 3.0
      CALL EARLY(X)
      PRINT *, X
      CALL ALSO(X)
      PRINT *, X
C     An array and a passed length, by each entry point
      CALL FILL(A, 3)
      CALL SHOWA(3, A)
      CALL SHOWS('ABC')
      CALL AGAIN(7, 'DEFGH')
C     The manual's example: one type
      PRINT *, MYTANH(0.0)
      PRINT *, MYSINH(0.0)
      PRINT *, MYCOSH(0.0)
C     A value of each type
      PRINT *, RVAL(1.0)
      PRINT *, IVAL(1)
      PRINT *, WVAL(1)
      PRINT *, DVAL(1.25D0)
      PRINT *, CVAL(1.0)
      PRINT *, LVAL(1)
      PRINT *, RVAL(2.0)
C     Defining one name defines the others of its type
      PRINT *, I1(5)
      PRINT *, I2(6)
C     CHARACTER through a hidden argument
      PRINT *, UPPER('HELLO')
      PRINT *, LOWER('HELLO')
      PRINT *, BAR('-')
      PRINT *, '[' // DOTS(3, '*') // ']'
C     Group 2 through a hidden argument
      PRINT *, QVAL(2.5D0)
      PRINT *, ZVAL(0.0, 1.5D0)
      END

      SUBROUTINE SUBT(A, B)
      INTEGER A, B
      PRINT *, A - B
      RETURN
      ENTRY BUST(B, A)
      PRINT *, A - B
      END

      SUBROUTINE STEPS(N)
      DATA NCALLS /0/
      N = N + 100
      ENTRY STEP2(N)
      N = N + 10
      NCALLS = NCALLS + 1
      RETURN
      ENTRY GETN(M)
      M = NCALLS
      END

      SUBROUTINE EARLY(A)
      ENTRY ALSO(A)
      REAL A
      A = A * 2.0
      END

      SUBROUTINE FILL(V, N)
      INTEGER V(*), N
      DO 10 I = 1, N
   10 V(I) = I * I
      RETURN
      ENTRY SHOWA(N, V)
      PRINT *, V(N)
      PRINT *, V(N - 1)
      END

      SUBROUTINE SHOWS(S)
      CHARACTER*(*) S
      PRINT *, S
      PRINT *, LEN(S)
      RETURN
      ENTRY AGAIN(N, S)
      PRINT *, S(2:4)
      PRINT *, LEN(S) + N
      END

      REAL FUNCTION MYTANH(X)
      REAL MYSINH, MYCOSH
      MYTANH = (EXP(X) - EXP(-X)) / (EXP(X) + EXP(-X))
      RETURN
      ENTRY MYSINH(X)
      MYSINH = (EXP(X) - EXP(-X)) / 2.0
      RETURN
      ENTRY MYCOSH(X)
      MYCOSH = (EXP(X) + EXP(-X)) / 2.0
      END

      REAL FUNCTION RVAL(X)
      INTEGER IVAL
      INTEGER*2 WVAL
      REAL*8 DVAL, D
      COMPLEX CVAL
      LOGICAL LVAL
      RVAL = X + 0.5
      RETURN
      ENTRY IVAL(N)
      IVAL = N + 1
      RETURN
      ENTRY WVAL(N)
      WVAL = N + 2
      RETURN
      ENTRY DVAL(D)
      DVAL = D * 2.0D0
      RETURN
      ENTRY CVAL(X)
      CVAL = CMPLX(X, X + 1.0)
      RETURN
      ENTRY LVAL(N)
      LVAL = N .GT. 0
      END

      INTEGER FUNCTION I1(N)
      INTEGER I2
      ENTRY I2(N)
      I1 = N * 2
      END

      CHARACTER*5 FUNCTION UPPER(S)
      CHARACTER*(*) S
      CHARACTER*5 LOWER
      UPPER = S
      RETURN
      ENTRY LOWER(S)
      LOWER = S(2:)
      END

      CHARACTER*(*) FUNCTION BAR(C)
      CHARACTER C
      CHARACTER*(*) DOTS
      DO 10 I = 1, LEN(BAR)
   10 BAR(I:I) = C
      RETURN
      ENTRY DOTS(N, C)
      DOTS = ' '
      DO 20 I = 1, N
   20 DOTS(I:I) = C
      END

      REAL*16 FUNCTION QVAL(X)
      REAL*8 X
      COMPLEX*16 ZVAL
      QVAL = X
      RETURN
      ENTRY ZVAL(Y, X)
      ZVAL = DCMPLX(X, X + 1.0D0)
      END
