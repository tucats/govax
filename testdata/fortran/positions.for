C POSITIONS - REWIND, BACKSPACE, and ENDFILE (sections 9.4 to 9.6), each
C failure's IOSTAT the number VMS's FORTRAN run-time library gave in
C testdata/for81's runs.
      PROGRAM POSITIONS
      INTEGER IOS, N, I, COUNT, U
      LOGICAL OPENED
      CHARACTER*12 LINE, FORM
C
C Back and forth over five records.
C
      OPEN (1, FILE='P1.DAT', STATUS='NEW')
      DO 10 I = 1, 5
   10 WRITE (1, '(I6)') I*I
      REWIND 1
      READ (1, '(I6)') N
      PRINT *, 'FIRST', N
      READ (1, '(I6)') N
      PRINT *, 'SECOND', N
      BACKSPACE 1
      READ (1, '(I6)') N
      PRINT *, 'SECOND AGAIN', N
      BACKSPACE (UNIT=1)
      BACKSPACE (1)
      READ (1, '(I6)') N
      PRINT *, 'FIRST AGAIN', N
C
C BACKSPACE at the start stays there, and the end of the file counts no
C record.
C
      REWIND (UNIT=1, IOSTAT=IOS, ERR=90)
      BACKSPACE 1
      COUNT = 0
   20 READ (1, '(I6)', END=30) N
      COUNT = COUNT + 1
      GO TO 20
   30 PRINT *, 'RECORDS', COUNT, IOS
      READ (1, '(I6)', IOSTAT=IOS) N
      PRINT *, 'PAST THE END', IOS
      BACKSPACE 1
      READ (1, '(I6)') N
      PRINT *, 'LAST', N
C
C A write after a read ends the file there.
C
      REWIND 1
      READ (1, '(I6)') N
      WRITE (1, '(I6)') 99
      READ (1, '(I6)', IOSTAT=IOS) N
      PRINT *, 'AFTER THE WRITE', IOS
      REWIND 1
      READ (1, '(I6)') N
      PRINT *, N
      READ (1, '(I6)') N
      PRINT *, N
      READ (1, '(I6)', IOSTAT=IOS) N
      PRINT *, IOS
      CLOSE (1, DISP='DELETE')
C
C ENDFILE's records are ends of file a READ can go on past, and BACKSPACE
C steps over.
C
      OPEN (2, FILE='P2.DAT', STATUS='NEW')
      WRITE (2, '(A)') ' PART ONE'
      ENDFILE 2
      WRITE (2, '(A)') ' PART TWO'
      ENDFILE (2, ERR=90)
      REWIND 2
      DO 40 I = 1, 5
      READ (2, '(A)', END=35) LINE
      PRINT *, LINE
      GO TO 40
   35 PRINT *, 'END OF PART', I
   40 CONTINUE
      BACKSPACE 2
      BACKSPACE 2
      READ (2, '(A)', IOSTAT=IOS) LINE
      PRINT *, IOS
      BACKSPACE 2
      READ (2, '(A)', IOSTAT=IOS) LINE
      PRINT *, IOS
      CLOSE (2, DISP='DELETE')
C
C What each statement says where it can't work.
C
      OPEN (3, FILE='P3.DAT', STATUS='NEW', ACCESS='DIRECT', RECL=4)
      REWIND (3, IOSTAT=IOS)
      PRINT *, 'DIRECT REWIND', IOS
      BACKSPACE (3, IOSTAT=IOS)
      PRINT *, 'DIRECT BACKSPACE', IOS
      ENDFILE (3, IOSTAT=IOS)
      PRINT *, 'DIRECT ENDFILE', IOS
      BACKSPACE (3, ERR=50)
      PRINT *, 'NOT HERE'
   50 CLOSE (3, DISP='DELETE')
      OPEN (3, FILE='P3.DAT', STATUS='NEW', RECORDTYPE='FIXED', RECL=8)
      ENDFILE (3, IOSTAT=IOS)
      PRINT *, 'FIXED ENDFILE', IOS
      REWIND 3
      BACKSPACE 3
      CLOSE (3, DISP='DELETE')
      U = 120
      REWIND (U, IOSTAT=IOS)
      PRINT *, 'NO SUCH UNIT', IOS
      BACKSPACE (U, IOSTAT=IOS)
      PRINT *, IOS
      ENDFILE (U, IOSTAT=IOS)
      PRINT *, IOS
C
C A unit not connected: only ENDFILE connects it, with no form yet.
C
      REWIND 9
      BACKSPACE 9
      INQUIRE (9, OPENED=OPENED)
      PRINT *, OPENED
      ENDFILE 9
      INQUIRE (9, OPENED=OPENED, FORM=FORM)
      PRINT *, OPENED, ' ', FORM
      WRITE (9, '(A)') ' WORDS'
      INQUIRE (9, FORM=FORM)
      PRINT *, FORM
      REWIND 9
      READ (9, '(A)', IOSTAT=IOS) LINE
      PRINT *, IOS
      READ (9, '(A)') LINE
      PRINT *, LINE
      CLOSE (9, DISP='DELETE')
C
C APPEND to a file that was there has no count of its records until a
C REWIND gives it one.
C
      OPEN (4, FILE='P4.DAT', STATUS='NEW')
      WRITE (4, '(A)') ' OLD'
      CLOSE (4)
      OPEN (4, FILE='P4.DAT', STATUS='OLD', ACCESS='APPEND')
      WRITE (4, '(A)') ' NEW'
      BACKSPACE (4, IOSTAT=IOS)
      PRINT *, 'APPEND BACKSPACE', IOS
      REWIND 4
      READ (4, '(A)') LINE
      READ (4, '(A)') LINE
      BACKSPACE 4
      READ (4, '(A)') LINE
      PRINT *, LINE
      CLOSE (4, DISP='DELETE')
C
C On a file READONLY, moving is fine, and ENDFILE isn't.
C
      OPEN (4, FILE='P5.DAT', STATUS='NEW')
      WRITE (4, '(A)') ' ONLY'
      CLOSE (4)
      OPEN (4, FILE='P5.DAT', STATUS='OLD', READONLY)
      READ (4, '(A)') LINE
      BACKSPACE 4
      READ (4, '(A)') LINE
      PRINT *, LINE
      ENDFILE (4, IOSTAT=IOS)
      PRINT *, 'READONLY ENDFILE', IOS
      CLOSE (4)
      OPEN (4, FILE='P5.DAT', STATUS='OLD')
      CLOSE (4, DISP='DELETE')
      STOP
   90 PRINT *, 'NOT HERE EITHER'
      END
