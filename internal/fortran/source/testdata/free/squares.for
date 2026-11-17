C Squares: the same program in fixed form (here) and free form
C (squares.f90).
      PROGRAM SQUARES
      INTEGER :: I, TOTAL
      TOTAL = 0
      DO 10 I = 1, 5
      TOTAL = TOTAL + I*I
   10 CONTINUE
      PRINT *, 'the sum of the squares ',
     1         'of 1 to 5 is', TOTAL   ! a comment
      PRINT 20,
     1   TOTAL
   20 FORMAT (' TOTAL = ',
     1        I5)
      END PROGRAM SQUARES
