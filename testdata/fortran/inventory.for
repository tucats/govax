      PROGRAM INVENT
C     An inventory read from the terminal and reported (Phase 80, subtask
C     14): fixed-column records read by a FORMAT until END=, a record
C     with a bad number taken by ERR= and counted, a price with no
C     decimal point placed by F8.2's d, a short record padded with
C     blanks; a table with headings and tab stops, a line for each item,
C     its totals, a title built in an internal file, and the totals
C     written again as a NAMELIST group.
      INTEGER MAXN
      PARAMETER (MAXN = 20)
      CHARACTER*12 NAME(MAXN)
      CHARACTER*40 TITLE
      INTEGER QTY(MAXN), N, NBAD, I, TOTQTY
      REAL PRICE(MAXN), TOTAL
      NAMELIST /TOTALS/ N, NBAD, TOTQTY, TOTAL
   10 FORMAT (A12, I5, F8.2)
   20 FORMAT (1X, A / 1X, 'Item', T16, 'Qty', T23, 'Price', T34, 'Value'
     1        / 1X, 37('-'))
   30 FORMAT (1X, A12, I5, F9.2, F11.2)
   40 FORMAT (1X, 37('-') / 1X, 'Total', T14, I5, T28, F11.2
     1        / 1X, 'Average price', T19, F9.2)
      N = 0
      NBAD = 0
  100 READ (5, 10, END=200, ERR=150) NAME(N+1), QTY(N+1), PRICE(N+1)
      N = N + 1
      IF (N .LT. MAXN) GO TO 100
      GO TO 200
  150 NBAD = NBAD + 1
      GO TO 100
  200 WRITE (TITLE, '(I2, '' items, '', I1, '' refused'')') N, NBAD
      PRINT 20, TITLE(:INDEX(TITLE, '  ') - 1)
      TOTQTY = 0
      TOTAL = 0.0
      DO 300 I = 1, N
         PRINT 30, NAME(I), QTY(I), PRICE(I), QTY(I) * PRICE(I)
         TOTQTY = TOTQTY + QTY(I)
         TOTAL = TOTAL + QTY(I) * PRICE(I)
  300 CONTINUE
      PRINT 40, TOTQTY, TOTAL, TOTAL / TOTQTY
      WRITE (6, TOTALS)
      END
