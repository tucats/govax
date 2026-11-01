C     Constants with no type of their own, each moved or computed as its type
C     says (Phase 82, subtask 5).
      PROGRAM CONSTS
      INTEGER*2 W
      REAL*8 D
      REAL*16 Q
      COMPLEX*8 C
      COMPLEX*16 CD
      INTEGER*2 R50 /3RABC/
      INTEGER J /6RABCDEF/
C     Bits that are no number: moved as integers of their width.
      X = '8000'X
      D = '8000'X
      Q = '8000'X
      C = '8000'X
      CD = '8000'X
C     Bits that are a number: the same.
      X = '00004080'X
      D = 8HABCDEFGH
C     Zero bits clear the place.
      D = '0'X
C     In an expression, a number of the other operand's type.
      Y = X + '00004080'X
      W = W + 2HXY
      I = 1HC - 1HA
      X = 'AB'
      I = I + 'A'
      IF ('1'X) I = .NOT. 'FF'X
      CALL S('34BC2'X, 4HABCD, 'ABCD')
      END
