C     What making a string of one character a byte must get right (Phase 84):
C     moves, what's cut and what's filled, the order of bytes above 127, NUL,
C     constants, every kind of place, what a store or a call changes, and values of
C     two lengths, which stay strings.
      PROGRAM OPTCHAR
      CHARACTER C, E, F, S*5, T*5, ROW(3), W*4, COM1, LINE*12, OUT*12
      CHARACTER SF, X, UP
      INTEGER I, N, IV
      LOGICAL L1, L2, L3, L4, L5, L6
      VOLATILE V
      CHARACTER V
      EQUIVALENCE (IV, W)
      COMMON /SHARED/ COM1
      STRUCTURE /ONE/
         BYTE B
      END STRUCTURE
      RECORD /ONE/ R1, R2
      SF(X) = X
C
  100 FORMAT (1X,A)
  110 FORMAT (1X,A,1X,I6)
  120 FORMAT (1X,A,1X,6L2)
  130 FORMAT (1X,A,1X,'[',A,']')
C
C     Moves: a source cut to one character; a destination longer than its source,
C     which is filled.
      C = 'X'
      PRINT 130, 'CONSTANT', C
      C = 'XYZ'
      PRINT 130, 'CUT CONSTANT', C
      CALL SETS(S, 'ABCDE')
      C = S
      PRINT 130, 'CUT VARIABLE', C
      C = S(3:5)
      PRINT 130, 'CUT SUBSTRING', C
      S(2:2) = 'Q'
      PRINT 130, 'ONE OF FIVE', S
      S(4:4) = S(1:1)
      PRINT 130, 'INSIDE ITSELF', S
      T = C
      PRINT 130, 'FILLED', T
C
C     The order is unsigned: CHAR(200) comes after every letter, unknown to the
C     compiler and then known to it.
      CALL SETC(C, CHAR(200))
      L1 = C .GT. 'A'
      L2 = C .GE. 'A'
      L3 = C .LT. 'A'
      L4 = C .LE. 'A'
      L5 = C .EQ. 'A'
      L6 = C .NE. 'A'
      PRINT 120, 'HIGH, UNKNOWN', L1, L2, L3, L4, L5, L6
      L1 = 'A' .LT. C
      L2 = 'A' .LE. C
      L3 = 'A' .GT. C
      L4 = 'A' .GE. C
      L5 = 'A' .EQ. C
      L6 = 'A' .NE. C
      PRINT 120, 'CONSTANT FIRST', L1, L2, L3, L4, L5, L6
      F = CHAR(200)
      L1 = F .GT. 'A'
      L2 = F .GE. 'A'
      L3 = F .LT. 'A'
      L4 = F .LE. 'A'
      L5 = F .EQ. 'A'
      L6 = F .NE. 'A'
      PRINT 120, 'HIGH, KNOWN', L1, L2, L3, L4, L5, L6
      N = 0
      CALL SETC(E, CHAR(201))
      IF (C .LT. E) N = N + 1
      IF (C .LE. E) N = N + 10
      IF (E .GT. C) N = N + 100
      IF (E .LT. C) N = N + 1000
      PRINT 110, 'TWO HIGH', N
C
C     NUL: a comparison with zero is a TST, and a byte of 200 is above it. The move
C     just before leaves C set from the comparison before that, so a TST left out
C     would see a borrow that isn't there.
      CALL SETC(F, CHAR(200))
      N = 0
      IF ('A' .LT. F) N = 1
      E = F
      IF (E .GT. CHAR(0)) N = N + 10
      PRINT 110, 'ABOVE NUL', N
      CALL SETC(C, CHAR(0))
      L1 = C .GT. CHAR(0)
      L2 = C .GE. CHAR(0)
      L3 = C .LT. CHAR(0)
      L4 = C .LE. CHAR(0)
      L5 = C .EQ. CHAR(0)
      L6 = C .NE. CHAR(0)
      PRINT 120, 'NUL', L1, L2, L3, L4, L5, L6
      L1 = CHAR(0) .LT. E
      L2 = CHAR(0) .GE. E
      PRINT 120, 'NUL FIRST', L1, L2
      C = CHAR(0)
      N = ICHAR(C)
      PRINT 110, 'NUL STORED', N
C
C     In a condition's .AND., .OR., and .NOT., and shared with a text that stays.
      PRINT 130, 'SHARED TEXT', 'M'
      N = 0
      DO 10 I = 1, 4
         CALL SETC(C, 'M')
         IF (I .EQ. 2) CALL SETC(C, 'a')
         IF (I .EQ. 3) CALL SETC(C, '@')
         IF (I .EQ. 4) CALL SETC(C, CHAR(200))
         IF (C .GE. 'A' .AND. C .LE. 'Z') N = N + 1
         IF (C .LT. 'A' .OR. C .GT. 'Z') N = N + 10
         IF (.NOT. C .EQ. 'M') N = N + 100
   10 CONTINUE
      PRINT 110, 'CONDITIONS', N
