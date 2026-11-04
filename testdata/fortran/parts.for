C PARTS - an indexed file (ORGANIZATION='INDEXED', section 9.1.15): an
C inventory by part number, by name, and by quantity on hand. The part number
C is the primary key, an ascending integer; the name an alternate character
C key, which several parts can share; the quantity a descending integer key.
C Each key's order, the keyed READ's specifiers (7.1.1.7), READ in sequence
C after a keyed READ (7.2.3), WRITE, REWRITE, and DELETE, the numbers each
C failure's IOSTAT gives, and the file again by another OPEN.
      PROGRAM PARTS
      INTEGER NUM, QTY, IOS
      CHARACTER*6 NAME
      CHARACTER*2 BIN
      OPEN (1, FILE='PARTS.IDX', STATUS='NEW', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='UNFORMATTED', RECORDTYPE='FIXED', RECL=4,
     2 KEY=(1:4:INTEGER, 5:10:CHARACTER, 11:14:INTEGER:DESCENDING))
C
C Written in no order: each record goes where its keys say. Two parts can't
C have one number, but three can have one name and two one quantity.
C
      WRITE (1) 1042, 'WIDGET', 120, 'A1'
      WRITE (1) 1007, 'BOLT  ', 500, 'A2'
      WRITE (1) 1099, 'NUT   ', 300, 'B1'
      WRITE (1) 1015, 'BOLT  ', 75, 'B2'
      WRITE (1) 1060, 'WASHER', 300, 'C1'
      WRITE (1) 1023, 'GEAR  ', 40, 'C2'
      WRITE (1) 1081, 'BOLT  ', 220, 'A3'
      WRITE (1, IOSTAT=IOS) 1007, 'CLAMP ', 10, 'D1'
      WRITE (6, 90) 'SAME NUMBER', IOS
C
C By number: the first at or past 0, then in sequence to the end.
C
      WRITE (6, 91) 'BY NUMBER'
      READ (1, KEYGE=0, KEYID=0) NUM, NAME, QTY, BIN
   10 WRITE (6, 92) NUM, NAME, QTY, BIN
      READ (1, END=11) NUM, NAME, QTY, BIN
      GO TO 10
C
C By name: parts of one name in the order they were written, and a READ with
C part of a name, which finds the first it begins.
C
   11 WRITE (6, 91) 'BY NAME'
      READ (1, KEY='BOLT  ', KEYID=1) NUM, NAME, QTY, BIN
   12 WRITE (6, 92) NUM, NAME, QTY, BIN
      READ (1, END=13) NUM, NAME, QTY, BIN
      GO TO 12
   13 READ (1, KEY='X', KEYID=1, IOSTAT=IOS) NUM
      WRITE (6, 90) 'NO SUCH NAME', IOS
      READ (1, KEYGE='W', KEYID=1) NUM, NAME
      WRITE (6, 93) NUM, NAME
      READ (1, KEYGT='BOLT') NUM, NAME
      WRITE (6, 93) NUM, NAME
      READ (1, KEY='WI', KEYID=1) NUM, NAME
      WRITE (6, 93) NUM, NAME
      READ (1, KEY='WASHERS', IOSTAT=IOS) NUM
      WRITE (6, 90) 'NAME TOO LONG', IOS
C
C By quantity, largest first: KEYLE and KEYLT go on from a number down, KEYNXT
C and KEYNXTNE take the key's own direction, and KEYGE and KEYGT are refused.
C
      WRITE (6, 91) 'BY QUANTITY'
      READ (1, KEYLE=300, KEYID=2) NUM, NAME, QTY
      WRITE (6, 94) NUM, QTY
      READ (1) NUM, NAME, QTY
      WRITE (6, 94) NUM, QTY
      READ (1) NUM, NAME, QTY
      WRITE (6, 94) NUM, QTY
      READ (1, KEYLT=300) NUM, NAME, QTY
      WRITE (6, 94) NUM, QTY
      READ (1, KEYNXT=100) NUM, NAME, QTY
      WRITE (6, 94) NUM, QTY
      READ (1, KEYNXTNE=75) NUM, NAME, QTY
      WRITE (6, 94) NUM, QTY
      READ (1, KEYGE=100, IOSTAT=IOS) NUM
      WRITE (6, 90) 'KEYGE', IOS
      READ (1, KEYGT=100, IOSTAT=IOS) NUM
      WRITE (6, 90) 'KEYGT', IOS
      READ (1, KEYLT=40, IOSTAT=IOS) NUM
      WRITE (6, 90) 'NOTHING BELOW', IOS
C
C On the number's key the other way, KEYLE isn't allowed.
C
      READ (1, KEYLE=1042, KEYID=0, IOSTAT=IOS) NUM
      WRITE (6, 90) 'KEYLE', IOS
C
C A shipment: REWRITE changes the quantity, so the record moves in that key's
C order. A new name moves it in that key's, after those already there. A new
C number can't be.
C
      READ (1, KEY=1015, KEYID=0) NUM, NAME, QTY, BIN
      REWRITE (1) NUM, NAME, QTY + 525, BIN
      READ (1, KEY=1060) NUM, NAME, QTY, BIN
      REWRITE (1) NUM, 'BOLT  ', QTY, BIN
      READ (1, KEY=1099) NUM, NAME, QTY, BIN
      REWRITE (1, IOSTAT=IOS) 2099, NAME, QTY, BIN
      WRITE (6, 90) 'NEW NUMBER', IOS
      UNLOCK (1)
      WRITE (6, 91) 'CHANGED'
      READ (1, KEYLE=99999, KEYID=2) NUM, NAME, QTY, BIN
   20 WRITE (6, 92) NUM, NAME, QTY, BIN
      READ (1, END=21) NUM, NAME, QTY, BIN
      GO TO 20
   21 READ (1, KEY='BOLT', KEYID=1) NUM, NAME, QTY, BIN
   22 IF (NAME .NE. 'BOLT  ') GO TO 23
      WRITE (6, 92) NUM, NAME, QTY, BIN
      READ (1) NUM, NAME, QTY, BIN
      GO TO 22
