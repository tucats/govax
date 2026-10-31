      PROGRAM FORMATS
C     Formatted output as Phase 80's subtask 4 leaves it: the format
C     interpreter's text, H, positions, records, groups, reversion, colon,
C     and dollar sign; A and Q; run-time formats in a variable and in an
C     array; and the conditions the walk raises. Real editing is a
C     later subtask's: NOTIMP, FOR number 2, until then.
      CHARACTER*5 NAME(3)
      CHARACTER*12 RTF
      CHARACTER*6 PARTS(2)
      INTEGER IOS, I
      DATA NAME /'ALPHA', 'BRAVO', 'GAMMA'/
   10 FORMAT (' TEXT AND ', 5HHOLLE, 'RITH')
   20 FORMAT (1X,'A',T6,'B',TL3,'C',TR2,'D',3X,'E',5X)
   30 FORMAT (1X, 3(A5, :, ', '))
   40 FORMAT (1X, 'NAMES:', 2(1X, A5))
   50 FORMAT (' ONE', //, ' THREE', /)
   60 FORMAT (1X, A3, '|', A7, '|', A)
      WRITE (6, 10)
      PRINT 10
      WRITE (6, 20)
      PRINT 30, (NAME(I), I = 1, 3)
      PRINT 30, (NAME(I), I = 1, 2)
      WRITE (6, 40) NAME, 'DELTA'
      WRITE (6, 50)
      WRITE (6, '('' PROMPT: '',$)')
      WRITE (6, '(''+CONTINUED'')')
      RTF = '(1X,A,A)'
      WRITE (6, RTF) 'RUN-', 'TIME'
      PARTS(1) = '(1X,''A'
      PARTS(2) = 'RRAY'')'
      WRITE (6, PARTS)
      WRITE (6, 60) 'ABCDE', 'ABCDE', 'ABCDE'
      WRITE (6, '(1X, A4)') 1094861636
      WRITE (6, '(1X, Q, A)') 99, 'Q TAKES 99'
      TYPE '(1X,''TYPE'')'
      WRITE (6, '(1X I3)', IOSTAT=IOS) 1
      PRINT *, 'SYNERRFOR', IOS
      WRITE (6, '(1X,''NO DATA'')', ERR=70) 'X'
      PRINT *, 'NOT HERE'
   70 PRINT *, 'INFFORLOO TAKEN BY ERR='
      WRITE (6, '(1X,T134,''X'')', IOSTAT=IOS)
      PRINT *, 'OUTSTAOVE', IOS
      WRITE (6, '(1X,T133,''X'')', IOSTAT=IOS)
      PRINT *, 'T133', IOS
      WRITE (6, '(1X,A,F5.1)', IOSTAT=IOS) 'F5.1 IS', 5.0
      PRINT *, 'NOTIMP', IOS
      END
