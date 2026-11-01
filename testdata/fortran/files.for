C FILES - OPEN and CLOSE on sequential files (chapter 9), each failure's
C IOSTAT the number VMS's FORTRAN run-time library gave in testdata/for81's
C runs, and each rule theirs.
      PROGRAM FILES
      INTEGER IOS, N, I
      CHARACTER*20 LINE, WHAT
      CHARACTER*400 LONG
C
C A new file written, closed, and read back.
C
      OPEN (UNIT=1, FILE='A1.DAT', STATUS='NEW')
      WRITE (1, '(A)') 'FIRST'
      WRITE (1, 10) 42
   10 FORMAT (' N=', I4)
      CLOSE (1)
      OPEN (1, FILE='A1.DAT', STATUS='OLD')
      READ (1, '(A)') LINE
      PRINT *, LINE
      READ (1, '(A)') LINE
      PRINT *, LINE
      READ (1, '(A)', IOSTAT=IOS) LINE
      PRINT *, 'END OF FILE', IOS
      CLOSE (1)
C
C What a file that can't be opened says.
C
      OPEN (2, FILE='NOSUCH.DAT', STATUS='OLD', IOSTAT=IOS)
      PRINT *, 'MISSING', IOS
      OPEN (2, FILE='NOSUCH:[X]Y.DAT', IOSTAT=IOS, ERR=20)
      PRINT *, 'NOT HERE'
   20 PRINT *, 'NO DEVICE', IOS
      OPEN (2, FILE='A[B', IOSTAT=IOS)
      PRINT *, 'BAD NAME', IOS
      OPEN (2, FILE='[NOSUCH]Z.DAT', STATUS='NEW', IOSTAT=IOS)
      PRINT *, 'NO DIRECTORY', IOS
      OPEN (120, FILE='A1.DAT', IOSTAT=IOS)
      PRINT *, 'BAD UNIT', IOS
C
C Run-time values get the rules a constant gets from the compiler.
C
      WHAT = 'NEWS'
      OPEN (2, FILE='A2.DAT', STATUS=WHAT, IOSTAT=IOS)
      PRINT *, 'BAD STATUS', IOS
      WHAT = 'new'
      OPEN (2, FILE='A2.DAT', STATUS=WHAT, READONLY, IOSTAT=IOS)
      PRINT *, 'READONLY NEW', IOS
      OPEN (2, FILE='A2.DAT', STATUS=WHAT, IOSTAT=IOS)
      PRINT *, 'ANY CASE', IOS
      WHAT = 'KEEP'
      CLOSE (2, DISPOSE=WHAT)
      WHAT = 'FIXED'
      OPEN (2, FILE='A3.DAT', RECORDTYPE=WHAT, IOSTAT=IOS)
      PRINT *, 'FIXED, NO RECL', IOS
      N = -1
      OPEN (2, FILE='A3.DAT', RECL=N, IOSTAT=IOS)
      PRINT *, 'NEGATIVE RECL', IOS
      N = 200
      OPEN (2, FILE='A3.DAT', BUFFERCOUNT=N, IOSTAT=IOS)
      PRINT *, 'BUFFERCOUNT', IOS
C
C Fixed-length records are filled out with blanks, and an existing file's
C attributes must agree with what is asked for.
C
      OPEN (3, FILE='A4.DAT', RECORDTYPE='FIXED', RECL=10,
     1      STATUS='NEW')
      WRITE (3, '(A)') 'ABC'
      WRITE (3, '(A)', IOSTAT=IOS) '12345678901'
      PRINT *, 'TOO LONG', IOS
      CLOSE (3)
      OPEN (3, FILE='A4.DAT', STATUS='OLD', RECL=5, IOSTAT=IOS)
      PRINT *, 'ANOTHER RECL', IOS
      OPEN (3, FILE='A4.DAT', STATUS='OLD', RECORDTYPE='VARIABLE',
     1      IOSTAT=IOS)
      PRINT *, 'ANOTHER TYPE', IOS
      OPEN (3, FILE='A4.DAT', STATUS='OLD', RECORDTYPE='FIXED', RECL=10,
     1      IOSTAT=IOS)
      PRINT *, 'THE SAME', IOS
      READ (3, '(A)') LINE
      PRINT *, '[', LINE, ']'
      CLOSE (3)
