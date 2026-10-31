C     Run-time formats (section 8.7): the manual's example, and a format in
C     each kind of place.
      PROGRAM RUNFMT
      REAL TABLE(10,5)
      INTEGER F(3), G(4)
      REAL*8 D(2)
      CHARACTER*12 S
      CHARACTER*6 PART(2)
      CHARACTER*8 CF
      DATA F /4H(1X,, 4HI5,I, 4H3)  /
      DATA G /4HJUNK, 4H(1X,, 4H'G',, 4HI2) /
      DATA D /8H(1X,'D',, 8HI3)     /
      DATA PART /'(1X,I2', ',I4)  '/
      DATA TABLE /100.5, .01, 8*0., 50.25, 1000., 8*0., .05, 2.5, 8*0.,
     1            200., .5, 8*0., 1., .09, 8*0./
      CALL PRINT(TABLE)
C     An array and its elements.
      WRITE (6, F) 1, 2
      WRITE (6, F(1)) 3, 4
      WRITE (6, G(2)) 5
      WRITE (6, D) 6
      WRITE (6, PART) 7, 8
      WRITE (6, PART(1)//PART(2)) 9, 10
C     A CHARACTER variable, changed between uses, in capitals or not.
      DO 10 I = 2, 4
      S = '(1x,i'//CHAR(ICHAR('0')+I)//')'
      WRITE (6, S) I
   10 CONTINUE
C     A substring, an expression, a function, a constant, and a dummy.
      S = 'XX(1X,I2)YY'
      WRITE (6, S(3:9)) 11
      WRITE (6, '(1X,'//'I3)') 12
      WRITE (6, CF(4)) 13
      WRITE (6, '(1X,I5)TRAILING') 14
      CALL DUMMY('(1X,I6)', 15)
C     One that isn't right, found when used.
      S = '(1X,I2'
      WRITE (6, S, IOSTAT=IOS) 16
      WRITE (6, '(1X,I3)') IOS
      END
      CHARACTER*8 FUNCTION CF(N)
      CF = '(1X,I'//CHAR(ICHAR('0')+N)//')'
      END
      SUBROUTINE DUMMY(FORM, N)
      CHARACTER*(*) FORM
      WRITE (6, FORM) N
      END
      SUBROUTINE PRINT(TABLE)
      REAL TABLE(10,5)
      CHARACTER*5 FORCHR(0:5), RPAR*1, FBIG, FMED, FSML
      DATA FORCHR(0),RPAR /' (', ')'/
      DATA FBIG,FMED,FSML /'F8.2,','F9.4,','F9.6,'/
      DO 20 I = 1, 2
      DO 18 J = 1, 5
      IF (TABLE(I,J) .GE. 100.) THEN
      FORCHR(J) = FBIG
      ELSE IF (TABLE(I,J) .GT. 0.1) THEN
      FORCHR(J) = FMED
      ELSE
      FORCHR(J) = FSML
      END IF
   18 CONTINUE
      FORCHR(5)(5:5) = RPAR
      WRITE (6, FORCHR) (TABLE(I,J), J = 1, 5)
   20 CONTINUE
      END
