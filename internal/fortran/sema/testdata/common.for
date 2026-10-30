C     Section 4.2's example: blank common and a named block, a block
C     named again continuing, and an array declared in COMMON.
      PROGRAM MAINP
      COMMON HEAT,X /BLK1/KILO,Q
      COMMON /BLK1/ R(3)
      REAL*8 D
      COMMON /TYPES/ B, D, N
      LOGICAL*1 B
      CHARACTER*5 C
      INTEGER M
      COMMON /CH/ C, M
      LOCAL = KILO
      END
C     Another unit's view of the blocks, the block names in the other
C     order, and a variable with a block's name.
      SUBROUTINE FIGURE
      COMMON /BLK1/LIMA,R / /ALFA,BET
      BLK1 = 1.0
      END
C     Section 4.6's examples, in $LOCAL.
      SUBROUTINE EQUIV1
      DOUBLE PRECISION DVAR
      INTEGER*2 IARR(4)
      EQUIVALENCE (DVAR, IARR(1))
      CHARACTER KEY*16, STAR*10
      EQUIVALENCE (KEY, STAR)
      FIRST = 1.0
      END
C     Table 4-1, three ways, each with arrays of its own.
      SUBROUTINE TABLE1
      DIMENSION TABLE(2,2), TRIPLE(2,2,2)
      EQUIVALENCE (TABLE(2,2), TRIPLE(1,2,2))
      DIMENSION T2(2,2), T3(2,2,2)
      EQUIVALENCE (T2, T3(2,2,1))
      DIMENSION U2(2,2), U3(2,2,2)
      EQUIVALENCE (U2(4), U3(7))
      END
C     Table 4-2: nonunity lower bounds.
      SUBROUTINE TABLE2
      DIMENSION A(2:3,4), B(2:4,4)
      EQUIVALENCE (A(3,4), B(2,4))
      END
C     Section 4.6.2's substrings.
      SUBROUTINE SUBSTR
      CHARACTER NAME*16, ID*9
      EQUIVALENCE (NAME(10:13), ID(2:5))
      CHARACTER FIELDS(100)*4, STAR(5)*5
      EQUIVALENCE (FIELDS(1)(2:4), STAR(2)(3:5))
      END
C     Groups: three lists making one group, placed where its first name
C     comes, and on its strictest member's boundary after a byte.
      SUBROUTINE GROUPS
      LOGICAL*1 FLAG, BYTES(8)
      INTEGER*4 LONG
      EQUIVALENCE (BYTES(3), LONG), (LONG, W)
      Z = 1.0
      END
C     Section 4.6.3's valid example, and a group lengthening a block from
C     its middle, and a name in a group with no storage before the block.
      SUBROUTINE EXTEND
      DIMENSION A(4), B(6)
      COMMON A
      EQUIVALENCE (A(2), B(1))
      COMMON /SHORT/ P, Q
      DIMENSION V(3)
      EQUIVALENCE (V(1), Q)
      END
