C     AVERAGE: the manual's first program (MANUAL.md, 14.1) and the
C     internals tour's (FORTRAN-INTERNALS.md), followed through every pass.
      PROGRAM AVERAGE
      REAL V(4), MEAN
      DO 10 I = 1, 4
   10 V(I) = I * 1.5
      PRINT *, 'Mean:', MEAN(V, 4)
      END
C
      REAL FUNCTION MEAN(A, N)
      REAL A(*)
      MEAN = 0
      DO 10 I = 1, N
   10 MEAN = MEAN + A(I)
      MEAN = MEAN / N
      END
