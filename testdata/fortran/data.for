C     DATA and initial values, in $LOCAL and in common blocks, from a BLOCK
C     DATA in this file: values kept from call to call, section 4.3's byte
C     rules seen through EQUIVALENCE, and an implied-DO over two dimensions.
      PROGRAM DATA
      EXTERNAL BLKDAT
      COMMON /AREA1/ R(3), S, T, U(2) /AREA2/ W, IX, Y
      LOGICAL T, W
      INTEGER S
      INTEGER M(3)/1, 2*7/, N/42/
      REAL X/1/
      DATA Z /-2.5/
      PRINT *, 'M', M, N, X, Z
      PRINT *, 'AREA1', R, S, T
      PRINT *, 'AREA2', W, IX, Y
      CALL COUNT
      CALL COUNT
      CALL COUNT
      CALL BYTES
      CALL TABLE
      END

C     A value set once by DATA, and kept from one call to the next.
      SUBROUTINE COUNT
      SAVE TIMES
      INTEGER TIMES/0/
      TIMES = TIMES + 1
      PRINT *, 'CALL', TIMES
      END

C     1.0 in F_floating is the longword 16512 (^X4080), and 'ABCD' the
C     longword ^X44434241, and 'AB' padded on the right with blanks is
C     ^X20204241. A D_floating 1.0 is the longwords 16512 and 0.
C     INTEGER*2 and LOGICAL*1 values read through a longword.
      SUBROUTINE BYTES
      INTEGER IR, IA, IB, IH, IO, IC, IT, ID(2), IW, IL
      REAL RR, RH
      CHARACTER*4 C
      CHARACTER*2 C2
      DOUBLE PRECISION D
      INTEGER*2 HW(2)
      LOGICAL*1 LB(4)
      CHARACTER*1 CODES(4)
      EQUIVALENCE (RR, IR), (RH, IH), (C, IC), (C2, IT), (D, ID)
      EQUIVALENCE (HW, IW), (LB, IL), (CODES, ICODES)
      DATA RR /1/, IA /'ABCD'/, IB /2HAB/
      DATA RH /'00004080'X/, IO /'12345'O/
      DATA C /'ABCDEFG'/, C2 /'X'/, D /1D0/
      DATA HW /-2, '12345'X/
      DATA LB /.TRUE., .FALSE., 2*'7F'X/
      DATA CODES /65, 66, 'FF'X, '43'X/
      PRINT *, IR, IA, IB, IH, IO
      PRINT *, IC, IT, ID
      PRINT *, IW, IL, ICODES
      END

C     An implied-DO over two dimensions with a step, the column between
C     given by elements, and a vector's odd and even elements apart.
      SUBROUTINE TABLE
      INTEGER T(2,3), V(5), P
      PARAMETER (P=3)
      DATA ((T(I,J), I=1,2), J=1,3,2) /1, 2, 3, 4/
      DATA T(1,2), T(2,2) /2*9/
      DATA (V(K), K=1,5,2) /P*5/, (V(K), K=2,4,2) /-1, -2/
      PRINT *, T
      PRINT *, V
      END

C     Section 4.1's example, in spirit.
      BLOCK DATA BLKDAT
      COMMON /AREA1/ R(3), S, T, U(2) /AREA2/ W, IX, Y
      LOGICAL T, W
      INTEGER S
      DATA R /1.0, 2*2.0/, T /.FALSE./, S /11/
      DATA W /.TRUE./, Y /3.5/, IX /-6/
      END
