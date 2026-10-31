      PROGRAM CHARS
C     CHARACTER values: assignment filling with blanks and cutting,
C     substrings with constant bounds and with bounds that aren't,
C     overlapping moves, arrays of them, comparisons as values and as
C     conditions, the blank padding of the shorter, ASCII's order past
C     127, EQUIVALENCE, and a common block another unit changes.
      CHARACTER*8 S, T*3, U*12, WORD*5, LINE*20
      CHARACTER*4 NAMES(3), CODE
      CHARACTER*10 MSG
      INTEGER I, J, N, ICODE
      LOGICAL L
      EQUIVALENCE (CODE, ICODE)
      COMMON /TEXT/ MSG
C     Filled and cut.
      S = 'ABC'
      PRINT *, '[', S, ']'
      T = 'VWXYZ'
      PRINT *, '[', T, ']'
      U = T
      PRINT *, '[', U, ']'
C     Constant bounds.
      S = 'ABCDEFGH'
      PRINT *, S(2:4), S(:2), S(7:)
      S(3:5) = '123'
      PRINT *, S
      S(1:2) = 'Q'
      PRINT *, S
C     Bounds that aren't constant.
      WORD = 'HELLO'
      DO 10 I = 1, 5
         PRINT *, WORD(I:I), WORD(:I), ' ', WORD(I:)
   10 CONTINUE
      LINE = ' '
      DO 20 I = 1, 5
         LINE(I:I) = WORD(6-I:6-I)
   20 CONTINUE
      PRINT *, '[', LINE(1:5), ']'
      I = 2
      J = 5
      S = 'ABCDEFGH'
      LINE(I:J+I) = S(J:)
      PRINT *, '[', S(I:J), '] [', LINE(:J+I+1), ']'
C     Overlapping moves.
      S(2:) = S
      PRINT *, S
      S = 'ABCDEFGH'
      S(:7) = S(2:)
      PRINT *, S
C     Arrays.
      NAMES(1) = 'ANN'
      NAMES(2) = 'BOBBY'
      NAMES(3) = 'CY'
      PRINT *, NAMES, '|'
      DO 30 I = 1, 3
         PRINT *, I, ' ', NAMES(I)(1:1), NAMES(I)(2:), '|'
   30 CONTINUE
C     Comparisons.
      L = NAMES(1) .LT. NAMES(2)
      PRINT *, L, NAMES(3) .GT. NAMES(2), 'ANN' .EQ. NAMES(1),
     1   NAMES(1) .EQ. 'ANN ', NAMES(1) .NE. 'ANN', 'AB' .LT. S(1:2)
      L = NAMES(1) .GE. 'ANN' .AND. NAMES(2) .LE. 'BOB'
      PRINT *, L, 'AB' .LT. 'AB!', WORD(3:4) .LE. WORD(4:5)
      DO 40 I = 1, 3
         IF (NAMES(I) .LT. 'B') THEN
            PRINT *, NAMES(I), ' is before B'
         ELSE IF (NAMES(I) .GE. 'C' .AND. I .EQ. 3) THEN
            PRINT *, NAMES(I), ' is C or after'
         ELSE
            PRINT *, NAMES(I), ' is a B'
         END IF
   40 CONTINUE
      N = 0
      DO 50 I = 1, 5
         IF (WORD(I:I) .NE. 'L') N = N + 1
   50 CONTINUE
      PRINT *, 'Not L:', N
      S = CHAR(200)
      IF (S .GT. 'Z') PRINT *, 'Code 200 is after Z'
      IF (.NOT. S .LE. 'Z' .OR. S .EQ. 'Z') PRINT *, 'and not before'
C     A CHARACTER value's bytes, and a common block.
      CODE = 'ABCD'
      PRINT *, ICODE
      MSG = 'SHIFTED'
      CALL SHIFT
      PRINT *, '[', MSG, ']'
      END

      SUBROUTINE SHIFT
      CHARACTER*10 MSG
      COMMON /TEXT/ MSG
      MSG(2:) = MSG
      MSG(1:1) = '>'
      END
