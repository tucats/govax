C RELATIVE - relative files (ORGANIZATION='RELATIVE', section 9.1.19), and
C the statements on their records: REWRITE (7.4), DELETE (9.7), and UNLOCK
C (9.8). A file of parts by number, written out of order and read back by
C number and in sequence, where the cells with no record are passed over;
C records rewritten and deleted; and each failure's IOSTAT the number VMS's
C FORTRAN run-time library gave in testdata/for81's runs.
      PROGRAM RELATIVE
      INTEGER I, N, IOS, NR
      CHARACTER*12 PART
      CHARACTER*8 ORG
C
C Formatted records of 12 characters, by number; a direct WRITE over a
C record replaces it, and a record never written isn't there.
C
      OPEN (1, FILE='PARTS.DAT', STATUS='NEW', ORGANIZATION='RELATIVE',
     1 ACCESS='DIRECT', RECL=12, FORM='FORMATTED', MAXREC=9,
     2 ASSOCIATEVARIABLE=NR)
      WRITE (1'5, '(A)') 'BOLT'
      WRITE (1'2, '(A)') 'NUT'
      WRITE (1'7, '(A)') 'WASHER'
      WRITE (1'5, '(A)') 'HEX BOLT'
      READ (1'5, '(A)') PART
      WRITE (6, '(1X,A,''|'',I3)') PART, NR
      READ (1'3, '(A)', IOSTAT=IOS) PART
      WRITE (6, '(1X,A,I3)') 'MISSING', IOS
      READ (1'10, '(A)', IOSTAT=IOS) PART
      WRITE (6, '(1X,A,I3)') 'PAST MAXREC', IOS
C
C REWRITE the record a READ reached; DELETE one by number, after which
C the associated variable is the next record's.
C
      READ (1'2, '(A)') PART
      REWRITE (1, '(A)') 'LOCK NUT'
      READ (1'2, '(A)') PART
      WRITE (6, '(1X,A)') PART
      DELETE (1'7)
      WRITE (6, '(1X,A,I3)') 'DELETED, NEXT', NR
      READ (1'7, '(A)', IOSTAT=IOS) PART
      DELETE (1, REC=7, IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'DELETED TWICE', IOS
C
C A REWRITE after a WRITE has no current record.
C
      WRITE (1'4, '(A)') 'SPRING'
      REWRITE (1, '(A)', IOSTAT=IOS) 'CLIP'
      WRITE (6, '(1X,A,I3)') 'NO CURRENT', IOS
      CLOSE (1)
C
C The same file in sequence: a READ passes over the cells with no
C record. DELETE takes the record the last READ reached.
C
      OPEN (1, FILE='PARTS.DAT', STATUS='OLD', ORGANIZATION='RELATIVE')
      N = 0
   10 READ (1, '(A)', END=20) PART
      N = N + 1
      WRITE (6, '(1X,I1,1X,A)') N, PART
      IF (PART .EQ. 'SPRING') DELETE (1)
      UNLOCK (1)
      GO TO 10
   20 REWIND (1, IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'REWIND', IOS
      CLOSE (1)
      OPEN (1, FILE='PARTS.DAT', STATUS='OLD', ORGANIZATION='RELATIVE')
      N = 0
   30 READ (1, '(A)', END=40) PART
      N = N + 1
      GO TO 30
   40 WRITE (6, '(1X,A,I2)') 'RECORDS', N
      CLOSE (1, DISPOSE='DELETE')
C
C On a sequential file: no DELETE, a REWRITE of the record's own length
C only, and no keyed READ.
C
      OPEN (2, FILE='SEQ.DAT', STATUS='NEW')
      WRITE (2, '(A)') 'ABCD'
      REWIND 2
      READ (2, '(A)') PART
      REWRITE (2, '(A)') 'WXYZ'
      REWIND 2
      READ (2, '(A)') PART
      WRITE (6, '(1X,A)') PART
      REWRITE (2, '(A)', IOSTAT=IOS) 'TOO LONG'
      DELETE (2, IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'DELETE', IOS
      READ (2, KEY='WX', IOSTAT=IOS) PART
      WRITE (6, '(1X,A,I3)') 'KEYED', IOS
      CLOSE (2, DISPOSE='DELETE')
C
C An indexed file isn't here yet.
C
      ORG = 'INDEXED'
      OPEN (3, FILE='IDX.DAT', STATUS='NEW', ORGANIZATION=ORG, RECL=8,
     1 IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'INDEXED', IOS
      END
