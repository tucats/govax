C     Chapters 5 and 6 together (Phase 79, subtask 11): an RPN calculator whose
C     stack is one subroutine's entry points, reporting its errors by alternate
C     return and scanning its input with a computed GO TO; Simpson's rule through
C     a dummy procedure, given a FORTRAN function, an intrinsic, and an entry
C     point; a root found by bisection, its function passed on and called from a
C     statement function, with an alternate return if there is no root; a matrix
C     product on adjustable arrays; an assigned GO TO choosing what a loop does;
C     the manual's hyperbolic functions, ENTRY and statement functions together.
C     Answers that aren't exact are shown to three places, to check by hand.
      PROGRAM FLOW
      EXTERNAL SQUARE, TANH, SINH, COSH, CUBIC
      INTRINSIC SIN
      REAL SIMPSN, TANH, SINH, COSH, MILLI, X, R
      REAL A(2,3), B(3,2), C(2,2)
      INTEGER I, J, K, N, V, WAY
      CHARACTER*12 WORD
      CHARACTER*(*) TITLE
      PARAMETER (TITLE = 'FLOW')
      CHARACTER*8 BANNER
      MILLI(X) = NINT(X*1000.0) / 1000.0
      PRINT *, TITLE
C     The calculator
      CALL RPN('3 4 + 2 *', V, *10, *20)
      PRINT *, V
      CALL RPN('12 3 / 7 -', V, *10, *20)
      PRINT *, V
      GO TO 30
10    PRINT *, 'UNDERFLOW'
      GO TO 40
20    PRINT *, 'BAD CHARACTER'
      GO TO 50
30    CALL RPN('1 +', V, *10, *20)
      PRINT *, 'NOT REACHED'
40    CALL RPN('1 2 %', V, *10, *20)
      PRINT *, 'NOT REACHED'
C     Simpson's rule, of three kinds of routine
50    PRINT *, MILLI(SIMPSN(SQUARE, 0.0, 3.0, 10))
      PRINT *, MILLI(SIMPSN(SIN, 0.0, 3.14159265, 20))
      PRINT *, MILLI(SIMPSN(COSH, 0.0, 1.0, 10))
      PRINT *, MILLI(SIMPSN(TANH, -1.0, 1.0, 4))
C     A root, and none
      CALL BISECT(CUBIC, 0.0, 2.0, R, *60)
      PRINT *, MILLI(R)
      CALL BISECT(SQUARE, 1.0, 2.0, R, *60)
      PRINT *, 'NOT REACHED'
60    PRINT *, 'NO ROOT'
C     Adjustable arrays
      DO J = 1, 3
        DO I = 1, 2
          A(I,J) = I + 10*J
          B(J,I) = I - J
        END DO
      END DO
      CALL MATMUL(A, B, C, 2, 3, 2)
      PRINT *, C
C     An assigned GO TO in a loop, each pass choosing the next
      ASSIGN 110 TO WAY
      N = 0
      DO 130 K = 1, 6
        GO TO WAY, (110, 120)
110     N = N + K
        ASSIGN 120 TO WAY
        GO TO 130
120     N = N * 2
        ASSIGN 110 TO WAY
130   CONTINUE
      PRINT *, N
C     The manual's hyperbolic functions
      PRINT *, MILLI(TANH(1.0)), MILLI(SINH(1.0)), MILLI(COSH(1.0))
C     A CHARACTER function of the length this unit says
      WORD = BANNER('AB') // BANNER('CD')
      PRINT *, WORD
      PRINT *, LEN(BANNER('AB'))
      STOP 'DONE'
      END

C     S in reverse Polish, scanned a character at a time; its value is left in V.
C     RETURN 1 is too few operands, and RETURN 2 a character it doesn't know.
      SUBROUTINE RPN(S, V, *, *)
      CHARACTER*(*) S
      CHARACTER CH
      INTEGER V, CLASS, I, A, B, NUM, DIGIT
      LOGICAL INNUM
      CALL STACK(0)
      INNUM = .FALSE.
      DO 50 I = 1, LEN(S) + 1
        CH = ' '
        IF (I .LE. LEN(S)) CH = S(I:I)
        DIGIT = INDEX('0123456789', CH)
        CLASS = 4
        IF (CH .EQ. ' ') CLASS = 1
        IF (DIGIT .GT. 0) CLASS = 2
        IF (INDEX('+-*/', CH) .GT. 0) CLASS = 3
        GO TO (10, 20, 30, 40), CLASS