C
C Dropped: a DELETE takes the record the last READ reached, and leaves none. A
C sequential READ goes on from where it was, and the number can be used again.
C
   23 READ (1, KEY=1042, KEYID=0) NUM
      DELETE (1)
      DELETE (1, IOSTAT=IOS)
      WRITE (6, 90) 'NO RECORD', IOS
      READ (1, KEY=1042, IOSTAT=IOS) NUM
      WRITE (6, 90) 'GONE', IOS
      READ (1, KEY=1023) NUM
      DELETE (1)
      READ (1) NUM, NAME
      WRITE (6, 93) NUM, NAME
      UNLOCK (1)
      WRITE (1) 1042, 'SPROCK', 15, 'E1'
      READ (1, KEY=1042) NUM, NAME, QTY, BIN
      WRITE (6, 92) NUM, NAME, QTY, BIN
      UNLOCK (1)
      CLOSE (1)
C
C Again, the keys given only in part, and in sequence from the start: the
C primary key's order, then in another OPEN without any KEY= at all. A file's
C keys have to be the ones it was made with.
C
      OPEN (1, FILE='PARTS.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='SEQUENTIAL', FORM='UNFORMATTED', KEY=(1:4:INTEGER),
     2 READONLY)
      WRITE (6, 91) 'AGAIN'
   30 READ (1, END=31) NUM, NAME, QTY, BIN
      WRITE (6, 92) NUM, NAME, QTY, BIN
      GO TO 30
   31 CLOSE (1)
      OPEN (1, FILE='PARTS.IDX', STATUS='OLD', ACCESS='KEYED',
     1 FORM='UNFORMATTED')
      READ (1, KEY='BOLT', KEYID=1) NUM
      WRITE (6, 94) NUM
      CLOSE (1)
      OPEN (1, FILE='PARTS.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='UNFORMATTED', KEY=(1:4:INTEGER:DESCENDING),
     2 IOSTAT=IOS)
      WRITE (6, 90) 'OTHER KEYS', IOS
      CALL SIZES
      OPEN (1, FILE='PARTS.IDX', STATUS='OLD', DISPOSE='DELETE')
      CLOSE (1)
      STOP
   90 FORMAT (1X, A, I4)
   91 FORMAT (1X, '-- ', A)
   92 FORMAT (1X, I4, 1X, A, I4, 1X, A)
   93 FORMAT (1X, I4, 1X, A)
   94 FORMAT (1X, I4, I4)
      END
C
C SIZES - formatted records, a city's name a descending character key and its
C code an ascending one: part of a key in its own direction, each READ's place
C for the READ in sequence after it, and a WRITE's blanks filling a FIXED record.
C
      SUBROUTINE SIZES
      CHARACTER*8 CITY
      CHARACTER*4 ZIP
      CHARACTER*12 LINE
      INTEGER IOS
      OPEN (2, FILE='SIZES.IDX', STATUS='NEW', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='FORMATTED', RECL=12,
     2 KEY=(1:8:CHARACTER:DESCENDING, 9:12))
      WRITE (2, '(A8,A4)') 'ALBANY  ', '1220'
      WRITE (2, '(A8,A4)') 'BOSTON  ', '0210'
      WRITE (2, '(A8,A4)') 'CHICAGO ', '6060'
      WRITE (2, '(A8)') 'DENVER  '
      READ (2, '(A8,A4)', KEYLE='BOSTON') CITY, ZIP
      WRITE (6, '(1X,A,''|'',A,''|'')') CITY, ZIP
      READ (2, '(A8,A4)', KEYLT='BOSTON') CITY, ZIP
      WRITE (6, '(1X,A,''|'',A,''|'')') CITY, ZIP
      READ (2, '(A8,A4)', KEYNXT='C') CITY, ZIP
      WRITE (6, '(1X,A,''|'',A,''|'')') CITY, ZIP
      READ (2, '(A8,A4)', KEYNXTNE='CHICAGO ') CITY, ZIP
      WRITE (6, '(1X,A,''|'',A,''|'')') CITY, ZIP
      READ (2, '(A8,A4)') CITY, ZIP
      WRITE (6, '(1X,A,''|'',A,''|'')') CITY, ZIP
      READ (2, '(A8,A4)', END=10) CITY, ZIP
      WRITE (6, '(1X,A)') 'NOT THE END'
   10 READ (2, '(A12)', KEY='1220', KEYID=1) LINE
      WRITE (6, '(1X,A)') LINE
      READ (2, '(A12)', KEYGE='3', IOSTAT=IOS) LINE
      WRITE (6, '(1X,A)') LINE
      READ (2, '(A12)', KEY='    ', IOSTAT=IOS) LINE
      WRITE (6, '(1X,A,I4)') LINE, IOS
      READ (2, '(A12)') LINE
      WRITE (6, '(1X,A)') LINE
      CLOSE (2, DISPOSE='DELETE')
      END
