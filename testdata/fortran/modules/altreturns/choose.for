C     The module of CHOOSE and its entry point AGAIN.
      SUBROUTINE CHOOSE(I, X, *, *)
      PRINT *, I, X
      RETURN I
      ENTRY AGAIN(*, X, I)
      PRINT *, I, X
      RETURN I
      END
