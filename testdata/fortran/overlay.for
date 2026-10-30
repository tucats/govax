C     EQUIVALENCE overlays: a matrix filled through a vector, an element
C     named by its number, a chain of lists, a LOGICAL's and a REAL's bits
C     read through an INTEGER, an overlay's neighbors left alone, and a
C     common block seen through overlays in two units.
      PROGRAM OVERLAY
      REAL M(3,3), V(9)
      EQUIVALENCE (M, V)
      INTEGER GRID(2,3), SLOT(2)
      EQUIVALENCE (GRID(5), SLOT(1))
      REAL A, B(3), C
      EQUIVALENCE (A, B(2)), (B(3), C)
      LOGICAL FLAG
      EQUIVALENCE (FLAG, IFLAG)
      REAL R
      EQUIVALENCE (R, IR)
      INTEGER IBEFOR, P(4), Q(2,2), IAFTER
      EQUIVALENCE (P(2), Q(1,1))
      COMMON /POINT/ X, Y, Z
      REAL PT(3)
      EQUIVALENCE (X, PT)

C     The identity matrix, set a vector's elements at a time. M(2,3) is
C     the eighth element in column-major order.
      DO 10 I = 1, 9
   10 V(I) = 0.0
      DO 20 I = 1, 9, 4
   20 V(I) = 1.0
      M(2,3) = 5.0
      PRINT *, 'M', M(1,1), M(2,2), M(3,3)
      PRINT *, 'V', V(8), V(2)

C     GRID's fifth element is GRID(1,3), so SLOT(2) is GRID(2,3).
      SLOT(1) = 11
      SLOT(2) = 12
      PRINT *, 'GRID', GRID(1,3), GRID(2,3)

C     Two lists through B make one group.
      B(1) = 1.0
      B(2) = 2.0
      B(3) = 3.0
      PRINT *, 'CHAIN', A, C
      A = 20.0
      C = 30.0
      PRINT *, 'B', B

C     .TRUE. is the longword -1, and only bit 0 of a LOGICAL counts.
      FLAG = .TRUE.
      PRINT *, 'FLAG', IFLAG
      IFLAG = 2
      PRINT *, 'BIT0', FLAG
      IFLAG = 3
      PRINT *, 'BIT0', FLAG

C     1.5 in F_floating is ^X40C0, and the exponent's lowest bit is 128:
C     adding it doubles the number.
      R = 1.5
      PRINT *, 'BITS', IR
      IR = IR + 128
      PRINT *, 'TWICE', R

C     Q runs one longword past P, and nothing else moves.
      IBEFOR = -1
      IAFTER = -2
      DO 30 K = 1, 4
   30 P(K) = K
      DO 40 J = 1, 2
      DO 40 I = 1, 2
   40 Q(I,J) = Q(I,J) * 10
      PRINT *, 'P', P
      PRINT *, 'Q', Q(2,2)
      PRINT *, 'GUARD', IBEFOR, IAFTER

C     The block under three names: here X, Y, Z and PT; in SCALE an array;
C     in SWAP integers over the same longwords.
      X = 1.0
      Y = 2.0
      Z = 3.0
      CALL SCALE(2.0)
      PRINT *, 'POINT', X, Y, Z, PT(3)
      PT(2) = 10.0
      PRINT *, 'Y', Y
      CALL SWAP
      PRINT *, 'SWAPPED', X, Z
      END

      SUBROUTINE SCALE(F)
      COMMON /POINT/ W(3)
      DO 10 I = 1, 3
   10 W(I) = W(I) * F
      END

C     Moving a REAL's bits through INTEGERs moves the REAL.
      SUBROUTINE SWAP
      COMMON /POINT/ X, Y, Z
      INTEGER IX(3)
      EQUIVALENCE (X, IX)
      ITEMP = IX(1)
      IX(1) = IX(3)
      IX(3) = ITEMP
      END
