      PROGRAM OPTLP
C     What a loop's form must keep (Phase 84, subtask 10): a DO closed on its
C     limit and not a count, a test moved to the loop's bottom, and an index
C     stepped in a register and not multiplied. Each value comes through a
C     CALL, so that no earlier pass can settle it.
      EXTERNAL NEXT
      INTEGER I, J, K, L, N, T, NEXT, IV, S, M2(5,5), D(3,4)
      INTEGER E1, E2, I2, J2, NC
      INTEGER*2 IS
      REAL X
      CHARACTER*3 NM(3)
      CHARACTER*3 LINE
      EQUIVALENCE (E1, E2), (I2, J2)
      COMMON /LIM/ NC
      DATA NM /'ABC', 'DEF', 'GHI'/
  900 FORMAT (1X, A8, 3I12)
C     ------ a DO loop's limit ------
C     The limit is read once: changing N doesn't change the trips.
      CALL SET(N, 5)
      T = 0
      DO 10 I = 1, N
         N = N - 1
         T = T + 1
   10 CONTINUE
      PRINT 900, 'CHANGED', I, N, T
C     No trips, either way.
      CALL SET(N, 0)
      T = 0
      DO 11 I = 1, N
         T = T + 1
   11 CONTINUE
      PRINT 900, 'NONE UP', I, T
      CALL SET(N, 3)
      DO 12 I = 1, N, -1
         T = T + 1
   12 CONTINUE
      PRINT 900, 'NONE DN', I, T
C     Steps that land on the limit and that don't.
      CALL SET(N, 10)
      T = 0
      DO 13 I = 1, N, 3
         T = T + I
   13 CONTINUE
      PRINT 900, 'BY 3 ON', I, T
      CALL SET(N, 9)
      T = 0
      DO 14 I = 1, N, 3
         T = T + I
   14 CONTINUE
      PRINT 900, 'BY 3 OFF', I, T
C     Down to 1, to 0, to a variable, and by 2.
      CALL SET(N, 4)
      T = 0
      DO 15 I = N, 1, -1
         T = T + I
   15 CONTINUE
      PRINT 900, 'DOWN 1', I, T
      T = 0
      DO 16 I = N, 0, -1
         T = T + 1
   16 CONTINUE
      PRINT 900, 'DOWN 0', I, T
      CALL SET(L, 2)
      T = 0
      DO 17 I = N, L, -1
         T = T + 1
   17 CONTINUE
      PRINT 900, 'DOWN L', I, T
      T = 0
      DO 18 I = N, 1, -2
         T = T + 1
   18 CONTINUE
      PRINT 900, 'DOWN 2', I, T
C     A limit a step can't go past: these must keep their counts.
      T = 0
      DO 19 I = 2147483645, 2147483647
         T = T + 1
   19 CONTINUE
      PRINT 900, 'LARGEST', I, T
      T = 0
      DO 20 I = -2147483646, -2147483647-1, -1
         T = T + 1
   20 CONTINUE
      PRINT 900, 'SMALLEST', I, T
C     Changing the variable is against the rules (section 5.3.1), but what the
C     count did must go on being done where it can be seen: a call, and
C     another name.
      T = 0
      DO 21 I = 1, 3
         CALL BUMP(I, 10)
         T = T + 1
   21 CONTINUE
      PRINT 900, 'BY CALL', I, T
      T = 0
      DO 22 I2 = 1, 3
         J2 = J2 + 10
         T = T + 1
   22 CONTINUE
      PRINT 900, 'BY NAME', I2, T
C     A limit in COMMON that a call changes.
      CALL SET(NC, 3)
      T = 0
      DO 23 I = 1, NC
         CALL MORE
         T = T + 1
   23 CONTINUE
      PRINT 900, 'COMMON', I, NC, T
C     A limit that reads the variable, before it's set.
      CALL SET(I, 2)
      T = 0
      DO 24 I = 1, I + 2
         T = T + 1
   24 CONTINUE
      PRINT 900, 'OWN', I, T
C     Out by GO TO, the variable keeping its value.
      CALL SET(N, 5)
      T = 0
      DO 25 I = 1, N
         IF (I .EQ. 3) GO TO 26
         T = T + 1
   25 CONTINUE
   26 PRINT 900, 'OUT', I, T
C     The extended range: out of the loop and back.
      CALL SET(N, 3)
      T = 0
      DO 27 I = 1, N
         GO TO 28
   29    T = T + I
   27 CONTINUE
      GO TO 30
   28 T = T + 10
      GO TO 29
   30 PRINT 900, 'EXTENDED', I, T
