C     Tables set by DATA and used by programs: days in the months, a binary
C     search, an interpolation, tax brackets in a common block from a BLOCK
C     DATA, and a matrix product of matrices given by rows and by columns.
      PROGRAM TABLES
      EXTERNAL RATES
      INTEGER A(2,3), B(3,2), C(2,2)
      DATA ((A(I,J), J=1,3), I=1,2) /1, 2, 3, 4, 5, 6/
      DATA B /7, 9, 11, 8, 10, 12/
      DATA C /4*0/
      PRINT *, 'DAYS', IDAY(1988,3,1), IDAY(1987,3,1), IDAY(1988,12,31)
      PRINT *, 'DAYS', IDAY(1900,3,1), IDAY(2000,3,1)
      PRINT *, 'FOUND', LOCATE(23), LOCATE(2), LOCATE(29), LOCATE(4)
      PRINT *, 'INTERP', YAT(0.5), YAT(1.5), YAT(3.0), YAT(5.0)
      PRINT *, 'TAX', TAX(500.0), TAX(3000.0), TAX(12000.0),
     1  TAX(30000.0)
      DO 30 I = 1, 2
      DO 30 J = 1, 2
      DO 30 K = 1, 3
   30 C(I,J) = C(I,J) + A(I,K) * B(K,J)
      PRINT *, 'PRODUCT', C
      END

C     The day of the year. A year divisible by 4 is a leap year, unless by
C     100 and not by 400.
      INTEGER FUNCTION IDAY(IY, M, ID)
      INTEGER MLEN(12)
      LOGICAL LEAP
      DATA MLEN /31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31/
      LEAP = (IY/4*4 .EQ. IY .AND. IY/100*100 .NE. IY)
     1       .OR. IY/400*400 .EQ. IY
      IDAY = ID
      DO 10 K = 1, M - 1
   10 IDAY = IDAY + MLEN(K)
      IF (LEAP .AND. M .GT. 2) IDAY = IDAY + 1
      END

C     K's place in a sorted table, or 0.
      INTEGER FUNCTION LOCATE(K)
      PARAMETER (N=10)
      INTEGER PRIME(N)
      DATA PRIME /2, 3, 5, 7, 11, 13, 17, 19, 23, 29/
      LOCATE = 0
      LOW = 1
      IHIGH = N
   10 IF (LOW .GT. IHIGH) RETURN
      MID = (LOW + IHIGH) / 2
      IF (PRIME(MID) .EQ. K) THEN
        LOCATE = MID
        RETURN
      ELSE IF (PRIME(MID) .LT. K) THEN
        LOW = MID + 1
      ELSE
        IHIGH = MID - 1
      END IF
      GO TO 10
      END

C     Straight lines between a table's points, and the last line on past
C     its end.
      REAL FUNCTION YAT(X)
      REAL XT(4), YT(4)
      DATA XT /0.0, 1.0, 2.0, 4.0/, YT /0.0, 10.0, 30.0, 70.0/
      DO 10 I = 2, 4
      IF (X .LE. XT(I)) GO TO 20
   10 CONTINUE
      I = 4
   20 YAT = YT(I-1) + (X-XT(I-1)) * (YT(I)-YT(I-1)) / (XT(I)-XT(I-1))
      END

C     Each bracket's rate on the part of the income in it.
      REAL FUNCTION TAX(X)
      COMMON /BRACK/ LIMIT(4), RATE(4)
      TAX = 0.0
      LOW = 0
      DO 10 I = 1, 4
      IF (X .LE. LOW) GO TO 20
      TOP = LIMIT(I)
      IF (X .LT. TOP) TOP = X
      TAX = TAX + (TOP - LOW) * RATE(I)
      LOW = LIMIT(I)
   10 CONTINUE
   20 RETURN
      END

      BLOCK DATA RATES
      COMMON /BRACK/ LIMIT(4), RATE(4)
      DATA LIMIT /1000, 5000, 20000, 1000000/
      DATA RATE /0.0, 0.125, 0.25, 0.5/
      END
