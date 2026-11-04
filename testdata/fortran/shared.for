C SHARED - two units on one indexed file (OPEN's SHARED, 9.1.25). Each unit has
C its own place in the file and its own key of reference. A READ locks the
C record it reaches until the unit's next statement, so a second unit's READ of
C it fails (FOR$_SPERECLOC, 52) until UNLOCK (9.8) or the next READ lets it go.
C What one unit writes, the other sees at once. A file opened without SHARED
C can't be opened again.
      PROGRAM SHARED
      INTEGER IOS
      INTEGER*2 ROOM, R5, R14, R21, R30, NONE
      DATA R5 /5/, R14 /14/, R21 /21/, R30 /30/, NONE /0/
      CHARACTER*8 WHO
      OPEN (1, FILE='SHARED.IDX', STATUS='NEW', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='UNFORMATTED', RECORDTYPE='FIXED', RECL=3,
     2 KEY=(1:8, 9:10:INTEGER), SHARED)
      WRITE (1) 'NOVAK   ', R21, NONE
      WRITE (1) 'HALL    ', R14, NONE
      WRITE (1) 'ORTIZ   ', R21, NONE
      WRITE (1) 'CHEN    ', R30, NONE
      OPEN (2, FILE='SHARED.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='UNFORMATTED', SHARED)
C
C Unit 1 holds HALL, then lets it go by reading another.
C
      READ (1, KEY='HALL') WHO, ROOM
      WRITE (6, 91) 1, WHO, ROOM
      READ (2, KEY='HALL', IOSTAT=IOS) WHO, ROOM
      WRITE (6, 90) 'HELD', IOS
      READ (2, KEY='NOVAK') WHO, ROOM
      WRITE (6, 91) 2, WHO, ROOM
      READ (1, KEY='NOVAK', IOSTAT=IOS) WHO
      WRITE (6, 90) 'HELD BY 2', IOS
      READ (2, KEY='HALL') WHO, ROOM
      WRITE (6, 91) 2, WHO, ROOM
      UNLOCK (2)
C
C Each unit's own key of reference and place in it: 1 by name, 2 by room.
C
      READ (1, KEY='CHEN') WHO, ROOM
      WRITE (6, 91) 1, WHO, ROOM
      UNLOCK (1)
      READ (2, KEY=R21, KEYID=1) WHO, ROOM
      WRITE (6, 91) 2, WHO, ROOM
      UNLOCK (2)
      READ (1) WHO, ROOM
      WRITE (6, 91) 1, WHO, ROOM
      UNLOCK (1)
      READ (2) WHO, ROOM
      WRITE (6, 91) 2, WHO, ROOM
      UNLOCK (2)
C
C Unit 2 moves a record and adds one, which unit 1 sees at once, by a key it
C reads and in the order it's reading.
C
      READ (2, KEY='ORTIZ', KEYID=0) WHO, ROOM
      REWRITE (2) WHO, R5, NONE
      WRITE (2) 'IBARRA  ', R21, NONE
      READ (1, KEY=R5, KEYID=1) WHO, ROOM
      WRITE (6, 91) 1, WHO, ROOM
      READ (1) WHO, ROOM
      WRITE (6, 91) 1, WHO, ROOM
      READ (1) WHO, ROOM
      WRITE (6, 91) 1, WHO, ROOM
      READ (1) WHO, ROOM
      WRITE (6, 91) 1, WHO, ROOM
      UNLOCK (1)
C
C Unit 2 takes a record out from in front of unit 1, whose READ goes on past it.
C
      READ (1, KEY='CHEN', KEYID=0) WHO, ROOM
      UNLOCK (1)
      READ (2, KEY='HALL', KEYID=0) WHO, ROOM
      DELETE (2)
      READ (1) WHO, ROOM
      WRITE (6, 91) 1, WHO, ROOM
      UNLOCK (1)
      READ (2, KEY='HALL', IOSTAT=IOS) WHO
      WRITE (6, 90) 'DELETED', IOS
C
C A READONLY unit beside them.
C
      CLOSE (2)
      OPEN (3, FILE='SHARED.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='UNFORMATTED', READONLY, SHARED)
      READ (3, KEY='CHEN') WHO, ROOM
      WRITE (6, 91) 3, WHO, ROOM
      REWRITE (3, IOSTAT=IOS) WHO, R5, NONE
      WRITE (6, 90) 'READONLY', IOS
      CLOSE (3)
      CLOSE (1)
C
C And not shared.
C
      OPEN (1, FILE='SHARED.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='UNFORMATTED')
      OPEN (2, FILE='SHARED.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='UNFORMATTED', IOSTAT=IOS)
      WRITE (6, 90) 'NOT SHARED', IOS
      CLOSE (1, DISPOSE='DELETE')
   90 FORMAT (1X, A, I4)
   91 FORMAT (1X, I1, 1X, A, I3)
      END
