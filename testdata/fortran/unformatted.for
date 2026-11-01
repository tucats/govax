C UNFORMATTED - unformatted sequential READ and WRITE (sections 7.2.1.4
C and 7.3.1.4): values of every type written as their bytes and read back,
C whole arrays and implied-DO lists, records shorter and longer than a
C READ's list, a record long enough to need several segments, BACKSPACE
C over it, a unit with no OPEN, and each failure's IOSTAT the number VMS's
C FORTRAN run-time library gave in testdata/for81's runs.
      PROGRAM UNFORMATTED
      INTEGER*2 H
      INTEGER I, J, N, IOS, SUM, BIG(1000), K(3)
      REAL X
      DOUBLE PRECISION D
      COMPLEX Z
      LOGICAL L, OPENED
      CHARACTER*6 S
      CHARACTER*12 FORM, TYPE
C
C One record of every type, read back as written.
C
      OPEN (1, FILE='U1.DAT', STATUS='NEW', FORM='UNFORMATTED')
      WRITE (1) 7, 3.5, 2.25D0, (1.0, -2.0), .TRUE., 'ABCDEF'
      WRITE (1) (I*I, I = 1, 3)
      WRITE (1)
      WRITE (1) 'LAST'
      REWIND 1
      READ (1) N, X, D, Z, L, S
      WRITE (6, '(1X,I3,F6.2,F6.2,2F6.1,L2,1X,A)') N, X, D, Z, L, S
C
C An array takes as many bytes as it holds, and a READ may take less than
C its record, the rest passed over.
C
      READ (1) K
      WRITE (6, '(1X,3I4)') K
      REWIND 1
      READ (1) H
      WRITE (6, '(1X,I6)') H
      READ (1) (K(I), I = 3, 1, -1)
      WRITE (6, '(1X,3I4)') K
C
C A READ that asks for more than its record holds fails: FOR$_INPSTAREQ,
C 67. One with no list passes a record over. Then the end.
C
      READ (1, IOSTAT=IOS) N
      WRITE (6, '(1X,A,I4)') 'PAST THE RECORD', IOS
      READ (1)
      READ (1, END=10) N
      WRITE (6, *) 'NOT HERE'
   10 WRITE (6, '(1X,A)') 'END OF U1'
      CLOSE (1, DISP='DELETE')
C
C A record of 4,000 bytes is two segments, and BACKSPACE steps over both.
C
      OPEN (2, FILE='U2.DAT', STATUS='NEW', FORM='UNFORMATTED')
      DO 20 I = 1, 1000
   20 BIG(I) = I
      WRITE (2) 1
      WRITE (2) BIG
      WRITE (2) 3
      BACKSPACE 2
      BACKSPACE 2
      DO 30 I = 1, 1000
   30 BIG(I) = 0
      READ (2) BIG
      SUM = 0
      DO 40 I = 1, 1000
   40 SUM = SUM + BIG(I)
      WRITE (6, '(1X,A,I8)') 'SUM', SUM
      READ (2) N
      WRITE (6, '(1X,A,I4)') 'AFTER IT', N
      BACKSPACE 2
      BACKSPACE 2
      READ (2) (BIG(I), I = 1, 2)
      WRITE (6, '(1X,A,2I4)') 'ITS START', BIG(1), BIG(2)
      READ (2) N
      WRITE (6, '(1X,A,I4)') 'AND AFTER', N
      CLOSE (2, DISP='DELETE')
C
C A unit with no OPEN is connected unformatted, with segments.
C
      WRITE (14) 42
      INQUIRE (14, FORM=FORM, RECORDTYPE=TYPE)
      WRITE (6, '(1X,A,1X,A)') FORM, TYPE
      REWIND 14
      READ (14) N
      WRITE (6, '(1X,I4)') N
      ENDFILE 14
      REWIND 14
      READ (14) N
      READ (14, IOSTAT=IOS) N
      WRITE (6, '(1X,A,I4)') 'ENDFILE', IOS
      CLOSE (14, DISP='DELETE')
C
C Records with no segments: a write too long for one fails,
C FOR$_OUTSTAOVE, 66, and writes nothing; a fixed one is filled out.
C
      OPEN (3, FILE='U3.DAT', STATUS='NEW', FORM='UNFORMATTED',
     1      RECORDTYPE='VARIABLE', RECL=2)
      WRITE (3) 1000, 2
      WRITE (3, IOSTAT=IOS) 1, 2, 3
      WRITE (6, '(1X,A,I4)') 'TOO LONG', IOS
      REWIND 3
      READ (3) I, J
      WRITE (6, '(1X,2I4)') I, J
      READ (3, IOSTAT=IOS) I
      WRITE (6, '(1X,A,I4)') 'NOTHING MORE', IOS
      CLOSE (3)
C
C Read as segmented, those records aren't: FOR$_SEGRECFOR, 35, for a
C first word that is no segment's control (1000).
C
      OPEN (3, FILE='U3.DAT', STATUS='OLD', FORM='UNFORMATTED',
     1      RECORDTYPE='SEGMENTED')
      READ (3, IOSTAT=IOS) I
      WRITE (6, '(1X,A,I4)') 'NO SEGMENTS', IOS
      CLOSE (3, DISP='DELETE')
      OPEN (3, FILE='U3.DAT', STATUS='NEW', FORM='UNFORMATTED',
     1      RECORDTYPE='FIXED', RECL=3)
      WRITE (3) 5
      REWIND 3
      READ (3) (K(I), I = 1, 3)
      WRITE (6, '(1X,3I4)') K
      CLOSE (3, DISP='DELETE')
C
C Each form's statements on the other's units: FOR$_MIXFILACC, 31.
C
      OPEN (4, FILE='U4.DAT', STATUS='NEW', FORM='UNFORMATTED')
      WRITE (4, '(I4)', IOSTAT=IOS) 1
      WRITE (6, '(1X,A,I4)') 'FORMATTED ON UNFORMATTED', IOS
      CLOSE (4, DISP='DELETE')
      OPEN (4, FILE='U4.DAT', STATUS='NEW')
      WRITE (4, IOSTAT=IOS, ERR=50) 1
   50 WRITE (6, '(1X,A,I4)') 'UNFORMATTED ON FORMATTED', IOS
      CLOSE (4, DISP='DELETE')
      INQUIRE (4, OPENED=OPENED)
      WRITE (6, '(1X,L1)') OPENED
      END
