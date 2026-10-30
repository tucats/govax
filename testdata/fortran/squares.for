C     SQUARES: the squares of the first ten integers.  The main program
C     has no PROGRAM statement, so it is named SQUARES$MAIN, for the file.
      PRINT *, 'The squares of 1 to 10'
      DO 10 I = 1, 10
         PRINT *, I, I*I
   10 CONTINUE
      END
