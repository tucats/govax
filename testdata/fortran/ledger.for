C LEDGER - Phase 81's file statements together: a day's transactions
C written to a sequential file and read back; accounts kept in a relative
C file by number, each transaction a READ and a REWRITE, a new account a
C WRITE, a closed one a DELETE; the accounts read in sequence into a
C report file of LIST carriage control, and a checkpoint of the totals in
C a segmented unformatted scratch file; INQUIRE of what is open.
      PROGRAM LEDGER
      INTEGER ACCT, AMT, BAL, IOS, N, NACCT, TOTAL, I
      INTEGER SAVED(3), BACK(3)
      CHARACTER*10 OWNER, NAME
      CHARACTER*1 KIND
      CHARACTER*40 LINE
      CHARACTER*12 ORG, ACC
      LOGICAL OPENED
C
C The day's transactions: O opens an account, D deposits, W withdraws, and
C C closes it.
C
      OPEN (1, FILE='TRANS.DAT', STATUS='NEW')
      WRITE (1, '(A1,I3,I6,1X,A10)') 'O', 7, 100, 'ADAMS',
     1 'O', 3, 250, 'BAKER', 'O', 12, 40, 'CLARK',
     2 'D', 7, 25, ' ', 'W', 3, 50, ' ', 'O', 5, 0, 'DAVIS',
     3 'C', 12, 0, ' ', 'W', 7, 500, ' ', 'D', 9, 10, ' '
      CLOSE (1)
C
C The accounts: a relative file of fixed-length records by number.
C
      OPEN (2, FILE='ACCTS.DAT', STATUS='NEW', ORGANIZATION='RELATIVE',
     1 ACCESS='DIRECT', RECL=20, FORM='FORMATTED', MAXREC=20)
      OPEN (1, FILE='TRANS.DAT', STATUS='OLD')
      N = 0
   10 READ (1, '(A1,I3,I6,1X,A10)', END=30) KIND, ACCT, AMT, NAME
      N = N + 1
      IF (KIND .EQ. 'O') THEN
         WRITE (2'ACCT, '(A10,I10)') NAME, AMT
         GO TO 10
      END IF
      READ (2'ACCT, '(A10,I10)', IOSTAT=IOS) OWNER, BAL
      IF (IOS .NE. 0) THEN
         WRITE (6, '(1X,A,I3,A,I3)') 'NO ACCOUNT', ACCT, ', IOSTAT', IOS
      ELSE IF (KIND .EQ. 'C') THEN
         DELETE (2'ACCT)
      ELSE IF (KIND .EQ. 'D') THEN
         REWRITE (2, '(A10,I10)') OWNER, BAL + AMT
      ELSE IF (AMT .GT. BAL) THEN
         WRITE (6, '(1X,A,1X,A)') 'REFUSED', OWNER
         UNLOCK (2)
      ELSE
         REWRITE (2, '(A10,I10)') OWNER, BAL - AMT
      END IF
      GO TO 10
   30 CLOSE (1, DISPOSE='DELETE')
      WRITE (6, '(1X,I2,A)') N, ' TRANSACTIONS'
      INQUIRE (2, ORGANIZATION=ORG, ACCESS=ACC, OPENED=OPENED)
      WRITE (6, '(1X,A,1X,A,L2)') ORG, ACC, OPENED
      CLOSE (2)
C
C The report: the accounts in sequence, the empty cells passed over.
C
      OPEN (2, FILE='ACCTS.DAT', STATUS='OLD', ORGANIZATION='RELATIVE')
      OPEN (3, FILE='REPORT.LIS', STATUS='NEW', CARRIAGECONTROL='LIST')
      NACCT = 0
      TOTAL = 0
   40 READ (2, '(A10,I10)', END=50) OWNER, BAL
      NACCT = NACCT + 1
      TOTAL = TOTAL + BAL
      WRITE (3, '(A10,I6)') OWNER, BAL
      GO TO 40
   50 WRITE (3, '(A10,I6)') 'TOTAL', TOTAL
      CLOSE (2, DISPOSE='DELETE')
      REWIND 3
   60 READ (3, '(A)', END=70) LINE
      WRITE (6, '(1X,A)') LINE(1:16)
      GO TO 60
   70 CLOSE (3, DISPOSE='DELETE')
C
C A checkpoint: written, the unit backed over, and read again.
C
      SAVED(1) = NACCT
      SAVED(2) = TOTAL
      SAVED(3) = N
      OPEN (4, STATUS='SCRATCH', FORM='UNFORMATTED')
      WRITE (4) (I, I = 1, 600)
      WRITE (4) SAVED
      BACKSPACE 4
      READ (4) BACK
      WRITE (6, '(1X,A,3I5)') 'CHECKPOINT', BACK
      CLOSE (4)
      END
