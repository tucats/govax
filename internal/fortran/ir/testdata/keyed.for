C     An indexed file's statements: OPEN's KEY= block, filled in by the
C     statement, and each specifier's comparison in a keyed READ.
      PROGRAM KEYED
      CHARACTER*6 S
      BYTE B(4)
      INTEGER*2 K2
      N = 3
      OPEN (UNIT=2, FILE='K.DAT', ORGANIZATION='INDEXED',
     1      ACCESS='KEYED', RECL=40, FORM='FORMATTED',
     2      KEY=(1:10, 11:14:INTEGER:DESCENDING, N:N+5:CHARACTER))
      READ (2, KEY='AB') X
      READ (2, KEYEQ=S, KEYID=2) X
      READ (2, KEYGE=K2) X
      READ (2, KEYGT=N+1, KEYID=N) X
      READ (2, KEYLE=B) X
      READ (2, '(F5.1)', KEYLT=7, ERR=10) X
      READ (2, KEYNXT=S(1:2), IOSTAT=I) X
      READ (2, KEYNXTNE=S) X
      REWRITE (2) X
      DELETE (2)
      UNLOCK (2)
   10 END
