      PROGRAM GRADES
C     Block IF with ELSE IF and ELSE, a block IF nested in another, and
C     STOP with a display ending the program before its END.
      INTEGER S(7), I, NPASS
      LOGICAL HONORS
      S(1) = 95
      S(2) = 83
      S(3) = 71
      S(4) = 64
      S(5) = 100
      S(6) = -1
      S(7) = 88
      NPASS = 0
      DO 10 I = 1, 7
         IF (S(I) .LT. 0) STOP 'END OF SCORES'
         HONORS = .FALSE.
         IF (S(I) .GE. 90) THEN
            PRINT *, S(I), ' A'
            IF (S(I) .EQ. 100) THEN
               HONORS = .TRUE.
            END IF
         ELSE IF (S(I) .GE. 80) THEN
            PRINT *, S(I), ' B'
         ELSE IF (S(I) .GE. 70) THEN
            PRINT *, S(I), ' C'
         ELSE
            PRINT *, S(I), ' F'
            GO TO 10
         END IF
         NPASS = NPASS + 1
         PRINT *, 'Passed so far:', NPASS, '  honors:', HONORS
   10 CONTINUE
      PRINT *, 'Not reached'
      END
