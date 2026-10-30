C     Names in common blocks and overlapping names in $LOCAL are cells
C     with places of their own; the code reaches them as any other.
      PROGRAM COMMON1
      COMMON /BLK/ I, A(3), X
      COMMON K
      REAL R
      INTEGER N, IA(2)
      EQUIVALENCE (R, N), (IA(2), R)
      A(2) = X
      K = I
      N = 1
      PRINT *, A, IA(1)
      END
      SUBROUTINE S
      COMMON /BLK/ J
      J = 1
      END
