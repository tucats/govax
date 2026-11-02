C     The system subroutines of appendix D (D.4, D.5.3), every form of argument the
C     manual gives them. The date and time can't show, so what's printed is whether
C     they agree with each other and with what LIB$DATE_TIME says.
      PROGRAM SYSROU
      CHARACTER*23 NOW
      CALL LIB$DATE_TIME(NOW)
      CALL DATES(NOW)
      CALL TIMES
      CALL IDATES(NOW)
      CALL SECONDS
      CALL RANDOM
      CALL BITS
C     Files are closed, and the status is the program's.
      OPEN (12, FILE='SYSROUTINES.DAT', STATUS='NEW')
      WRITE (12, '(A)') 'closed by EXIT'
      CALL EXIT(3)
      END

C     DATE: the same 9 bytes wherever they go.
      SUBROUTINE DATES(NOW)
      CHARACTER*23 NOW
      CHARACTER*9 WANT, DAY, ELEM(2), HELD
      CHARACTER*11 SURE
      CHARACTER*4 SHORT
      CHARACTER*12 LONG
      BYTE BYTES(9)
      INTEGER*4 LONGS(3)
      CHARACTER*12 OVER
      EQUIVALENCE (LONGS, OVER), (BYTES, HELD)
      STRUCTURE /DATED/
          INTEGER*2 BEFORE
          CHARACTER*9 TEXT
          INTEGER*2 AFTER
      END STRUCTURE
      RECORD /DATED/ REC
      WANT = NOW(1:7) // NOW(10:11)
      CALL DATE(DAY)
      PRINT *, 'CHARACTER', DAY .EQ. WANT
      CALL DATE(BYTES)
      PRINT *, 'BYTE array', HELD .EQ. WANT
      HELD = '*********'
      CALL DATE(BYTES(1))
      PRINT *, 'BYTE element', HELD .EQ. WANT
      OVER = '************'
      CALL DATE(LONGS)
      PRINT *, 'INTEGER array', OVER .EQ. WANT // '***'
      SURE = '***********'
      CALL DATE(SURE(2:10))
      PRINT *, 'substring', SURE .EQ. '*' // WANT // '*'
      ELEM(1) = '*********'
      ELEM(2) = '*********'
      CALL DATE(ELEM(2))
      PRINT *, 'element', ELEM(1) .EQ. '*********', ELEM(2) .EQ. WANT
      CALL DATE(ELEM)
      PRINT *, 'array', ELEM(1) .EQ. WANT, ELEM(2) .EQ. WANT
      REC.BEFORE = -1
      REC.AFTER = -2
      CALL DATE(REC.TEXT)
      PRINT *, 'field', REC.TEXT .EQ. WANT, REC.BEFORE, REC.AFTER
      SHORT = '****'
      CALL DATE(SHORT)
      PRINT *, 'short', SHORT .EQ. WANT(1:4)
      LONG = '************'
      CALL DATE(LONG)
      PRINT *, 'long', LONG .EQ. WANT
      CALL PASSED(LONG(1:11))
      PRINT *, 'passed length', LONG .EQ. WANT
      END

      SUBROUTINE PASSED(TEXT)
      CHARACTER*(*) TEXT
      CALL DATE(TEXT)
      END

C     TIME: hh:mm:ss, 8 bytes.
      SUBROUTINE TIMES
      CHARACTER*8 CLOCK, HELD
      CHARACTER*10 LONG
      CHARACTER*3 SHORT
      BYTE BYTES(8)
      LOGICAL SHAPED
      EQUIVALENCE (BYTES, HELD)
      CALL TIME(CLOCK)
      PRINT *, 'clock', SHAPED(CLOCK)
      HELD = '********'
      CALL TIME(BYTES)
      PRINT *, 'bytes', SHAPED(HELD)
      LONG = '**********'
      CALL TIME(LONG)
      PRINT *, 'long', SHAPED(LONG(1:8)), LONG(9:) .EQ. ' '
      SHORT = '***'
      CALL TIME(SHORT)
      PRINT *, 'short', SHORT(3:3) .EQ. ':'
      END

      LOGICAL FUNCTION SHAPED(T)
      CHARACTER*8 T
      CHARACTER*1 C
      INTEGER HH, MM, SS
      READ (T, '(I2,A1,I2,A1,I2)') HH, C, MM, C, SS
      SHAPED = T(3:3) .EQ. ':' .AND. T(6:6) .EQ. ':' .AND. HH .LE. 23
     1    .AND. MM .LE. 59 .AND. SS .LE. 59
      END