C
C STATUS='SCRATCH' and the dispositions.
C
      OPEN (4, FILE='A5.DAT', STATUS='SCRATCH')
      WRITE (4, '(A)') 'GONE'
      CLOSE (4, DISPOSE='KEEP', IOSTAT=IOS)
      PRINT *, 'SCRATCH KEPT', IOS
      CLOSE (4)
      OPEN (4, FILE='A5.DAT', STATUS='OLD', IOSTAT=IOS)
      PRINT *, 'SCRATCH GONE', IOS
      OPEN (4, FILE='A6.DAT', STATUS='NEW')
      CLOSE (4, DISP='DELETE')
      OPEN (4, FILE='A6.DAT', STATUS='OLD', IOSTAT=IOS)
      PRINT *, 'DELETED', IOS
      OPEN (4, FILE='A7.DAT', STATUS='NEW', DISPOSE='DELETE')
      CLOSE (4, STATUS='SAVE')
      OPEN (4, FILE='A7.DAT', STATUS='OLD', IOSTAT=IOS)
      PRINT *, 'SAVED', IOS
      CLOSE (4, STATUS='PRINT')
      OPEN (4, FILE='A7.DAT', STATUS='OLD', IOSTAT=IOS, READONLY)
      PRINT *, 'PRINTED, THERE', IOS
      WRITE (4, '(A)', IOSTAT=IOS) 'NO'
      PRINT *, 'WRITE READONLY', IOS
      CLOSE (4, STATUS='DELETE', IOSTAT=IOS)
      PRINT *, 'DELETE READONLY', IOS
      CLOSE (4)
C
C ACCESS='APPEND', and CLOSE of what isn't open.
C
      OPEN (5, FILE='A1.DAT', ACCESS='APPEND', STATUS='OLD')
      WRITE (5, '(A)') 'LAST'
      CLOSE (5)
      OPEN (5, FILE='A1.DAT', STATUS='OLD')
      DO 30 I = 1, 3
         READ (5, '(A)') LINE
   30 CONTINUE
      PRINT *, LINE
      CLOSE (5)
      CLOSE (5, IOSTAT=IOS)
      PRINT *, 'CLOSE TWICE', IOS
      CLOSE (77, IOSTAT=IOS)
      PRINT *, 'NEVER OPEN', IOS
      CLOSE (120, IOSTAT=IOS)
      PRINT *, 'CLOSE BAD UNIT', IOS
C
C A connected unit: on the same file only BLANK changes; on another the first
C closes.
C
      OPEN (7, FILE='A8.DAT', STATUS='NEW')
      WRITE (7, '(A)') '1 2'
      CLOSE (7)
      OPEN (7, FILE='A8.DAT', STATUS='OLD', BLANK='ZERO')
      READ (7, '(I3)') N
      PRINT *, 'ZERO', N
      CLOSE (7)
      OPEN (7, FILE='A8.DAT', STATUS='OLD', BLANK='ZERO')
      OPEN (7, FILE='A8.DAT', BLANK='NULL', IOSTAT=IOS)
      PRINT *, 'THE SAME FILE', IOS
      READ (7, '(I3)') N
      PRINT *, 'NULL', N
      OPEN (7, FILE='A9.DAT', STATUS='NEW', DISPOSE='DELETE')
      OPEN (7, FILE='A8.DAT', STATUS='OLD', IOSTAT=IOS)
      PRINT *, 'ANOTHER FILE', IOS
      CLOSE (7)
      OPEN (7, FILE='A9.DAT', STATUS='OLD', IOSTAT=IOS)
      PRINT *, 'THE FIRST WENT', IOS
C
C A unit with no OPEN is FORnnn.DAT, and has BLANK='ZERO'.
C
      WRITE (20, '(A)') '3 4'
      WRITE (20, *) 7, 8
      CLOSE (20)
      READ (20, '(I3)') N
      PRINT *, 'IMPLICIT', N
      READ (20, *) N, I
      PRINT *, 'LIST', N, I
      CLOSE (20)
      OPEN (21, STATUS='OLD', FILE='FOR020', DEFAULTFILE='.DAT',
     1      IOSTAT=IOS)
      PRINT *, 'DEFAULTFILE', IOS
      CLOSE (21, DISP='DELETE')
      READ (20, '(A)', IOSTAT=IOS) LINE
      PRINT *, 'NO FILE', IOS
C
C A long record, past 133 bytes and the old 256, with its RECL; a shorter RECL
C for a file with longer records takes the whole record.
C
      LONG = 'START'
      LONG(390:) = 'END'
      OPEN (8, FILE='A10.DAT', RECL=400, STATUS='NEW')
      WRITE (8, '(A)') LONG
      CLOSE (8)
      OPEN (8, FILE='A10.DAT', STATUS='OLD', RECL=50)
      LONG = ' '
      READ (8, '(A)') LONG
      PRINT *, LONG(1:5), LONG(390:392)
      CLOSE (8)
      OPEN (8, FILE='A11.DAT', STATUS='NEW')
      WRITE (8, '(A)', IOSTAT=IOS) LONG
      PRINT *, 'PAST 133', IOS
      CLOSE (8, DISP='DELETE')
C
C Units 5 and 6 may be closed, and come back.
C
      WRITE (6, '(A)') ' SIX'
      CLOSE (6)
      WRITE (6, '(A)') ' SIX AGAIN'
      PRINT *, 'DONE'
      END
