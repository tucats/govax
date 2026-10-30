C     COMMON and EQUIVALENCE: units of one file sharing blocks under names
C     of their own, a block lengthened by EQUIVALENCE, a REAL's bits read
C     through an INTEGER, and arrays overlaid in $LOCAL.
      PROGRAM COMMON
      COMMON /SHARED/ I, X, A(3)
      COMMON TOTAL, N
      DIMENSION B(6)
      COMMON /EXT/ C(4)
      EQUIVALENCE (C(2), B(1))
      REAL R
      INTEGER BITS
      EQUIVALENCE (R, BITS)
      INTEGER P(4), Q(2,2)
      EQUIVALENCE (P(2), Q(1,1))
      I = 7
      X = 2.5
      A(1) = 1.0
      A(2) = 2.0
      A(3) = 3.0
      CALL SETUP
      PRINT *, 'BLANK', TOTAL, N
      CALL ADD(10)
      CALL ADD(20)
      PRINT *, 'ADDED', TOTAL, N
      PRINT *, 'SHARED', I, X, A
C     B(1) is C(2), and B(3) past C(4) is the block's own.
      C(2) = 5.0
      C(4) = 6.0
      B(5) = 8.0
      CALL SEEEXT
      PRINT *, 'B', B(1), B(3), B(5)
C     1.0 in F_floating is the longword 16512 (^X4080).
      R = 1.0
      PRINT *, 'BITS', BITS
      BITS = 16448
      PRINT *, 'REAL', R
      DO 10 K = 1, 4
   10 P(K) = K * 10
      PRINT *, 'Q', Q(1,1), Q(2,1), Q(1,2)
      Q(1,2) = 99
      PRINT *, 'P', P(4)
      END
C     The block's names here are other names, in another order of types.
      SUBROUTINE SETUP
      COMMON /SHARED/ J, Y, D(3)
      COMMON SUM, M
      SUM = 100.0
      M = 1
      J = J + 1
      Y = Y * 2.0
      D(2) = D(1) + D(3)
      END
C     A unit that names only part of a block.
      SUBROUTINE ADD(K)
      COMMON TOTAL, N
      TOTAL = TOTAL + K
      N = N + 1
      END
C     A longer view of the block EQUIVALENCE lengthened in the main program.
      SUBROUTINE SEEEXT
      COMMON /EXT/ E(7)
      PRINT *, 'EXT', E(2), E(4), E(6)
      END
