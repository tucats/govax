C The STR$ arithmetic routines from FORTRAN (Phase 113, subtask 7). Each
C number is three arguments: a sign (0 or 1), the power of ten its digits
C are multiplied by, and the digits, a string. A = -1000 and B = .0002 are
C the manual's examples' numbers. A CHARACTER variable takes a result's
C digits right-justified, zeros before them, as VMS 7.1's routines write
C them (subtask 13a), and one too short for them is STR$_TRU.
      PROGRAM STRMATH
      IMPLICIT NONE
      INCLUDE '($SSDEF)'
      INCLUDE '($STRDEF)'
      CHARACTER*20 C
      CHARACTER*3 SHORT
      INTEGER CSIGN, CEXP, STATUS, DIVZERO
      INTEGER STR$ADD, STR$MUL, STR$DIVIDE, STR$RECIP, STR$ROUND
C
C     -1000 + .0002 = -999.9998, and the product, -.2.
      STATUS = STR$ADD(1, 3, '1', 0, -4, '2', CSIGN, CEXP, C)
      CALL SHOW('ADD', STATUS, CSIGN, CEXP, C)
      STATUS = STR$MUL(1, 3, '1', 0, -4, '2', CSIGN, CEXP, C)
      CALL SHOW('MUL', STATUS, CSIGN, CEXP, C)
C
C     The quotient, -5000000, to 3 places, truncated (0); 22/7 to 6,
C     rounded (1).
      STATUS = STR$DIVIDE(1, 3, '1', 0, -4, '2', 3, 0, CSIGN, CEXP, C)
      CALL SHOW('DIVIDE', STATUS, CSIGN, CEXP, C)
      STATUS = STR$DIVIDE(0, 0, '22', 0, 0, '7', 6, 1, CSIGN, CEXP, C)
      CALL SHOW('DIVIDE', STATUS, CSIGN, CEXP, C)
C
C     1/A to the place B's digit is in: -.001.
      STATUS = STR$RECIP(1, 3, '1', 0, -4, '2', CSIGN, CEXP, C)
      CALL SHOW('RECIP', STATUS, CSIGN, CEXP, C)
C
C     999.9998 to three significant digits, rounded (0) and truncated
C     (1); 3.1415926 to five.
      STATUS = STR$ROUND(3, 0, 0, -4, '9999998', CSIGN, CEXP, C)
      CALL SHOW('ROUND', STATUS, CSIGN, CEXP, C)
      STATUS = STR$ROUND(3, 1, 0, -4, '9999998', CSIGN, CEXP, C)
      CALL SHOW('ROUND', STATUS, CSIGN, CEXP, C)
      STATUS = STR$ROUND(5, 0, 0, -7, '31415926', CSIGN, CEXP, C)
      CALL SHOW('ROUND', STATUS, CSIGN, CEXP, C)
C
C     Too long for three characters.
      STATUS = STR$ADD(0, 0, '12340', 0, 0, '5', CSIGN, CEXP, SHORT)
      PRINT 100, 'TRU', STATUS .EQ. STR$_TRU, CSIGN, CEXP, SHORT
C
C     Dividing by zero signals, and LIB$SIG_TO_RET makes it a status.
      PRINT 100, 'ZERO', DIVZERO() .EQ. STR$_DIVBY_ZER
100   FORMAT (1X, A, L2, I3, I4, 1X, A)
      END

C     SHOW prints a result: its status, sign, exponent, and digits.
      SUBROUTINE SHOW(NAME, STATUS, CSIGN, CEXP, C)
      IMPLICIT NONE
      INCLUDE '($SSDEF)'
      CHARACTER*(*) NAME, C
      INTEGER STATUS, CSIGN, CEXP
      PRINT 100, NAME, STATUS .EQ. SS$_NORMAL, CSIGN, CEXP, C
100   FORMAT (1X, A, L2, I3, I4, 1X, A)
      END

C     DIVZERO is STR$DIVIDE's status for 1 / 0.
      INTEGER FUNCTION DIVZERO()
      IMPLICIT NONE
      CHARACTER*20 C
      INTEGER CSIGN, CEXP, STR$DIVIDE
      EXTERNAL LIB$SIG_TO_RET
      CALL LIB$ESTABLISH(LIB$SIG_TO_RET)
      DIVZERO = STR$DIVIDE(0, 0, '1', 0, 0, '0', 2, 0, CSIGN, CEXP, C)
      END
