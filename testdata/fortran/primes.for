      PROGRAM PRIMES
C     The primes below 100 by trial division, the remainder computed by
C     hand (MOD is a later phase's), kept in an array and printed whole.
      INTEGER N, D, COUNT, P(25)
      LOGICAL PRIME
      COUNT = 0
      DO 20 N = 2, 100
         PRIME = .TRUE.
         DO 10 D = 2, N - 1
            IF (D*D .GT. N) GO TO 15
            IF (N - (N/D)*D .EQ. 0) PRIME = .FALSE.
            IF (.NOT. PRIME) GO TO 15
   10    CONTINUE
   15    IF (PRIME) THEN
            COUNT = COUNT + 1
            P(COUNT) = N
         END IF
   20 CONTINUE
      PRINT *, COUNT, ' primes below 100:'
      PRINT *, P
      PRINT *, 'Is 91 prime? ', 91 - (91/7)*7 .NE. 0
      END
