      PROGRAM PARAMS
C     PARAMETER constants, both forms (section 4.11 and appendix A.4),
C     in declarations and in expressions, and constant expressions
C     computed when compiling: ** and intrinsic functions of constants.
      INTEGER N, M
      REAL PI, PIOV2
      CHARACTER*(*) TITLE
      PARAMETER (N = 5, M = N * 2 + 1)
      PARAMETER (PI = 3.1415927, PIOV2 = PI / 2)
      PARAMETER (TITLE = 'CONSTANTS')
      PARAMETER LIMIT = 2 ** 10, FLAG = .TRUE.
      INTEGER SQ(N), K(-N:N)
      PRINT *, TITLE // ':'
      PRINT *, N, M, LIMIT, MAX(N, M, 7), MOD(-M, 4)
      PRINT *, PI, PIOV2, 2.0 ** (-2)
      DO 10 I = 1, N
         SQ(I) = I * I
   10 CONTINUE
      PRINT *, SQ
      DO 20 I = -N, N
   20 K(I) = I * M
      PRINT *, K(-N), K(0), K(N)
      PRINT *, FLAG, ICHAR('A'), 'AB' .LT. 'B'
      END
