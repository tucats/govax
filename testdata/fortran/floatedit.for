      PROGRAM FLOATEDIT
C     Floating editing, Phase 80's subtask 6: F, E, D, and G with widths
C     and digits, the scale factor and how long it lasts, signs, complex
C     values as two fields, the default widths of Table 8-4, the forms G
C     chooses, and each format of REAL*8 and REAL*16. A field too narrow
C     is asterisks, and IOSTAT gets OUTCONERR (63) while the statement
C     goes on; F of an INTEGER edits its bits and fails with FORVARMIS (61)
C     at the statement's end, its records written.
      REAL X, Y
      REAL*8 D
      REAL*16 Q
      COMPLEX C
      INTEGER IOS
   10 FORMAT (1X, F8.5, F9.3, F10.4, F5.2)
   20 FORMAT (1X, F2.1, F4.2, F2.0, F5.0, F6.1)
   30 FORMAT (1X, E9.2, E12.3, E8.3, E14.5E4)
   40 FORMAT (1X, 1PE12.3, -1PE12.2, 4PE12.3, 0PE12.3)
   50 FORMAT (1X, F8.2, 2PF8.2, F8.2)
   60 FORMAT (1X, G13.6)
   70 FORMAT (1X, 2PG13.3, G13.3)
   80 FORMAT (1X, SP, F8.2, E12.3, G13.3, SS, F8.2, S, F8.2)
   90 FORMAT (1X, 2F8.5)
      X = 2.3547188
      Y = -23.24352
      WRITE (6, 10) X, 8789.7361, Y, -0.2
      WRITE (6, 20) 0.5, -0.2, 0.0, 0.6, -0.01
      WRITE (6, 30) 475867.222, 0.00069, 0.5555, -1.001
      WRITE (6, 40) -270.139, -270.139, 270.139, 270.139
      WRITE (6, 50) 25.0, 25.0, 25.0
      WRITE (6, 60) 0.01234567, 123456.789, -1234567.89, 0.0
      WRITE (6, 60) 0.09999999, -0.12345678
      WRITE (6, 70) 5.0, 5.0E8
      WRITE (6, 80) 1.0, 1.0, 1.0, 1.0, 1.0
      C = (2.3547188, 3.4567345)
      WRITE (6, 90) C
      WRITE (6, '(1X,F8.2)') C
C     Rounding is on the value's exact digits, and a value that shows only
C     zeros has no minus sign.
      WRITE (6, '(1X,F5.1,F5.1,F6.1,F6.1)') 2.25, 2.35, -0.01, -0.06
C     REAL*8 and REAL*16, with D and the default widths.
      D = 5413.87625793D0
      WRITE (6, '(1X,D23.12,F12.4)') D, D
      Q = 3.25Q0
      WRITE (6, '(1X,F8.3,E12.4E3)') Q, Q
      WRITE (6, '(1X,F)') 3.25
      WRITE (6, '(1X,E)') 3.25
      WRITE (6, '(1X,G)') 3.25
      WRITE (6, '(1X,D)') 3.25
      WRITE (6, '(1X,F)') 3.25D0
      WRITE (6, '(1X,E)') 3.25D0
      WRITE (6, '(1X,G)') 3.25D0
      WRITE (6, '(1X,F)') Q
      WRITE (6, '(1X,E)') Q
      WRITE (6, '(1X,G)') Q
C     An exponent too wide is asterisks, and so is a field too narrow.
      WRITE (6, '(1X,E12.4E1)', IOSTAT=IOS) 1.0E12
      PRINT *, 'OUTCONERR', IOS
      WRITE (6, '(1X,F2.1,F5.2)', IOSTAT=IOS) 51.44, 1.0
      PRINT *, 'OUTCONERR', IOS
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) 3, 4
      PRINT *, 'FORVARMIS', IOS
      END
