C INQUIRE - the INQUIRE statement (section 9.3), each answer the one VMS's
C FORTRAN run-time library gave in testdata/for81's runs, or over govax where
C the phase doc says so.
      PROGRAM INQ
      LOGICAL EX, OP, NM
      LOGICAL*1 EX1
      LOGICAL*2 EX2
      INTEGER N, RL, NR, IOS
      INTEGER*2 N2
      INTEGER LIST(3)
      CHARACTER*40 FN
      CHARACTER*12 AC, SQ, DR, FM, FD, UF, BL, OG, RT, KY, CC
      CHARACTER*4 SHORT
      CHARACTER*12 WORDS(2)
      CHARACTER*12 PART
C
C A unit that can exist, and one that can't.
C
      FN = 'NOT TOUCHED'
      INQUIRE (19, EXIST=EX, OPENED=OP, NUMBER=N, NAMED=NM, NAME=FN,
     1     ACCESS=AC, RECL=RL, NEXTREC=NR, IOSTAT=IOS)
      PRINT *, EX, OP, N, NM, ' [', FN(1:4), '] ', AC, RL, NR, IOS
      INQUIRE (UNIT=120, EXIST=EX, NUMBER=N, IOSTAT=IOS, ERR=90)
      PRINT *, EX, N, IOS
      INQUIRE (-1, EXIST=EX, NUMBER=N)
      PRINT *, EX, N
C
C A file that isn't there, and names that can't be asked about. Blanks after
C a name don't count.
C
      FN = 'MISSING.DAT'
      INQUIRE (FILE=FN, EXIST=EX, NAMED=NM, NAME=FN, NUMBER=N)
      PRINT *, EX, NM, N, ' ', FN
      INQUIRE (FILE='OLD*.DAT', EXIST=EX, NAMED=NM, NAME=FN)
      PRINT *, EX, NM, ' ', FN
      INQUIRE (FILE='NOSUCH:[X]Y.DAT', EXIST=EX, NAMED=NM, NAME=FN)
      PRINT *, EX, NM, ' ', FN
      INQUIRE (FILE='A[B', EXIST=EX, NAMED=NM, NAME=FN, IOSTAT=IOS)
      PRINT *, EX, NM, ' ', FN, IOS
C
C A connected unit, by its number and by its file's name.
C
      OPEN (10, FILE='INQ1.DAT', STATUS='NEW')
      WRITE (10, '(A)') 'SOME TEXT'
      INQUIRE (10, EXIST=EX, OPENED=OP, NUMBER=N, NAMED=NM, NAME=FN,
     1     ACCESS=AC, SEQUENTIAL=SQ, DIRECT=DR, FORM=FM, FORMATTED=FD,
     2     UNFORMATTED=UF, RECL=RL, NEXTREC=NR, BLANK=BL,
     3     ORGANIZATION=OG, RECORDTYPE=RT, KEYED=KY, CARRIAGECONTROL=CC)
      PRINT *, EX, OP, N, NM, ' ', FN
      PRINT *, AC, SQ, DR
      PRINT *, FM, FD, UF
      PRINT *, RL, NR, ' ', BL, OG
      PRINT *, RT, KY, CC
      N = 99
      INQUIRE (FILE='INQ1.DAT', NUMBER=N, OPENED=OP, FORM=FM)
      PRINT *, N, OP, ' ', FM
      INQUIRE (FILE='INQ1', NUMBER=N)
      PRINT *, N
      CLOSE (10)
C
C The same file closed: the file's own attributes, and nothing of a unit.
C The default name is .DAT, or what DEFAULTFILE= says.
C
      INQUIRE (FILE='INQ1', EXIST=EX, OPENED=OP, NUMBER=N, NAMED=NM,
     1     NAME=FN, ACCESS=AC, SEQUENTIAL=SQ, DIRECT=DR, FORM=FM,
     2     FORMATTED=FD, UNFORMATTED=UF, RECL=RL, NEXTREC=NR, BLANK=BL,
     3     ORGANIZATION=OG, RECORDTYPE=RT, KEYED=KY, CARRIAGECONTROL=CC)
      PRINT *, EX, OP, N, NM, ' ', FN
      PRINT *, AC, SQ, DR
      PRINT *, FM, FD, UF
      PRINT *, RL, NR, ' ', BL, OG
      PRINT *, RT, KY, CC
      INQUIRE (FILE='INQ1', DEFAULTFILE='.XYZ', EXIST=EX, NAME=FN)
      PRINT *, EX, ' ', FN
      INQUIRE (DEFAULTFILE='INQ1.DAT', EXIST=EX)
      PRINT *, EX
      FN = 'INQ1.DAT'
      INQUIRE (19, OPENED=OP, NUMBER=N, NAME=FN)
      PRINT *, OP, N, ' [', FN(1:4), ']'
C
C Fixed-length records make a file direct, in any form.
C
      OPEN (11, FILE='INQ2.DAT', STATUS='NEW', RECORDTYPE='FIXED',
     1     RECL=20)
      CLOSE (11)
      INQUIRE (FILE='INQ2.DAT', DIRECT=DR, RECL=RL, RECORDTYPE=RT)
      PRINT *, DR, RL, ' ', RT
      OPEN (11, FILE='INQ2.DAT', STATUS='OLD', FORM='UNFORMATTED',
     1     DISP='DELETE')
      INQUIRE (11, RECL=RL, FORM=FM, BLANK=BL, RECORDTYPE=RT)
      PRINT *, RL, ' ', FM, BL, RT
      CLOSE (11)
C
C Places of every width, an array element, a substring, and a text cut short.
C
      OPEN (12, ACCESS='DIRECT', RECL=4, DISP='DELETE')
      N2 = -1
      EX1 = .FALSE.
      EX2 = .FALSE.
      LIST(2) = -1
      INQUIRE (12, RECL=N2, EXIST=EX1, OPENED=EX2, NEXTREC=LIST(2))
      PRINT *, N2, EX2, LIST(2)
      IF (EX1) PRINT *, 'A BYTE IS TRUE'
      INQUIRE (12, ACCESS=SHORT, FORM=WORDS(2), RECORDTYPE=PART(3:9))
      PRINT *, SHORT, ' ', WORDS(2), ' ', PART(3:9)
      CLOSE (12)
C
C A scratch file has no name to open it by; the unit knows what it's called.
C
      OPEN (13, STATUS='SCRATCH')
      INQUIRE (13, NAMED=NM, NAME=FN, RECL=RL)
      PRINT *, NM, ' ', FN, RL
      CLOSE (13)
C
C A default unit is connected by its first use, and until then is like any.
C
      INQUIRE (6, OPENED=OP, NAME=FN)
      PRINT *, OP, ' [', FN(1:4), ']'
      WRITE (6, '(A)') ' UNIT 6'
      INQUIRE (6, OPENED=OP, NAME=FN, NAMED=NM, NUMBER=N)
      PRINT *, OP, NM, N, ' ', FN
      OPEN (10, FILE='INQ1.DAT', STATUS='OLD', DISP='DELETE')
      STOP
   90 PRINT *, 'THE ERR= WAS TAKEN'
      END