C
C     A value of another length is a string compared, filled with blanks.
      CALL SETC(C, 'A')
      L1 = C .EQ. 'A '
      L2 = C .EQ. 'AB'
      L3 = C .LT. 'AB'
      L4 = 'A' .EQ. C
      PRINT 120, 'TWO LENGTHS', L1, L2, L3, L4
C
C     A loop's places: a store in the loop changes what it reads, by the byte or as
C     a whole string.
      CALL SETS(S, 'ABCDE')
      N = 0
      DO 20 I = 1, 3
         IF (S(1:1) .EQ. 'A') N = N + 1
         S(1:1) = 'B'
   20 CONTINUE
      PRINT 110, 'STORE IN LOOP', N
      CALL SETS(S, 'ABCDE')
      CALL SETS(T, 'BBBBB')
      N = 0
      DO 30 I = 1, 3
         IF (S(1:1) .EQ. 'A') N = N + 1
         S = T
   30 CONTINUE
      PRINT 110, 'STRING IN LOOP', N
      CALL SETS(S, 'ABCDE')
      N = 0
      DO 40 I = 1, 3
         IF (S(1:1) .EQ. 'A') N = N + 1
         S(2:2) = 'Z'
   40 CONTINUE
      PRINT 110, 'OTHER IN LOOP', N
C
C     Sharing storage with an INTEGER, in both directions.
      IV = 65
      L1 = W(1:1) .EQ. 'A'
      W(1:1) = 'B'
      N = IV
      PRINT 120, 'OVERLAP', L1
      PRINT 110, 'OVERLAP', N
      W(2:2) = CHAR(1)
      N = IV
      PRINT 110, 'OVERLAP', N
      C = 'A'
      IV = 66
      W(1:1) = C
      N = IV
      PRINT 110, 'OVERLAP', N
C
C     COMMON changed by a call, and a known store that doesn't survive it.
      COM1 = 'P'
      CALL CHANGE
      L1 = COM1 .EQ. 'P'
      L2 = COM1 .EQ. 'Q'
      PRINT 120, 'COMMON', L1, L2
C
C     An array of characters, through a dummy, and a dummy string.
      ROW(1) = 'R'
      ROW(2) = 'S'
      ROW(3) = 'T'
      CALL REV(ROW, 3)
      PRINT 130, 'ADJUSTABLE', ROW(1)//ROW(2)//ROW(3)
      CALL SETS(LINE, ' A  B C   DE ')
      CALL SQUEEZE(LINE, OUT, N)
      PRINT 130, 'SQUEEZED', OUT(1:N)
      CALL LEFT(OUT, N)
      PRINT 130, 'SHIFTED', OUT(1:N)
C
C     Functions and a statement function.
      C = UP('b')
      PRINT 130, 'FUNCTION', C
      L1 = UP('c') .EQ. 'C'
      L2 = SF('d') .EQ. 'D'
      L3 = SF('e') .EQ. 'e'
      PRINT 120, 'FUNCTIONS', L1, L2, L3
C
C     VOLATILE stays where it is.
      V = 'X'
      V = 'Y'
      L1 = V .EQ. 'Y'
      PRINT 120, 'VOLATILE', L1
C
C     A record of one byte moves as the record it is.
      R1.B = 7
      R2 = R1
      N = R2.B
      PRINT 110, 'RECORD', N
      END
C
      SUBROUTINE SETC(TO, FROM)
      CHARACTER TO, FROM
      TO = FROM
      END
C
      SUBROUTINE SETS(TO, FROM)
      CHARACTER*(*) TO, FROM
      TO = FROM
      END
C
      SUBROUTINE SETI(TO, FROM)
      INTEGER TO, FROM
      TO = FROM
      END
C
      SUBROUTINE CHANGE
      CHARACTER COM1
      COMMON /SHARED/ COM1
      COM1 = 'Q'
      END
C
      SUBROUTINE REV(A, N)
      INTEGER N, I
      CHARACTER A(N), HOLD
      DO 10 I = 1, N/2
         HOLD = A(I)
         A(I) = A(N+1-I)
         A(N+1-I) = HOLD
   10 CONTINUE
      END
C
      SUBROUTINE SQUEEZE(FROM, TO, N)
      CHARACTER*(*) FROM, TO
      INTEGER N, I
      N = 0
      DO 10 I = 1, LEN(FROM)
         IF (FROM(I:I) .NE. ' ') THEN
            N = N + 1
            TO(N:N) = FROM(I:I)
         END IF
   10 CONTINUE
      END
C
      SUBROUTINE LEFT(S, N)
      CHARACTER*(*) S
      INTEGER N, I
      DO 10 I = 1, N-1
         S(I:I) = S(I+1:I+1)
   10 CONTINUE
      N = N - 1
      END
C
      CHARACTER FUNCTION UP(C)
      CHARACTER C
      UP = CHAR(ICHAR(C) - 32)
      END
