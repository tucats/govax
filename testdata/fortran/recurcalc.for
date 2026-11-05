C A recursive descent evaluator (Phase 89, subtask 11): EXPR, TERM, and FACTOR
C are CHARACTER functions with RESULT, each returning the expression it read with
C every operation in parentheses, built by concatenation, and its value through
C its argument. A parenthesis calls EXPR again from FACTOR, so the three are
C active at many depths at once, each with its own result, operands, and
C operator in its frame. What each prints is read again, and must give itself
C and the same value. The scanner's place and the text are in COMMON.
      PROGRAM CALC
      CHARACTER*60 LINES(8), SRC, S, AGAIN, EXPR
      INTEGER POS, NLEN, IERR, V, W, I, LENTRM
      COMMON /SCAN/ POS, NLEN, IERR
      COMMON /TEXT/ SRC
      DATA LINES /'1+2*3', '(1+2)*3', '  10 - 4 - 3',
     1   '-7/2 + 2*-(3)', '((((((8))))))', '1-(2-(3-(4-(5-6))))',
     2   '2*(3+', '((2+3)*(4-6))/-(1+1)*100'/

      DO 10 I = 1, 8
         SRC = LINES(I)
         CALL START
         S = EXPR(V)
         IF (IERR .EQ. 0 .AND. POS .LE. NLEN) IERR = POS
         WRITE (6, 100) SRC(1:LENTRM(SRC)), S(1:LENTRM(S)), V
  100    FORMAT (1X, A, '  IS  ', A, '  =', I5)
         IF (IERR .NE. 0) THEN
            WRITE (6, 110) IERR
  110       FORMAT (' ERROR AT COLUMN', I3)
         ELSE
            SRC = S
            CALL START
            AGAIN = EXPR(W)
            WRITE (6, 120) AGAIN .EQ. S, W .EQ. V
  120       FORMAT (' READ AGAIN:', 2L2)
         END IF
   10 CONTINUE
      END

C Ready to read SRC from its first column.
      SUBROUTINE START
      CHARACTER*60 SRC
      INTEGER POS, NLEN, IERR, LENTRM
      COMMON /SCAN/ POS, NLEN, IERR
      COMMON /TEXT/ SRC
      POS = 1
      NLEN = LENTRM(SRC)
      IERR = 0
      END

C expression = term { (+ | -) term }
      RECURSIVE CHARACTER*60 FUNCTION EXPR(V) RESULT(S)
      CHARACTER*60 R, TERM
      CHARACTER*1 OP, PEEK
      INTEGER V, W, POS, NLEN, IERR, LENTRM
      COMMON /SCAN/ POS, NLEN, IERR
      S = TERM(V)
      OP = PEEK()
      DO WHILE (OP .EQ. '+' .OR. OP .EQ. '-')
         POS = POS + 1
         R = TERM(W)
         S = '(' // S(1:LENTRM(S)) // OP // R(1:LENTRM(R)) // ')'
         IF (OP .EQ. '+') THEN
            V = V + W
         ELSE
            V = V - W
         END IF
         OP = PEEK()
      END DO
      END

C term = factor { (* | /) factor }
      RECURSIVE CHARACTER*60 FUNCTION TERM(V) RESULT(S)
      CHARACTER*60 R, FACTOR
      CHARACTER*1 OP, PEEK
      INTEGER V, W, POS, NLEN, IERR, LENTRM
      COMMON /SCAN/ POS, NLEN, IERR
      S = FACTOR(V)
      OP = PEEK()
      DO WHILE (OP .EQ. '*' .OR. OP .EQ. '/')
         POS = POS + 1
         R = FACTOR(W)
         S = '(' // S(1:LENTRM(S)) // OP // R(1:LENTRM(R)) // ')'
         IF (OP .EQ. '*') THEN
            V = V * W
         ELSE IF (W .NE. 0) THEN
            V = V / W
         END IF
         OP = PEEK()
      END DO
      END

C factor = number | - factor | ( expression ). What isn't one is a ?, and the
C first error's column is kept.
      RECURSIVE CHARACTER*60 FUNCTION FACTOR(V) RESULT(S)
      CHARACTER*60 SRC, R, EXPR
      CHARACTER*1 C, PEEK
      INTEGER V, POS, NLEN, IERR, FIRST, LENTRM
      COMMON /SCAN/ POS, NLEN, IERR
      COMMON /TEXT/ SRC
      C = PEEK()
      IF (C .EQ. '(') THEN
         POS = POS + 1
         S = EXPR(V)
         IF (PEEK() .EQ. ')') THEN
            POS = POS + 1
         ELSE IF (IERR .EQ. 0) THEN
            IERR = POS
         END IF
      ELSE IF (C .EQ. '-') THEN
         POS = POS + 1
         R = FACTOR(V)
         S = '-' // R(1:LENTRM(R))
         V = -V
      ELSE IF (C .GE. '0' .AND. C .LE. '9') THEN
         FIRST = POS
         V = 0
         DO WHILE (SRC(POS:POS) .GE. '0' .AND. SRC(POS:POS) .LE. '9')
            V = V * 10 + ICHAR(SRC(POS:POS)) - ICHAR('0')
            POS = POS + 1
         END DO
         S = SRC(FIRST:POS-1)
      ELSE
         S = '?'
         V = 0
         IF (IERR .EQ. 0) IERR = POS
      END IF
      END

C The next character that isn't a blank, a blank at the end.
      CHARACTER*1 FUNCTION PEEK()
      CHARACTER*60 SRC
      INTEGER POS, NLEN, IERR
      COMMON /SCAN/ POS, NLEN, IERR
      COMMON /TEXT/ SRC
      PEEK = ' '
      DO WHILE (POS .LE. NLEN)
         IF (SRC(POS:POS) .NE. ' ') THEN
            PEEK = SRC(POS:POS)
            RETURN
         END IF
         POS = POS + 1
      END DO
      END

C The length of a text without its trailing blanks.
      INTEGER FUNCTION LENTRM(S)
      CHARACTER*(*) S
      DO 10 LENTRM = LEN(S), 1, -1
         IF (S(LENTRM:LENTRM) .NE. ' ') RETURN
   10 CONTINUE
      LENTRM = 0
      END
