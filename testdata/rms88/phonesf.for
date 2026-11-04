C PHONESF - FORTRAN on the file PHONES.MAR made and PHONESC.MAR added to, whose
C keys it doesn't declare: the OPEN finds them there, and so does each READ. A
C record's fields are text, so a key is a field's place. Its keys' rules are the
C file's, whatever made it: a descending key, a key of two segments, read
C generically, and a number that can't be used twice.
      PROGRAM PHONESF
      CHARACTER*12 LAST
      CHARACTER*8 FIRST, NUM
      CHARACTER*20 NOTE
      INTEGER IOS
      OPEN (1, FILE='PHONES.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='FORMATTED')
      READ (1, 90, KEY='WALKER') LAST, FIRST, NUM, NOTE
      WRITE (6, 91) LAST, FIRST, NUM, NOTE
      READ (1, 90) LAST, FIRST, NUM, NOTE
      WRITE (6, 91) LAST, FIRST, NUM, NOTE
      READ (1, 90, KEY='212-3003', KEYID=1) LAST, FIRST, NUM, NOTE
      WRITE (6, 91) LAST, FIRST, NUM, NOTE
      READ (1, 90, KEYLE='500', KEYID=2) LAST, FIRST, NUM, NOTE
      WRITE (6, 91) LAST, FIRST, NUM, NOTE
      READ (1, 90, KEYGE='500', IOSTAT=IOS) LAST
      WRITE (6, '(1X,A,I3)') 'THE OTHER WAY', IOS
      READ (1, 90, KEYGE='A', KEYID=3) LAST, FIRST, NUM, NOTE
   10 WRITE (6, 91) LAST, FIRST, NUM, NOTE
      READ (1, 90, END=20) LAST, FIRST, NUM, NOTE
      GO TO 10
C
C A new person, and then the same person with a note, which puts the record in
C the note's index, after those it has already.
C
   20 WRITE (1, 90) 'BAKER       ', 'TOM     ', '312-7007'
      READ (1, 90, KEY='BAKER', KEYID=0) LAST, FIRST, NUM, NOTE
      WRITE (6, 91) LAST, FIRST, NUM, NOTE
      REWRITE (1, 90) LAST, FIRST, NUM, 'TENNIS'
      READ (1, 90, KEY='TENNIS', KEYID=3) LAST, FIRST, NUM, NOTE
      WRITE (6, 91) LAST, FIRST, NUM, NOTE
      READ (1, 90) LAST, FIRST, NUM, NOTE
      WRITE (6, 91) LAST, FIRST, NUM, NOTE
      UNLOCK (1)
      WRITE (1, 90, IOSTAT=IOS) 'BAKER       ', 'SAM     ',
     1 '312-7007'
      WRITE (6, '(1X,A,I3)') 'SAME NUMBER', IOS
      WRITE (1, 90, IOSTAT=IOS) 'BAKER       ', 'TOM     ',
     1 '312-7008'
      WRITE (6, '(1X,A,I3)') 'SAME NAME', IOS
      CLOSE (1)
   90 FORMAT (A12, A8, A8, A)
   91 FORMAT (1X, A12, A8, A8, '|', A, '|')
      END
