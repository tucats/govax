C Phase 89, subtask 4: a recursive unit's compiler's cells are in its frame
C (temporaries, a DO count and step, descriptors written whole at each use, an
C adjustable array's descriptor, ENTRY's argument cells and selector, a
C concatenation's text and a dynamic string), but a FORMAT's parsed form.
      RECURSIVE INTEGER FUNCTION F(N, A, S)
      INTEGER N, A(N), K, M, G
      CHARACTER*(*) S
      CHARACTER*8 T
      REAL X
      DO 10 K = 1, N, M
         A(K) = F(N - 1, A, S)
   10 CONTINUE
      T = S(1:2) // 'AB'
      CALL SUB(S // S, T)
      WRITE (6, 100) K, A, T
  100 FORMAT (I3, 4I5, A)
      F = K
      RETURN
      ENTRY G(X)
      G = X
      END
