C     A value kept in a common block, set by one entry point and read by
C     another's in another unit; and a function with a value of each of two
C     types.
      SUBROUTINE KEEP
      COMMON /KEPT/ V
      ENTRY SETVAL(X)
      V = X
      END

      REAL FUNCTION ONCE(X)
      REAL*8 TWICE
      ONCE = X
      RETURN
      ENTRY TWICE(X)
      TWICE = X * 2.0
      END

      REAL FUNCTION VALUE()
      COMMON /KEPT/ V
      ENTRY GETVAL()
      GETVAL = V
      END
