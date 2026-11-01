C     Constants with no type of their own (Phase 82, subtask 5): octal,
C     hexadecimal, and Hollerith constants taking a type from where they are,
C     character constants in numeric contexts, and RADIX-50 constants.
      PROGRAM TYPELS
      INTEGER*2 I2, W(3), S, WW(4)
      INTEGER*4 J, K, IX, IY(2), IZ(4), IV(3)
      REAL*4 X, Y
      REAL*8 D, DR
      REAL*16 Q
      COMPLEX*8 C
      COMPLEX*16 CD
      LOGICAL*4 L
      LOGICAL*2 L2
      BYTE B
      CHARACTER*8 T
      CHARACTER*8 TD
      REAL PX
      PARAMETER (PX = '00004080'X, NH = 2HAB)
      EQUIVALENCE (X, IX), (D, IY), (Q, IZ), (C, IY), (CD, IZ), (D, TD)
      DATA W /3RABC, 3R$.9, 3R   /
      DATA K /6RABCDEF/, S /6RABCDEF/, DR /3RABC/
      DATA WW /2*2RAB, 3Rabc, 1R9/
      INTEGER*4 JR /4RABCD/
      SF(A) = 'AB'
C
C     An octal or hexadecimal constant takes the other operand's type.
      X = '00004080'X
      TYPE 10, X, IX
      Y = X + '00004080'X
      TYPE 11, Y
      L = X .EQ. '00004080'X
      TYPE *, L
      X = PX
      TYPE 11, X
      I2 = 5
      I2 = I2 + '777'O
      TYPE *, I2
      I2 = 'FFFF'X
      TYPE *, I2
      B = 'FF'X
      TYPE *, B
      L = '1'X
      TYPE *, L
      L2 = 'FFFF'X
      TYPE *, L2
      D = 'FFF99A'X
      TYPE 12, IY
      TYPE 14, '00004080'X
      J = 2 ** '3'X
      TYPE *, J
C
C     Assigned, the bits are moved whatever they are.
      X = '8000'X
      TYPE 13, IX
      X = '00000001'X
      TYPE 13, IX
      D = '123456780000800A'X
      TYPE 12, IY
      Q = '0123456789ABCDEF000080001234FFFF'X
      TYPE 15, IZ
      C = '0000000100008000'X
      TYPE 12, IY
      CD = '12345678000000000000000000008000'X
      TYPE 15, IZ
C
C     With nothing to take a type from, the default integer.
      J = '7777'O - 'A39'X
      TYPE *, J
      J = .NOT. '73777'O
      TYPE *, J
      J = -'10'X
      TYPE *, J
      IF ('AF77'X) 20, 20, 21
   20 TYPE *, 'WRONG'
   21 IV('2'X) = 7
      TYPE *, IV('2'O)
      DO 22 J = '1'X, '3'O
        TYPE *, J
   22 CONTINUE
      T = 'ABCDEFGH'
      TYPE *, T('2'X:'3'O)
      IF ('1'X) TYPE *, 'ODD'
      IF ('2'X) TYPE *, 'EVEN'
C
C     Hollerith constants are text, blanks on the right.
      X = 4HABCD
      TYPE 13, IX
      X = 2HAB
      TYPE 13, IX
      I2 = 2HXY
      TYPE 16, I2
      I2 = 1HX
      TYPE 16, I2
      I2 = 4HXY  
      TYPE 16, I2
      D = 8HABCDEFGH
      TYPE *, TD
      J = 1HC - 1HA
      TYPE *, J
      J = .NOT. 1HB
      TYPE *, J
      J = 3 + 1HA
      TYPE 13, J
      J = N*2HAB
      TYPE *, J
      N = 1HZ
      IF (N .EQ. 1HZ) TYPE *, 'ZED'
      IF (2HAB) 30, 30, 31
   30 TYPE *, 'WRONG'
   31 TYPE 13, NH
C
C     A character constant in a numeric context is one too.
      X = 'AB'
      TYPE 13, IX
      J = 'AB'
      TYPE 13, J
      J = 'C' - 'A'
      TYPE *, J
      J = 1 + 'A'
      TYPE 13, J
      L = 'AB' .EQ. 2HAB
      TYPE *, L
      J = .NOT. 'B'
      TYPE *, J
      Y = SF(1.0)
      X = Y
      TYPE 13, IX
      T = 'AB'
      TYPE *, T .EQ. 'AB'
C
C     As an actual argument: no type, four bytes, or a Hollerith constant's
C     own length.
      CALL LONG('34BC2'X)
      CALL TEXT(9HABCDEFGHI)
      CALL TEXT('ABCDEFGHI')
      CALL BYVAL(%VAL('FF'X))
C
C     RADIX-50, in DATA and initial values.
      TYPE *, W
      TYPE 13, K
      TYPE 16, S
      TYPE 13, JR
      TYPE 12, DR
      TYPE *, WW
      CALL WORDS
   10 FORMAT (1X, F4.1, 1X, Z8.8)
   11 FORMAT (1X, F4.1)
   12 FORMAT (1X, Z8.8, 1X, Z8.8)
   13 FORMAT (1X, Z8.8)
   14 FORMAT (1X, Z8.8)
   15 FORMAT (1X, Z8.8, 1X, Z8.8, 1X, Z8.8, 1X, Z8.8)
   16 FORMAT (1X, Z4.4)
      END
C
      SUBROUTINE LONG(J)
      TYPE 10, J
   10 FORMAT (1X, Z8.8)
      END
C
      SUBROUTINE TEXT(B)
      BYTE B(9)
      TYPE 10, B
   10 FORMAT (1X, 9A1)
      END
C
      SUBROUTINE BYVAL(N)
      TYPE *, %LOC(N)
      END
C
C     Under /NOI4 the default integer is a word.
      OPTIONS /NOI4
      SUBROUTINE WORDS
      TYPE *, 'FFFF'X
      TYPE *, 1HA - 1HB
      TYPE *, .NOT. 'FF'X
      END
