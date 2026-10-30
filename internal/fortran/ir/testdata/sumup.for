      SUBROUTINE SUMUP(T, N, S)
      INTEGER N, T(10)
      S = 0
      DO 100 K = 1, N
  100 S = S + T(K)
      END