C     A dummy as the variable and the limit.
      CALL SET(N, 5)
      CALL DUMDO(IV, N, S)
      PRINT 900, 'DUMMY', IV, S
C     Two loops sharing a terminal statement.
      CALL SET(N, 2)
      CALL SET(L, 3)
      T = 0
      DO 31 I = 1, N
      DO 31 J = 1, L
         T = T + 1
   31 CONTINUE
      PRINT 900, 'SHARED', I, J, T
C     An implied-DO list, and variables that keep their counts.
      DO 32 I = 1, 3
         D(1,I) = I*7
   32 CONTINUE
      PRINT 901, (D(1,I), I = 1, L)
  901 FORMAT (1X, 'IMPLIED ', 3I12)
      PRINT 900, 'AFTER', I
      T = 0
      DO 33 X = 1.0, 2.0, 0.5
         T = T + 1
   33 CONTINUE
      PRINT 902, 'REAL', X, T
  902 FORMAT (1X, A8, F12.2, I12)
      T = 0
      DO 34 IS = 1, L
         T = T + 1
   34 CONTINUE
      PRINT 900, 'WORD', IS, T
C     ------ a test at the bottom ------
      CALL SET(K, 1)
      CALL SET(N, 6)
      T = 0
   40 IF (K .GT. N) GO TO 41
      T = T + K
      K = K + 2
      GO TO 40
   41 PRINT 900, 'GO TO', K, T
C     No trips.
      CALL SET(K, 9)
      T = 0
   42 IF (K .GT. N) GO TO 43
      T = T + K
      K = K + 2
      GO TO 42
   43 PRINT 900, 'NOT ONCE', K, T
C     DO WHILE, and DO WHILE calling a function in its test.
      CALL SET(K, 1)
      CALL SET(N, 20)
      T = 0
      DO WHILE (K .LT. N)
         K = K*2
         T = T + 1
      END DO
      PRINT 900, 'WHILE', K, T
      CALL SET(K, 0)
      T = 0
      DO WHILE (NEXT(K) .LT. 4)
         T = T + 1
      END DO
      PRINT 900, 'WHILE FN', K, T
C     Statements before the test.
      CALL SET(K, 0)
      T = 0
   44 K = K + 1
      IF (K .GE. 5) GO TO 45
      T = T + K
      GO TO 44
   45 PRINT 900, 'STMTS', K, T
C     Two ways back to the top.
      CALL SET(K, 0)
      T = 0
   46 IF (K .GT. 10) GO TO 47
      K = K + 1
      IF (MOD(K,2) .EQ. 0) GO TO 46
      T = T + 1
      GO TO 46
   47 PRINT 900, 'TWO BACK', K, T
C     Two ways to the test from outside.
      CALL SET(L, 1)
      CALL SET(K, 0)
      IF (L .GT. 0) GO TO 48
      K = 100
   48 IF (K .GT. 2) GO TO 49
      K = K + 1
      GO TO 48
   49 PRINT 900, 'TWO IN', K
      CALL SET(L, 0)
      CALL SET(K, 0)
      IF (L .GT. 0) GO TO 50
      K = 100
   50 IF (K .GT. 2) GO TO 51
      K = K + 1
      GO TO 50
   51 PRINT 900, 'TWO IN', K
C     A loop that starts its routine.
      CALL SET(K, 0)
      CALL UPTO(K)
      PRINT 900, 'STARTS', K
C     ------ an index stepped ------
      CALL SET(N, 5)
      DO 61 J = 1, N
         DO 60 I = 1, N
            M2(I,J) = I*10 + J
   60    CONTINUE
   61 CONTINUE
C     The diagonal.
      T = 0
      DO 62 I = 1, N
         T = T + M2(I,I)
   62 CONTINUE
      PRINT 900, 'DIAGONAL', I, T
C     Along a row.
      CALL SET(K, 2)
      T = 0
      DO 63 J = 1, N
         T = T + M2(K,J)
   63 CONTINUE
      PRINT 900, 'ROW', J, T
C     In place, along a row.
      DO 65 J = 1, 4
         DO 64 I = 1, 3
            D(I,J) = 0
   64    CONTINUE
   65 CONTINUE
      CALL SET(L, 4)
      DO 66 J = 1, L
         D(2,J) = D(2,J) + J
   66 CONTINUE
      PRINT 900, 'IN PLACE', D(1,1), D(2,1), D(2,4)
      PRINT 900, 'IN PLACE', D(3,3), D(2,2), D(3,4)
C     Stepping down, and by 2.
      T = 0
      DO 67 J = N, 1, -1
         T = T + M2(1,J)*J
   67 CONTINUE
      PRINT 900, 'DOWN', J, T
      T = 0
      DO 68 J = 1, N, 2
         T = T + M2(2,J)
   68 CONTINUE
      PRINT 900, 'BY 2', J, T
