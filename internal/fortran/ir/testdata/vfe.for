C     Variable format expressions: a FORMAT's routine, with a dummy argument,
C     a call, a conversion, and a value computed before a dynamic string goes.
      SUBROUTINE VFE(N, S)
      CHARACTER*(*) S
   10 FORMAT (1X, <N>(I<F(N)>.<N-1>), E12.4E<X>, -<N>P)
   20 FORMAT (I<LEN(S//'A')>, T<N>)
      WRITE (6, 10) (I, I = 1, N), X
      WRITE (6, 20) N
      ASSIGN 10 TO K
      WRITE (6, K) 1
      END