C     IDATE: month, day, and year's last two digits, by the arguments' length.
      SUBROUTINE IDATES(NOW)
      CHARACTER*23 NOW
      CHARACTER*3 MONTHS(12)
      DATA MONTHS /'JAN','FEB','MAR','APR','MAY','JUN','JUL','AUG',
     1    'SEP','OCT','NOV','DEC'/
      INTEGER*2 IM, ID, IY, GUARD(3)
      INTEGER M, D, Y, THREE(3), YEAR, DAYOF
      CALL IDATE(M, D, Y)
      PRINT *, 'longwords', MONTHS(M) .EQ. NOW(4:6),
     1    Y .EQ. MOD(YEAR(NOW), 100), D .EQ. DAYOF(NOW)
      CALL IDATE(IM, ID, IY)
      PRINT *, 'words', IM .EQ. M, ID .EQ. D, IY .EQ. Y
      CALL IDATE(THREE(1), THREE(2), THREE(3))
      PRINT *, 'elements', THREE(1) .EQ. M, THREE(2) .EQ. D,
     1    THREE(3) .EQ. Y
      GUARD(1) = -1
      GUARD(3) = -1
      CALL IDATE(IM, GUARD(2), IY)
      PRINT *, 'neighbors', GUARD(1), GUARD(3)
      END

      INTEGER FUNCTION YEAR(NOW)
      CHARACTER*23 NOW
      READ (NOW(8:11), '(I4)') YEAR
      END

      INTEGER FUNCTION DAYOF(NOW)
      CHARACTER*23 NOW
      READ (NOW(1:2), '(BN,I2)') DAYOF
      END

C     SECNDS: REAL whatever IMPLICIT says, a day added once if the result would be
C     negative.
      SUBROUTINE SECONDS
      IMPLICIT INTEGER (A-Z)
      REAL T1, ELAPSED, AHEAD, TODAY, FAR
      T1 = SECNDS(0.0)
      ELAPSED = SECNDS(T1)
      PRINT *, 'elapsed', ELAPSED .GE. 0.0 .AND. ELAPSED .LT. 10.0
      PRINT *, 'today', T1 .GE. 0.0 .AND. T1 .LT. 86400.0
      TODAY = SECNDS(0.0)
      AHEAD = SECNDS(86400.0)
      PRINT *, 'ahead', ABS(AHEAD - TODAY) .LT. 10.0
      FAR = SECNDS(1.0E7)
      PRINT *, 'once', FAR .LT. 0.0
      PRINT *, 'an expression', SECNDS(T1 + 1.0) .LE. SECNDS(T1)
      END

C     RAN: MTH$RANDOM, SEED = 69069 * SEED + 1, with the seed's high 24 bits as the
C     fraction; the seed may be an element.
      SUBROUTINE RANDOM
      IMPLICIT INTEGER (A-Z)
      INTEGER SEEDS(2)
      SEED = 1
      PRINT 10, RAN(SEED) * 16777216.0, SEED
      PRINT 10, RAN(SEED) * 16777216.0, SEED
      SEEDS(1) = 12345
      SEEDS(2) = 0
      PRINT 10, RAN(SEEDS(1)) * 16777216.0, SEEDS(1)
      PRINT 10, RAN(SEEDS(2)) * 16777216.0, SEEDS(2)
10    FORMAT (1X, F12.1, I13)
      END

C     MVBITS: the field moved, and only the field.
      SUBROUTINE BITS
      INTEGER M, N, I, J, LEN, TWO(2)
      INTEGER*2 W, X, WI, WTWO(2)
      PARAMETER (LEN = 3)
      M = 15
      N = 0
      CALL MVBITS(M, 1, LEN, N, 4)
      PRINT 10, 'part', N, M
      M = '12345678'X
      N = -1
      I = 8
      J = 16
      CALL MVBITS(M, I, I, N, J)
      PRINT 10, 'in the middle', N, M
      M = 7
      CALL MVBITS(M, 0, 8, M, 4)
      PRINT 10, 'in place', M, M
      M = 1
      N = 0
      CALL MVBITS(M, 0, 1, N, 31)
      PRINT 10, 'the sign bit', N, M
      M = '12345678'X
      N = 7
      CALL MVBITS(M, 0, 0, N, 0)
      PRINT 10, 'no bits', N, M
      CALL MVBITS(M, 0, 32, N, 0)
      PRINT 10, 'them all', N, M
      TWO(1) = 0
      TWO(2) = 0
      M = -1
      CALL MVBITS(M, I + 1, 4 * 2, TWO(2), 2 ** 3)
      PRINT 10, 'expressions', TWO(1), TWO(2)
      W = 15
      X = 0
      CALL MVBITS(W, 1, LEN, X, 4)
      PRINT 20, 'words', X, W
      W = 1
      X = 0
      CALL MVBITS(W, 0, 1, X, 15)
      PRINT 20, 'the sign bit', X, W
      W = -1
      WI = 8
      WTWO(1) = 0
      WTWO(2) = 0
      CALL MVBITS(W, WI, I, WTWO(1), J - 12)
      PRINT 20, 'word and longword', WTWO(1), WTWO(2)
10    FORMAT (1X, A20, 2Z10.8)
20    FORMAT (1X, A20, 2Z6.4)
      END
