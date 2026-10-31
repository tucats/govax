C     Procedures as arguments, dummy procedures, null arguments, Hollerith
C     constants, and the built-in functions (sections 6.1.1, 6.1.2).
      PROGRAM PROCS
      EXTERNAL SUBA, FUNB
      INTRINSIC SIN
      CHARACTER*6 CHF
      EXTERNAL CHF
      REAL*8 D
      DIMENSION A(3)
      CHARACTER*4 S
      CALL PASS(SUBA, FUNB, SIN, CHF)
      CALL NULLS(, X, )
      CALL HOLL(4HABCD)
      CALL OUT(%VAL(I), %VAL(X), %REF(D), %DESCR(D), %REF(S), %DESCR(A))
      CALL OUT(%REF(SUBA), %DESCR(SUBA), %REF(CHF), %DESCR(CHF))
      I = %LOC(X) + %LOC(A) + %LOC(A(2)) + %LOC(SUBA) + %LOC(S(2:3))
      END

      SUBROUTINE PASS(SUB, F, G, CH)
      EXTERNAL SUB, G
      CHARACTER*(*) CH
      EXTERNAL CH
      CHARACTER*12 T
      CALL SUB(1, *10)
      X = F(1.0) + G(2.0)
      T = CH(3)
      CALL AGAIN(SUB, F, G, CH)
      I = LEN(CH(3))
10    RETURN
      END

      SUBROUTINE NULLS(A, B, C)
      IF (%LOC(A) .EQ. 0) B = 1.0
      END

      SUBROUTINE HOLL(X)
      Y = X
      END

      SUBROUTINE CONST
      CALL NUMBER('ABCD')
      CALL NUMBER(%DESCR('ABCD'))
      CALL TEXT('ABCD')
      END

      SUBROUTINE NUMBER(X)
      Y = X
      END

      SUBROUTINE TEXT(X)
      CHARACTER*(*) X
      Y = LEN(X)
      END
