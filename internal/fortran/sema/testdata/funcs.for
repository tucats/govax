      PROGRAM FUNCS
      CHARACTER*5 NAME5, S
      CHARACTER*3 REPEAT
      REAL*16 QF, Q
      COMPLEX*16 ZF, Z
      S = NAME5(1)
      S = REPEAT('A') // 'X'
      Q = QF(Q)
      Z = ZF(Z)
      PRINT *, LEN(NAME5(2)), NAME5(3)
      END

      CHARACTER*5 FUNCTION NAME5(I)
      NAME5 = 'ABCDE'
      NAME5(I:I) = '-'
      END

      CHARACTER*(*) FUNCTION REPEAT(CARG)
      CHARACTER*1 CARG
      DO 10 I = 1, LEN(REPEAT)
10    REPEAT(I:I) = CARG
      END

      REAL*16 FUNCTION QF(X)
      REAL*16 X
      QF = X + 1
      END

      COMPLEX*16 FUNCTION ZF(X)
      COMPLEX*16 X
      ZF = X * X
      RETURN
      END
