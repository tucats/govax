C     The opening balances. HIST has no values, so it starts as zeros.
      BLOCK DATA OPENBK
      INCLUDE 'SHARED.INC'
      COMMON /ACCTS/ BAL(3), HIST(6), MARK
      DATA BAL /100.0, 200.0, 300.0/, MARK /7/
      DATA NPOST /0/, TOTAL /0.0/
      END
