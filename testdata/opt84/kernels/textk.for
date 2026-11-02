      PROGRAM TEXTK
C     Character work: blanks squeezed from lines, words counted.
      CHARACTER*80 LINE, OUT
      INTEGER I, J, L, WORDS, ITER
      LOGICAL INWORD
      LINE = ' THE QUICK BROWN   FOX JUMPS OVER  THE LAZY DOG  AGAIN'
      DO 100 ITER = 1, 500
         J = 0
         WORDS = 0
         INWORD = .FALSE.
         OUT = ' '
         DO 10 I = 1, LEN(LINE)
            IF (LINE(I:I) .NE. ' ') THEN
               J = J + 1
               OUT(J:J) = LINE(I:I)
               IF (.NOT. INWORD) WORDS = WORDS + 1
               INWORD = .TRUE.
            ELSE
               INWORD = .FALSE.
            END IF
   10    CONTINUE
  100 CONTINUE
      PRINT *, WORDS, OUT(1:J)
      END
