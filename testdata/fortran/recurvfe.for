C     Variable format expressions in routines active more than once (Phase 89,
C     subtask 6). FTNRTL gives an expression the FP of the routine doing the
C     I/O, so it reaches that activation's frame, whichever entered last.
      PROGRAM RECVFE
      INTEGER IA(3), DEEP
      CHARACTER*14 SR, SA, TR, TA
      COMMON /FROMAST/ TR, TA, NEXT
      DATA IA /2, 3, 4/
C
C     Each activation's own local and dummy, read after the deeper ones ran.
      CALL DOWN(3, 2)
C
C     A dummy through ENTRY's cell, and an adjustable array's bound.
      CALL EDOWN(IA, 3)
C
C     A call in the expression, its value in a temporary of the frame, and a
C     local read after it. WIDTH has expressions of its own, on an internal file.
      CALL CALLER(2)
C
C     An expression that recurses: each activation's statement waits for
C     the next one's, and each internal file is in its own frame.
      PRINT 100, DEEP(3), DEEP(1)
  100 FORMAT (1X, 2I3)
C
C     A character value in an expression, and enough of an expression to be
C     set aside in the frame.
      CALL TEXT('ABC', 2)
      CALL SPILL(1, 3)
C
C     A loop keeping values in registers around an I/O statement with an
C     expression: the frame's base in the expression is kept from them.
      CALL LOOPER(4)
C
C     Entered again by an AST in the middle of an expression: the rest of the
C     statement still finds the first activation's frame.
      NEXT = 1
      CALL SHARED(5, SR)
      PRINT 110, SR, TR
C
C     The same in a unit that isn't RECURSIVE, with an AUTOMATIC variable.
      NEXT = 2
      CALL AUTOS(5, SA)
      PRINT 110, SA, TA
  110 FORMAT (1X, '[', A, '] [', A, ']')
      END

      RECURSIVE SUBROUTINE DOWN(N, W)
      INTEGER N, W, K
      K = N + 1
      IF (N .GT. 1) CALL DOWN(N - 1, W + 1)
      WRITE (6, 10) N, K
   10 FORMAT (1X, I<W>, I<K>)
      END

      RECURSIVE SUBROUTINE EDOWN(A, N)
      INTEGER N, A(N)
      ENTRY EUP(N, A)
      IF (N .GT. 1) CALL EUP(N - 1, A)
      WRITE (6, 10) N
   10 FORMAT (1X, I<A(N)>)
      END

      RECURSIVE SUBROUTINE CALLER(L)
      INTEGER L, WIDTH, K
      K = L + 1
      WRITE (6, 10) 7, 8, 9
   10 FORMAT (1X, I<WIDTH(3) + L>, I<K>, I<L>)
      END

C     WIDTH(N) is N, by way of its own activations and an internal file.
      RECURSIVE INTEGER FUNCTION WIDTH(N) RESULT(M)
      INTEGER N, K, V
      CHARACTER*8 S
      K = N + 1
      IF (N .GT. 1) THEN
         M = WIDTH(N - 1) + 1
      ELSE
         M = 1
      END IF
      WRITE (S, 10) N
      READ (S, 10) V
   10 FORMAT (I<K>)
      M = M + V - N
      END

C     DEEP(N) is N: the place of the digit N in a field N wide.
      RECURSIVE INTEGER FUNCTION DEEP(N) RESULT(M)
      INTEGER N
      CHARACTER*8 S
      IF (N .LE. 1) THEN
         M = 1
      ELSE
         S = ' '
         WRITE (S, 10) N
   10    FORMAT (I<DEEP(N - 1) + 1>)
         M = INDEX(S, CHAR(48 + N))
      END IF
      END

      RECURSIVE SUBROUTINE TEXT(T, N)
      CHARACTER*(*) T
      CHARACTER*4 U
      INTEGER N, ILEN
      U = 'WXYZ'
      IF (N .GT. 1) CALL TEXT(T // 'D', N - 1)
      WRITE (6, 10) N, N, N
   10 FORMAT (1X, I<LEN(T // U)>, I<ILEN(U(:N))>, I<ILEN(T)>)
      END

      INTEGER FUNCTION ILEN(S)
      CHARACTER*(*) S
      ILEN = LEN(S)
      END

      RECURSIVE SUBROUTINE SPILL(I, J)
      INTEGER I, J
      IF (J .GT. 2) CALL SPILL(I, J - 1)
      WRITE (6, 10) J
   10 FORMAT (1X, I<MOD(((I+1)*(J+2) + (I+3)*(J+4))
     1  * ((I+5)*(J+6) + (I+7)*(J+8))
     2  - ((I-1)*(J-2) + (I-3)*(J-4))
     3  * ((I-5)*(J-6) + (I-7)*(J-8)), 7) + 2>)
      END

      RECURSIVE SUBROUTINE LOOPER(N)
      INTEGER N, I, A, B, C, D, E, F, G, H
      A = 0
      B = 1
      C = 2
      D = 3
      E = 4
      F = 5
      G = 6
      H = 7
      DO 20 I = 1, N
         A = A + 1
         B = B + A
         C = C + B
         D = D + C
         E = E + D
         F = F + E
         G = G + F
         H = H + G
   20 CONTINUE
      WRITE (6, 10) A, B
   10 FORMAT (1X, I<N>, I<N + 1>)
      WRITE (6, 30) C, D, E, F, G, H
   30 FORMAT (1X, 6I5)
      END

C     SHARED is entered again by an AST that POKE's $DCLAST lets in.
      RECURSIVE SUBROUTINE SHARED(N, S)
      INTEGER N, W, POKE
      CHARACTER*(*) S
      W = N + 1
      WRITE (S, 10) 7, 7, 7
   10 FORMAT (I<W>, I<POKE(N)>, I<W>)
      END

C     AUTOS the same, its W AUTOMATIC, its compiler's cells static.
      SUBROUTINE AUTOS(N, S)
      AUTOMATIC W
      INTEGER N, W, POKE
      CHARACTER*(*) S
      W = N + 1
      WRITE (S, 10) 7, 7, 7
   10 FORMAT (I<W>, I<POKE(N)>, I<W>)
      END

      INTEGER FUNCTION POKE(N)
      INTEGER N, ST, SYS$DCLAST
      EXTERNAL AGAIN
      POKE = 2
      IF (N .EQ. 5) ST = SYS$DCLAST(AGAIN,,)
      END

      SUBROUTINE AGAIN
      CHARACTER*14 TR, TA
      COMMON /FROMAST/ TR, TA, NEXT
      IF (NEXT .EQ. 1) THEN
         CALL SHARED(1, TR)
      ELSE
         CALL AUTOS(1, TA)
      END IF
      END
