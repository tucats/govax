C     A file of two units. REPORT's TAIL starts at the last balance and runs
C     on over the history, lengthening its view of /ACCTS/ past the main
C     program's; SUMBAL has other names for all of it.
      SUBROUTINE REPORT
      COMMON /ACCTS/ ALL(3)
      REAL TAIL(7)
      EQUIVALENCE (ALL(3), TAIL(1))
      PRINT *, 'HIST', TAIL(2), TAIL(3), TAIL(4), TAIL(5)
      PRINT *, 'UNUSED', TAIL(7)
      END

      REAL FUNCTION SUMBAL()
      COMMON /ACCTS/ B(3), H(6), MARK
      SUMBAL = B(1) + B(2) + B(3) + MARK
      END
