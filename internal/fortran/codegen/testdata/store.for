C     The tour's program for declarations and storage
C     (docs/FORTRAN-INTERNALS.md, section 8).
      PROGRAM STORE
      PARAMETER (N = 3)
      COMMON /SHARED/ COUNT, TOTAL(N)
      INTEGER COUNT, TABLE(N)/1, 2*7/, BITS
      REAL X
      EQUIVALENCE (X, BITS)
      DATA X /1.0/
      COUNT = TABLE(2)
      PRINT *, COUNT, BITS, TOTAL
      END

      BLOCK DATA START
      COMMON /SHARED/ COUNT, TOTAL(3)
      INTEGER COUNT
      DATA TOTAL /3*2.5/
      END
