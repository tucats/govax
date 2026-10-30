      PROGRAM FACTS
C     N! as an INTEGER and as a REAL, up to 12! (13! overflows a
C     longword), then INTEGER and REAL arithmetic mixed.
      INTEGER N, NFACT
      REAL RFACT, HALF
      NFACT = 1
      RFACT = 1.0
      WRITE (*,*) 'N, N! as an INTEGER, and N! as a REAL'
      DO 10 N = 1, 12
         NFACT = NFACT * N
         RFACT = RFACT * N
         PRINT *, N, NFACT, RFACT
   10 CONTINUE
C     INTEGER division truncates toward zero; a REAL operand makes the
C     operation REAL; assigning a REAL to an INTEGER truncates.
      HALF = 7 / 2
      PRINT *, HALF, 7 / 2.0, 7.0 / 2, -7 / 2
      N = 2.9 * 3
      PRINT *, N, -N / 3, N / 4.0
      RFACT = 0.0
      DO 20 N = 1, 4
   20 RFACT = RFACT + 1.0 / N
      PRINT *, RFACT, -RFACT * 1000, RFACT / 10000
      END
