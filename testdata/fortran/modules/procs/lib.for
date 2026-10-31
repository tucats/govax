      REAL FUNCTION CALLF(F, X)
      CALLF = F(X)
      END

      SUBROUTINE CALLS(S, I)
      CALL S(I)
      END

      SUBROUTINE CALLCH(F)
      CHARACTER*(*) F
      PRINT *, F(1), LEN(F(1))
      END

      REAL FUNCTION TWICE(X)
      TWICE = 2.0 * X
      RETURN
      ENTRY HALF(X)
      HALF = X / 2.0
      END

      CHARACTER*(*) FUNCTION NAMER(I)
      NAMER = 'NAMED'
      END
