C     Records one posting: blank common, /STATS/ by INCLUDE, and the whole
C     of /ACCTS/.
      SUBROUTINE POST(IACCT, AMOUNT)
      INCLUDE 'SHARED.INC'
      COMMON /ACCTS/ BAL(3), HIST(6), MARK
      COMMON LAST
      BAL(IACCT) = BAL(IACCT) + AMOUNT
      NPOST = NPOST + 1
      TOTAL = TOTAL + AMOUNT
      HIST(NPOST) = AMOUNT
      MARK = MARK + 1
      LAST = IACCT
      END
