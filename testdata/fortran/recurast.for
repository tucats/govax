C Recursive routines entered again by an AST while they're active (Phase 89,
C subtask 11), each activation's frame left as it was. F is in the middle of an
C expression, a call's value held in a temporary, when POKE declares an AST that
C calls F again with other values. NEST holds a CHARACTER function's value from
C its own call when PING does the same. SPIN's deepest activation waits in a loop
C with no call in it for a timer's AST, which runs SPIN six deep below it; every
C activation then checks its array, its internal file's text, and its DO loop.
      PROGRAM RECAST
      INTEGER F, OUTER, INNER, NAST, NDONE, TOTAL, ATOTAL, NBAD, SPINS
      CHARACTER*16 NEST, OUTERS, INNERS
      LOGICAL INAST
      COMMON /AST/ INNER, NAST, NDONE, TOTAL, ATOTAL, NBAD, SPINS, INAST
      COMMON /ASTC/ INNERS
      VOLATILE /AST/, /ASTC/

      NAST = 0
      OUTER = F(3, 100)
      WRITE (6, 100) OUTER, INNER, NAST
  100 FORMAT (' F:', 2I6, '   ASTS', I2)

      NAST = 0
      OUTERS = NEST(4, '*')
      WRITE (6, 110) OUTERS, INNERS, NAST
  110 FORMAT (' NEST: ', A, 1X, A, '   ASTS', I2)

      NDONE = 0
      NBAD = 0
      SPINS = 0
      TOTAL = 0
      INAST = .FALSE.
      CALL SPIN(1, 4, TOTAL)
      WRITE (6, 120) TOTAL, ATOTAL, NBAD, NDONE, SPINS .GT. 0
  120 FORMAT (' SPIN:', 2I6, '   BAD', I2, '   DONE', I2,
     1   '   WAITED', L2)
      END

C N*K for each level, and POKE's N, added on the way back up.
      RECURSIVE INTEGER FUNCTION F(N, K) RESULT(R)
      INTEGER N, K, POKE
      IF (N .EQ. 0) THEN
         R = 0
      ELSE
         R = N * K + F(N - 1, K) + POKE(N)
      END IF
      END

C At level 2, the first time, an AST: it comes when SYS$DCLAST returns.
      RECURSIVE INTEGER FUNCTION POKE(N) RESULT(P)
      EXTERNAL AGAIN
      INTEGER N, I, SYS$DCLAST
      COMMON /AST/ INNER, NAST, NDONE, TOTAL, ATOTAL, NBAD, SPINS, INAST
      VOLATILE /AST/
      IF (N .EQ. 2 .AND. NAST .EQ. 0) THEN
         NAST = 1
         I = SYS$DCLAST(AGAIN,,)
      END IF
      P = N
      END

      SUBROUTINE AGAIN
      INTEGER F
      COMMON /AST/ INNER, NAST, NDONE, TOTAL, ATOTAL, NBAD, SPINS, INAST
      VOLATILE /AST/
      INNER = F(4, 7)
      END

C LEAF in N pairs of parentheses. T, the value of the call below, is used after
C PING, which may have let an AST in.
      RECURSIVE CHARACTER*16 FUNCTION NEST(N, LEAF) RESULT(S)
      INTEGER N
      CHARACTER*1 LEAF
      CHARACTER*16 T
      IF (N .EQ. 0) THEN
         S = LEAF
      ELSE
         T = NEST(N - 1, LEAF)
         CALL PING(N)
         S = '(' // T(1:2*N-1) // ')'
      END IF
      END

      RECURSIVE SUBROUTINE PING(N)
      EXTERNAL AGAIN2
      INTEGER N, I, SYS$DCLAST
      COMMON /AST/ INNER, NAST, NDONE, TOTAL, ATOTAL, NBAD, SPINS, INAST
      VOLATILE /AST/
      IF (N .EQ. 2 .AND. NAST .EQ. 0) THEN
         NAST = 1
         I = SYS$DCLAST(AGAIN2,,)
      END IF
      END

      SUBROUTINE AGAIN2
      CHARACTER*16 NEST, INNERS
      COMMON /ASTC/ INNERS
      VOLATILE /ASTC/
      INNERS = NEST(3, '#')
      END

C LIMIT levels, each with an array and a text of its own. Outside the AST the
C last waits for the timer's.
      RECURSIVE SUBROUTINE SPIN(N, LIMIT, SUM)
      EXTERNAL TICK
      INTEGER N, LIMIT, SUM, I, A(8), DELTA(2), ST, SYS$SETIMR
      CHARACTER*8 TAG, WANT
      COMMON /AST/ INNER, NAST, NDONE, TOTAL, ATOTAL, NBAD, SPINS, INAST
      VOLATILE /AST/
      DATA DELTA /-100000, -1/
      DO 10 I = 1, 8
         A(I) = N * 10 + I
   10 CONTINUE
      WRITE (TAG, '(A, I3)') 'LEVEL', N
      IF (N .LT. LIMIT) THEN
         CALL SPIN(N + 1, LIMIT, SUM)
      ELSE IF (.NOT. INAST) THEN
         ST = SYS$SETIMR(, DELTA, TICK,)
         DO WHILE (NDONE .EQ. 0)
            SPINS = SPINS + 1
         END DO
      END IF
      DO 20 I = 1, 8
         IF (A(I) .NE. N * 10 + I) NBAD = NBAD + 1
         SUM = SUM + A(I)
   20 CONTINUE
      WRITE (WANT, '(A, I3)') 'LEVEL', N
      IF (TAG .NE. WANT) NBAD = NBAD + 1
      END

      SUBROUTINE TICK
      COMMON /AST/ INNER, NAST, NDONE, TOTAL, ATOTAL, NBAD, SPINS, INAST
      VOLATILE /AST/
      INAST = .TRUE.
      ATOTAL = 0
      CALL SPIN(1, 6, ATOTAL)
      NDONE = 1
      END
