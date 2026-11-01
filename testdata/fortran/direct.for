C DIRECT - direct access READ and WRITE (sections 7.2.2 and 7.3.2), FIND
C (A.3), DEFINE FILE (A.2), and ASSOCIATEVARIABLE (9.1.2): records written
C out of order and read back, the records between them zeros, a REAL
C record number, the associated variable stepping through a file, a format
C of two records writing two, a unit DEFINE FILE describes opened by its
C first statement, and each failure's IOSTAT the number VMS's FORTRAN
C run-time library gave in testdata/for81's runs.
      PROGRAM DIRECT
      INTEGER I, J, K, N, IOS, NR, KEY(3)
      INTEGER*2 NR2
      REAL X
      CHARACTER*8 S, T
      LOGICAL OPENED
C
C Unformatted records of three longwords, written out of order. After
C each statement the associated variable is the next record's number.
C
      OPEN (1, FILE='D1.DAT', STATUS='NEW', ACCESS='DIRECT', RECL=3,
     1 ASSOCIATEVARIABLE=NR)
      WRITE (1'3) 30, 31, 32
      WRITE (6, '(1X,A,I3)') 'AFTER 3', NR
      WRITE (1, REC=1) 10, 11, 12
      READ (1'3) KEY
      WRITE (6, '(1X,3I4,I3)') KEY, NR
      READ (1'2) KEY
      WRITE (6, '(1X,3I4)') KEY
C
C A REAL record number is converted, and the associated variable walks
C the file.
C
      X = 1.0
      READ (1, REC=X) I
      WRITE (6, '(1X,I4,I3)') I, NR
      DO 10 J = 1, 2
      READ (1'NR) I
   10 WRITE (6, '(1X,I4,I3)') I, NR
C
C Past the end; a record too long, written as far as it goes; record 0;
C and a sequential READ of a direct unit.
C
      READ (1'4, IOSTAT=IOS) I
      WRITE (6, '(1X,A,I4)') 'PAST THE END', IOS
      WRITE (1'4, IOSTAT=IOS) 40, 41, 42, 43
      READ (1'4) KEY
      WRITE (6, '(1X,A,I3,3I4)') 'TOO LONG', IOS, KEY
      READ (1'0, ERR=20) I
      STOP 'NO ERROR'
   20 WRITE (6, '(1X,A)') 'RECORD 0'
      READ (1, IOSTAT=IOS) I
      WRITE (6, '(1X,A,I4)') 'SEQUENTIAL', IOS
C
C FIND makes its record the associated variable's value, and NEXTREC.
C
      FIND (1'2)
      INQUIRE (1, NEXTREC=N)
      WRITE (6, '(1X,A,2I3)') 'FIND', NR, N
      CLOSE (1, DISP='DELETE')
C
C Formatted records of 8 characters: a format of two records writes two,
C and a value longer than the record writes nothing.
C
      OPEN (2, FILE='D2.DAT', STATUS='NEW', ACCESS='DIRECT', RECL=8,
     1 FORM='FORMATTED')
      WRITE (2, '(A/A)', REC=2) 'TWO', 'THREE'
      READ (2, '(A)', REC=3) S
      READ (2, '(A)', REC=2) T
      WRITE (6, '(1X,A,''|'',A,''|'')') S, T
      WRITE (2, '(A)', REC=1, IOSTAT=IOS) 'TOOLONGXX'
      READ (2, '(A)', REC=4, IOSTAT=J) S
      WRITE (6, '(1X,2I4)') IOS, J
      CLOSE (2, DISP='DELETE')
C
C DEFINE FILE: five records of two words, FOR003.DAT, made by the first
C WRITE. The associated variable, a word, is the record number of each.
C
      DEFINE FILE 3(5, 2, U, NR2)
      INQUIRE (3, OPENED=OPENED)
      WRITE (6, '(1X,A,L2)') 'DEFINED, OPENED', OPENED
      NR2 = 1
      DO 30 J = 1, 5
   30 WRITE (3'NR2) J*J
      WRITE (6, '(1X,A,I3)') 'NEXT', NR2
      WRITE (3'6, IOSTAT=IOS) 36
      WRITE (6, '(1X,A,I4)') 'PAST 5', IOS
      INQUIRE (3, OPENED=OPENED, RECL=N, NEXTREC=K)
      WRITE (6, '(1X,L2,2I3)') OPENED, N, K
      CLOSE (3)
C
C Described again, the file is found by a READ.
C
      DEFINE FILE 3(5, 2, U, NR2)
      READ (3'4) I
      WRITE (6, '(1X,2I4)') I, NR2
      CLOSE (3, DISP='DELETE')
      END
