C     The main program. It declares only the first part of /ACCTS/, which
C     POST, OPENBK, and SUMBAL declare longer, and REPORT longer by
C     EQUIVALENCE; the block's values come from OPENBK, in its own file,
C     which EXTERNAL names.
      PROGRAM LEDGER
      EXTERNAL OPENBK
      INCLUDE 'SHARED.INC'
      COMMON /ACCTS/ BAL(3)
      COMMON LAST
      PRINT *, 'OPEN', BAL, NPOST, TOTAL
      CALL POST(1, 50.0)
      CALL POST(2, 25.0)
      CALL POST(1, -12.5)
      CALL POST(3, 4.0)
      PRINT *, 'BAL', BAL
      PRINT *, 'POSTS', NPOST, TOTAL, LAST
      CALL REPORT
      X = SUMBAL()
      PRINT *, 'SUM', X
      END
