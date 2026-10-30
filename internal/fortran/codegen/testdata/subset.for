C     Every statement of Phase 75's subset, in three program units.
      PROGRAM SUBSET
      INTEGER I, N, TABLE(10)
      REAL X, Y, AVG
      LOGICAL DONE
      DIMENSION V(0:9)
      N = 10
      DONE = .FALSE.
C     Fill the table, then sum it.
      DO 10 I = 1, N
         TABLE(I) = I*I - 2*I + 1
         V(I-1) = I / 2.0
   10 CONTINUE
      CALL SUMUP(TABLE, N, X)
      Y = AVG(TABLE, N)
      IF (Y .GT. 100.0 .AND. .NOT. DONE) THEN
         PRINT *, 'BIG', Y
      ELSE IF (Y .GT. 10) THEN
         WRITE (*,*) 'MEDIUM', Y
      ELSE
         WRITE (UNIT=*, FMT=*) 'SMALL', Y, DONE, V
      END IF
      I = 0
   20 I = I + 1
      IF (I .LT. 3) GO TO 20
      IF (I .NE. 3) STOP 'WRONG COUNT'
      PRINT *
      STOP
      END
C
      SUBROUTINE SUMUP(T, N, S)
      INTEGER N, T(*)
      S = 0.0
      DO 100, K = N, 1, -1
  100 S = S + T(K)
      RETURN
      END
C
      REAL FUNCTION AVG(T, N)
      INTEGER N, T(10)
      AVG = 0
      DO 200 J = 1, N
         AVG = AVG + T(J)
  200 CONTINUE
      AVG = AVG / N
      END
