C     A recursive unit's variable format expressions: a frame variable, a dummy
C     through ENTRY's cell, a temporary and a descriptor in the frame, each a
C     displacement from the base FTNRTL gives the expression's routine.
      RECURSIVE SUBROUTINE S(N, T)
      INTEGER N, K, L
      CHARACTER*(*) T
      CHARACTER*4 U
      ENTRY E(T, N)
      K = N + 1
      U = T
      IF (N .GT. 1) CALL E(T, N - 1)
      WRITE (6, 10) N
   10 FORMAT (I<K>, I<N>, I<L(U)>)
      END