C     An index going down as the loop goes up.
      T = 0
      DO 69 J = 1, N
         T = T + M2(2,N+1-J)*J
   69 CONTINUE
      PRINT 900, 'BACKWARD', J, T
C     Stepped only some of the time.
      CALL SET(K, 1)
      CALL SET(J, 1)
      T = 0
   70 IF (J .GT. N) GO TO 71
      T = T + M2(2,K)
      IF (MOD(J,2) .EQ. 0) K = K + 1
      J = J + 1
      GO TO 70
   71 PRINT 900, 'SOMETIME', J, K, T
C     Stepped twice a trip, and read between.
      CALL SET(K, 1)
      T = 0
      DO 72 I = 1, 2
         T = T + M2(1,K)
         K = K + 1
         T = T + M2(1,K)*100
         K = K + 1
   72 CONTINUE
      PRINT 900, 'TWICE', K, T
C     Two names for one variable, both stepped: neither can be followed.
      CALL SET(E1, 1)
      T = 0
   73 IF (E1 .GT. 4) GO TO 74
      E1 = E1 + 1
      T = T + M2(1,E2)
      E2 = E2 + 1
      GO TO 73
   74 PRINT 900, 'OVERLAP', E1, T
C     Past a call, which must leave the register alone.
      CALL SET(L, 3)
      T = 0
      DO 75 J = 1, L
         CALL BUSY
         T = T + M2(1,J)
   75 CONTINUE
      PRINT 900, 'CALL', J, T
C     A variable a call may change can't be followed.
      CALL SET(K, 1)
      T = 0
   76 IF (K .GT. 5) GO TO 77
      T = T + M2(1,K)
      CALL BUMP(K, 1)
      K = K + 1
      GO TO 76
   77 PRINT 900, 'CHANGED', K, T
C     A CHARACTER array, whose stride no index mode scales by.
      LINE = ' '
      DO 78 I = 1, 3
         LINE(I:I) = NM(I)(2:2)
   78 CONTINUE
      PRINT 903, 'TEXT', LINE
  903 FORMAT (1X, A8, A3)
C     An adjustable array, up and down.
      CALL SET(L, 5)
      CALL ADJ(M2, L, N, S, T)
      PRINT 900, 'ADJUST', S, T
C     Out of the loop in mid stride.
      T = 0
      DO 79 I = 1, N
         IF (M2(I,I) .GT. 30) GO TO 80
         T = T + 1
   79 CONTINUE
   80 PRINT 900, 'MID', I, T
C     Not as an index.
      T = 0
      S = 0
      CALL SET(L, 4)
      DO 81 I = 1, L
         T = T + I*7
         S = S + (I*3 - 1)
   81 CONTINUE
      PRINT 900, 'VALUES', I, T, S
C     The variable on both sides of a difference.
      T = 0
      DO 82 I = 1, L
         T = T + (I*3 - I)
   82 CONTINUE
      PRINT 900, 'BOTH', I, T
      END

      SUBROUTINE SET(N, V)
      INTEGER N, V
      N = V
      END

      SUBROUTINE BUMP(I, K)
      INTEGER I, K
      I = I + K
      END

      SUBROUTINE MORE
      COMMON /LIM/ NC
      NC = NC + 10
      END

      SUBROUTINE DUMDO(I, N, S)
      INTEGER I, N, S
      S = 0
      DO 10 I = 1, N
         S = S + I
   10 CONTINUE
      END

      INTEGER FUNCTION NEXT(K)
      INTEGER K
      K = K + 1
      NEXT = K
      END

      SUBROUTINE UPTO(K)
      INTEGER K
   10 IF (K .GE. 3) RETURN
      K = K + 1
      GO TO 10
      END

      SUBROUTINE BUSY
C     Uses every register the caller might have a value in.
      INTEGER A, B, C, D, E, F, G, H, I, J, N
      N = 3
      CALL SET(N, 3)
      A = 0
      DO 10 B = 1, N
         DO 10 C = 1, N
            DO 10 D = 1, N
               DO 10 E = 1, N
                  DO 10 F = 1, N
                     G = B*C
                     H = D*E
                     I = F + G
                     J = H + I
                     A = A + J
   10 CONTINUE
      END

      SUBROUTINE ADJ(A, M, N, S, T)
      INTEGER M, N, A(M,N), S, T, J
      S = 0
      DO 10 J = 1, N
         S = S + A(2,J)
   10 CONTINUE
      T = 0
      DO 20 J = N, 1, -1
         T = T + A(1,J)*J
   20 CONTINUE
      END