10      IF (INNUM) CALL PUSH(NUM)
        INNUM = .FALSE.
        GO TO 50
20      IF (.NOT. INNUM) NUM = 0
        INNUM = .TRUE.
        NUM = NUM*10 + DIGIT - 1
        GO TO 50
30      CALL POP(B, *90)
        CALL POP(A, *90)
        GO TO (31, 32, 33, 34), INDEX('+-*/', CH)
31      CALL PUSH(A + B)
        GO TO 50
32      CALL PUSH(A - B)
        GO TO 50
33      CALL PUSH(A * B)
        GO TO 50
34      CALL PUSH(A / B)
        GO TO 50
40      RETURN 2
50    CONTINUE
      CALL POP(V, *90)
      RETURN
90    RETURN 1
      END

C     A stack: STACK empties it, and PUSH and POP use it.
      SUBROUTINE STACK(V)
      INTEGER V, ITEMS(20), TOP
      SAVE ITEMS, TOP
      TOP = 0
      RETURN
      ENTRY PUSH(V)
      TOP = TOP + 1
      ITEMS(TOP) = V
      RETURN
      ENTRY POP(V, *)
      IF (TOP .LE. 0) RETURN 1
      V = ITEMS(TOP)
      TOP = TOP - 1
      RETURN
      END

C     Simpson's rule for F from A to B, N (even) intervals.
      REAL FUNCTION SIMPSN(F, A, B, N)
      REAL A, B, F, H, S
      INTEGER N, I
      H = (B - A) / N
      S = F(A) + F(B)
      DO I = 1, N - 1
        IF (MOD(I, 2) .EQ. 1) THEN
          S = S + 4.0*F(A + I*H)
        ELSE
          S = S + 2.0*F(A + I*H)
        END IF
      END DO
      SIMPSN = S*H/3.0
      END

C     A root of F between LO and HI, within a part in ten thousand, or RETURN 1 if
C     F has the same sign at both.
      SUBROUTINE BISECT(F, LO, HI, R, *)
      EXTERNAL F
      REAL F, LO, HI, R, A, B, FA, M, EVAL, PROBE, X
      EVAL(X) = PROBE(F, X)
      A = LO
      B = HI
      FA = EVAL(A)
      IF (FA*EVAL(B) .GT. 0.0) RETURN 1
      DO WHILE (B - A .GT. 0.0001)
        M = (A + B) / 2.0
        IF (FA*EVAL(M)) 10, 20, 30
10      B = M
        GO TO 40
20      A = M
        B = M
        GO TO 40
30      A = M
40    END DO
      R = (A + B) / 2.0
      END

      REAL FUNCTION PROBE(G, X)
      REAL G, X
      PROBE = G(X)
      END

      REAL FUNCTION SQUARE(X)
      SQUARE = X*X
      END

      REAL FUNCTION CUBIC(X)
      CUBIC = X*X*X - 2.0
      END

C     C = A*B, their sizes given.
      SUBROUTINE MATMUL(A, B, C, L, M, N)
      INTEGER L, M, N, I, J, K
      REAL A(L, M), B(M, N), C(L, N)
      DO 10 J = 1, N
        DO 10 I = 1, L
          C(I, J) = 0.0
          DO 10 K = 1, M
            C(I, J) = C(I, J) + A(I, K)*B(K, J)
10    CONTINUE
      END

C     The language manual's example, section 6.2.4.1.
      REAL FUNCTION TANH(X)
C     Statement function to compute twice sinh
      TSINH(Y) = EXP(Y) - EXP(-Y)
C     Statement function to compute twice cosh
      TCOSH(Y) = EXP(Y) + EXP(-Y)
C     Compute tanh
      TANH = TSINH(X)/TCOSH(X)
      RETURN
C     Compute sinh
      ENTRY SINH(X)
      SINH = TSINH(X)/2.0
      RETURN
C     Compute cosh
      ENTRY COSH(X)
      COSH = TCOSH(X)/2.0
      RETURN
      END

      CHARACTER*(*) FUNCTION BANNER(S)
      CHARACTER*(*) S
      BANNER = '*' // S // '*'
      END
