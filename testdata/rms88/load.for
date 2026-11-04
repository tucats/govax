C LOAD - an indexed file big enough to have every part of one: thousands of
C statements on two files, whose keys come in no order, so buckets split, index
C levels grow, and records move and leave pointers behind them. After each
C stage every key is walked from its start. Its output is what a plain model of
C the same statements says (make_load_out.py), never a run of this program.
      PROGRAM LOAD
      CALL FIXED
      CALL VARIED
      END
C
C FIXED - 2000 records of 100 bytes: an ascending integer, the primary key; a
C descending integer of 17 values, so duplicates by the hundred; and text. The
C primary key's values are the multiples of a prime, modulo a larger one. The fill
C has no runs in it, so that compression leaves each record its size.
C
      SUBROUTINE FIXED
      PARAMETER (N=2000, MODULUS=10007, MULT=7919, NGROUP=17)
      INTEGER K, G, I, NUM, BAD, MISS, MOVED
      CHARACTER*8 TAG
      CHARACTER*80 FILL
      DO I = 1, 80
         FILL(I:I) = CHAR(65 + MOD(I, 26))
      END DO
      OPEN (1, FILE='LOAD.IDX', STATUS='NEW', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='UNFORMATTED', RECORDTYPE='FIXED', RECL=25,
     2 KEY=(1:4:INTEGER, 5:8:INTEGER:DESCENDING, 13:20:CHARACTER))
      DO I = 1, N
         K = MOD(I * MULT, MODULUS)
         WRITE (TAG, '(A1,I7.7)') 'K', K
         WRITE (1) K, MOD(K, NGROUP), I, TAG, FILL
      END DO
      CALL WALK('LOADED')
C
C Each tenth record by its number, each fiftieth by its text, and numbers that
C aren't there.
C
      BAD = 0
      NUM = 0
      DO I = 10, N, 10
         READ (1, KEY=MOD(I * MULT, MODULUS), KEYID=0) K, G, NUM
         IF (NUM .NE. I) BAD = BAD + 1
      END DO
      DO I = 50, N, 50
         WRITE (TAG, '(A1,I7.7)') 'K', MOD(I * MULT, MODULUS)
         READ (1, KEY=TAG, KEYID=2) K, G, NUM
         IF (NUM .NE. I) BAD = BAD + 1
      END DO
      MISS = 0
      DO I = 1, 100
         READ (1, KEY=MODULUS + I, KEYID=0, IOSTAT=NUM) K
         IF (NUM .EQ. 36) MISS = MISS + 1
      END DO
      WRITE (6, '(1X,A,I4,A,I4)') 'FOUND WRONG', BAD, ' MISSING', MISS
C
C A third of them go.
C
      DO I = 3, N, 3
         READ (1, KEY=MOD(I * MULT, MODULUS), KEYID=0) K
         DELETE (1)
      END DO
      MISS = 0
      DO I = 3, N, 3
         READ (1, KEY=MOD(I * MULT, MODULUS), IOSTAT=NUM) K
         IF (NUM .EQ. 36) MISS = MISS + 1
      END DO
      WRITE (6, '(1X,A,I4)') 'GONE', MISS
      CALL WALK('DELETED')
C
C Each fifth left moves to the next group, a change in one alternate key.
C
      MOVED = 0
      DO I = 1, N, 5
         IF (MOD(I, 3) .NE. 0) THEN
            READ (1, KEY=MOD(I * MULT, MODULUS), KEYID=0) K, G, NUM, TAG
            REWRITE (1) K, MOD(G + 1, NGROUP), NUM, TAG, FILL
            MOVED = MOVED + 1
         END IF
      END DO
      WRITE (6, '(1X,A,I4)') 'MOVED', MOVED
      CALL WALK('MOVED')
C
C Those that went come back, numbered after all the others.
C
      DO I = 3, N, 3
         K = MOD(I * MULT, MODULUS)
         WRITE (TAG, '(A1,I7.7)') 'K', K
         WRITE (1) K, MOD(K, NGROUP), N + I, TAG, FILL
      END DO
      CALL WALK('BACK')
      CLOSE (1)
      END
