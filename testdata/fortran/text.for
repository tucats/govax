C     Character handling end to end (Phase 78, subtask 10): a line split
C     into words by INDEX, the words sorted by LLT and counted, a word
C     reversed and tested as a palindrome, letters changed to lowercase
C     and enciphered through ICHAR and CHAR, the letters counted, a number
C     written as its digits, a title centered by LEN, and a sentence
C     built by concatenation.
      PROGRAM TEXT
      CHARACTER*48 LINE
      CHARACTER*10 WORDS(10), W
      CHARACTER*30 TITLE
      CHARACTER*12 DIGITS
      INTEGER N, I, J, COUNT(26), K
      LOGICAL PALIN
      LINE = 'THE QUICK BROWN FOX JUMPS OVER THE LAZY DOG'
C     The words, split at blanks.
      CALL SPLIT(LINE, WORDS, N)
      PRINT *, N, ' WORDS'
C     Sorted, by insertion.
      DO 20 I = 2, N
         W = WORDS(I)
         J = I - 1
   10    IF (J .GE. 1) THEN
            IF (LLT(W, WORDS(J))) THEN
               WORDS(J+1) = WORDS(J)
               J = J - 1
               GO TO 10
            END IF
         END IF
         WORDS(J+1) = W
   20 CONTINUE
      PRINT *, WORDS(1)(1:5), WORDS(2)(1:5), WORDS(3)(1:5),
     1         WORDS(4)(1:5), WORDS(5)(1:5)
      PRINT *, WORDS(6)(1:5), WORDS(7)(1:5), WORDS(8)(1:5),
     1         WORDS(9)(1:5)
C     Reversed, and palindromes.
      W = 'LEVEL'
      CALL REVERS(W(1:5))
      PRINT *, W, '|', PALIN('LEVEL'), PALIN('RADAR '), PALIN('DOG')
      PRINT *, PALIN('A'), PALIN('ABBA'), PALIN('ABCA')
C     Lowercase, and a Caesar cipher of three.
      W = 'FOX JUMPS'
      CALL LOWER(W)
      PRINT *, W, '|'
      W = 'XYZ ABC'
      CALL SHIFT(W, 3)
      PRINT *, W, '|'
      CALL SHIFT(W, -3)
      PRINT *, W, '|'
C     The letters counted.
      DO 30 I = 1, 26
         COUNT(I) = 0
   30 CONTINUE
      DO 40 I = 1, LEN(LINE)
         K = ICHAR(LINE(I:I)) - ICHAR('A') + 1
         IF (K .GE. 1 .AND. K .LE. 26) COUNT(K) = COUNT(K) + 1
   40 CONTINUE
      K = 0
      DO 50 I = 1, 26
         IF (COUNT(I) .EQ. 0) PRINT *, 'NO ', CHAR(I + 64)
         IF (COUNT(I) .GT. COUNT(K+1)) K = I - 1
   50 CONTINUE
      PRINT *, 'MOST: ', CHAR(K + 65), COUNT(K+1)
C     A number as its digits.
      CALL ITOA(-30215, DIGITS)
      PRINT *, '[', DIGITS, ']'
      CALL ITOA(0, DIGITS)
      PRINT *, '[', DIGITS, ']'
C     A title centered.
      CALL CENTER('CHAPTER ONE', TITLE)
      PRINT *, '[', TITLE, ']'
C     A sentence built of parts.
      I = INDEX(LINE, 'FOX')
      J = INDEX(LINE, 'DOG')
      TITLE = LINE(J:J+2) // LINE(I-1:J-1) // LINE(I:I+2)
      PRINT *, TITLE, '|'
      END
C     SPLIT puts the words of LINE in WORDS, and their number in N.
      SUBROUTINE SPLIT(LINE, WORDS, N)
      CHARACTER*(*) LINE, WORDS(*)
      INTEGER N, I, J
      N = 0
      I = 1
   10 IF (I .GT. LEN(LINE)) RETURN
      IF (LINE(I:I) .EQ. ' ') THEN
         I = I + 1
         GO TO 10
      END IF
      J = INDEX(LINE(I:), ' ')
      IF (J .EQ. 0) J = LEN(LINE) - I + 2
      N = N + 1
      WORDS(N) = LINE(I:I+J-2)
      I = I + J
      GO TO 10
      END
C     REVERS reverses S in place.
      SUBROUTINE REVERS(S)
      CHARACTER*(*) S
      CHARACTER*1 T
      N = LEN(S)
      DO 10 I = 1, N/2
         T = S(I:I)
         S(I:I) = S(N+1-I:N+1-I)
         S(N+1-I:N+1-I) = T
   10 CONTINUE
      END
C     PALIN says whether S, but for trailing blanks, reads the same
C     backward.
      LOGICAL FUNCTION PALIN(S)
      CHARACTER*(*) S
      N = LEN(S)
   10 IF (N .GT. 1 .AND. S(N:N) .EQ. ' ') THEN
         N = N - 1
         GO TO 10
      END IF
      PALIN = .TRUE.
      DO 20 I = 1, N/2
         IF (S(I:I) .NE. S(N+1-I:N+1-I)) PALIN = .FALSE.
   20 CONTINUE
      END
C     LOWER makes S's letters lowercase.
      SUBROUTINE LOWER(S)
      CHARACTER*(*) S
      DO 10 I = 1, LEN(S)
         IF (LGE(S(I:I), 'A') .AND. LLE(S(I:I), 'Z'))
     1      S(I:I) = CHAR(ICHAR(S(I:I)) + 32)
   10 CONTINUE
      END
C     SHIFT moves each of S's letters K places along the alphabet,
C     around from Z to A.
      SUBROUTINE SHIFT(S, K)
      CHARACTER*(*) S
      INTEGER K, C
      DO 10 I = 1, LEN(S)
         C = ICHAR(S(I:I)) - ICHAR('A')
         IF (C .GE. 0 .AND. C .LT. 26)
     1      S(I:I) = CHAR(MOD(C + K + 26, 26) + ICHAR('A'))
   10 CONTINUE
      END
C     ITOA writes N's digits, with a sign if it's negative, at the left
C     of S, filled out with blanks.
      SUBROUTINE ITOA(N, S)
      CHARACTER*(*) S
      CHARACTER*11 T
      INTEGER N, M, K
      M = ABS(N)
      K = LEN(T) + 1
   10 K = K - 1
      T(K:K) = CHAR(MOD(M, 10) + ICHAR('0'))
      M = M/10
      IF (M .NE. 0) GO TO 10
      IF (N .LT. 0) THEN
         K = K - 1
         T(K:K) = '-'
      END IF
      S = T(K:)
      END
C     CENTER puts STR in the middle of S.
      SUBROUTINE CENTER(STR, S)
      CHARACTER*(*) STR, S
      K = (LEN(S) - LEN(STR))/2
      S = ' '
      S(K+1:K+LEN(STR)) = STR
      END
