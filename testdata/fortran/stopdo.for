C     STOP may end a DO loop's range (section 5.3.1 lists what may not),
C     and a jump to it runs it: the loop never reaches its second pass.
      PROGRAM STOPDO
      DO 10 I = 1, 3
         PRINT *, I
         IF (I .GT. 5) GO TO 10
         PRINT *, 'END OF THE BODY'
   10 STOP 'THE TERMINAL STATEMENT'
      PRINT *, 'NOT REACHED'
      END