C
C WALK - every key of unit 1 from its start: how many records, the sum of the
C primary key, each group's size, and how often a key's order isn't kept.
C Records of one group stay in the order they joined it, which NUM shows until
C the first moves; WRONG(3) counts where it goes down.
C
      SUBROUTINE WALK(TITLE)
      CHARACTER*(*) TITLE
      PARAMETER (NGROUP=17)
      INTEGER K, G, NUM, LAST, LASTG, LASTN, SUM, N(3), WRONG(3)
      INTEGER GROUP(0:NGROUP-1), L
      CHARACTER*8 TAG, LTAG
      N(1) = 0
      N(2) = 0
      N(3) = 0
      WRONG(1) = 0
      WRONG(2) = 0
      WRONG(3) = 0
      SUM = 0
      LAST = -1
      READ (1, KEYGE=0, KEYID=0) K, G, NUM, TAG
   10 N(1) = N(1) + 1
      SUM = SUM + K
      IF (K .LE. LAST) WRONG(1) = WRONG(1) + 1
      LAST = K
      READ (1, END=20) K, G, NUM, TAG
      GO TO 10
   20 LTAG = ' '
      READ (1, KEYGE=' ', KEYID=2) K, G, NUM, TAG
   30 N(2) = N(2) + 1
      IF (TAG .LE. LTAG) WRONG(2) = WRONG(2) + 1
      LTAG = TAG
      READ (1, END=40) K, G, NUM, TAG
      GO TO 30
   40 DO L = 0, NGROUP - 1
         GROUP(L) = 0
      END DO
      LASTG = NGROUP
      LASTN = 0
      READ (1, KEYLE=NGROUP, KEYID=1) K, G, NUM
   50 N(3) = N(3) + 1
      GROUP(G) = GROUP(G) + 1
      IF (G .GT. LASTG) WRONG(3) = WRONG(3) + 1
      IF (G .EQ. LASTG .AND. NUM .LT. LASTN) WRONG(3) = WRONG(3) + 1
      LASTG = G
      LASTN = NUM
      READ (1, END=60) K, G, NUM
      GO TO 50
   60 UNLOCK (1)
      WRITE (6, '(1X,A,3I6,I9,3I5)') TITLE, N, SUM, WRONG
      WRITE (6, '(1X,17I4)') GROUP
      END
C
C VARIED - 500 records of 12 to 161 bytes, in a file of variable records, a
C second key past the end of the shorter ones, which aren't in its index. Each
C fourth grows to 190 bytes, which moves it, and each seventh is cut to 10, which
C takes it out of the second key. Q editing shows each record's length.
C
      SUBROUTINE VARIED
      PARAMETER (N=500, MODULUS=10007, MULT=7919)
      CHARACTER*200 LINE
      INTEGER I, J, K, L
      DO J = 1, 200
         LINE(J:J) = '.'
      END DO
      OPEN (2, FILE='LOADV.IDX', STATUS='NEW', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='FORMATTED', RECORDTYPE='VARIABLE',
     2 RECL=200, KEY=(1:8, 41:44))
      DO I = 1, N
         K = MOD(I * MULT, MODULUS)
         CALL FILLIN(LINE, K)
         L = 12 + MOD(K, 150)
         WRITE (2, '(A)') LINE(1:L)
      END DO
      CALL WALKV('LOADED')
      DO I = 4, N, 4
         K = MOD(I * MULT, MODULUS)
         WRITE (LINE, '(A1,I7.7)') 'K', K
         READ (2, '(A)', KEY=LINE(1:8), KEYID=0) LINE
         CALL FILLIN(LINE, K)
         REWRITE (2, '(A)') LINE(1:190)
      END DO
      CALL WALKV('LONGER')
      DO I = 7, N, 7
         K = MOD(I * MULT, MODULUS)
         WRITE (LINE, '(A1,I7.7)') 'K', K
         READ (2, '(A)', KEY=LINE(1:8), KEYID=0) LINE
         REWRITE (2, '(A)') LINE(1:10)
      END DO
      CALL WALKV('SHORTER')
      CLOSE (2)
      END
C
C FILLIN - a record's whole text: its number's text at 1, dots, and at 41 a code
C of its number, modulo 97.
C
      SUBROUTINE FILLIN(LINE, K)
      CHARACTER*200 LINE
      INTEGER J, K
      DO J = 9, 200
         LINE(J:J) = '.'
      END DO
      WRITE (LINE(1:8), '(A1,I7.7)') 'K', K
      WRITE (LINE(41:44), '(I4.4)') MOD(K, 97)
      END
C
C WALKV - both of unit 2's keys from their starts: how many records and the sum of
C their lengths.
C
      SUBROUTINE WALKV(TITLE)
      CHARACTER*(*) TITLE
      CHARACTER*200 LINE
      CHARACTER*8 LAST
      CHARACTER*4 LCODE
      INTEGER L, N, SUM, N2, SUM2, WRONG
      N = 0
      SUM = 0
      WRONG = 0
      LAST = ' '
      READ (2, '(Q,A)', KEYGE=' ', KEYID=0) L, LINE
   10 N = N + 1
      SUM = SUM + L
      IF (LINE(1:8) .LE. LAST) WRONG = WRONG + 1
      LAST = LINE(1:8)
      READ (2, '(Q,A)', END=20) L, LINE
      GO TO 10
   20 N2 = 0
      SUM2 = 0
      LCODE = ' '
      READ (2, '(Q,A)', KEYGE=' ', KEYID=1) L, LINE
   30 N2 = N2 + 1
      SUM2 = SUM2 + L
      IF (LINE(41:44) .LT. LCODE) WRONG = WRONG + 1
      LCODE = LINE(41:44)
      READ (2, '(Q,A)', END=40) L, LINE
      GO TO 30
   40 UNLOCK (2)
      WRITE (6, '(1X,A,2I7,2I7,I4)') TITLE, N, SUM, N2, SUM2, WRONG
      END
