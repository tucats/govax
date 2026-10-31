C     Every length of INTEGER and LOGICAL (Phase 78, subtask 2): section
C     2.2.1.1's unsigned constants, word arithmetic, mixed lengths,
C     logical operators, short arguments and array elements, a word DO
C     variable and function, and OPTIONS /NOI4.
      PROGRAM LENGTHS
      BYTE B, BV(3)
      LOGICAL*1 L1
      LOGICAL*2 L2
      LOGICAL*4 L4
      INTEGER*2 I2, J2, K2, IV(3), NEXT2
      INTEGER*4 I4, J4
C     A short variable takes a constant's bits (2.2.1.1).
      B = 255
      I2 = 65535
      PRINT *, B, I2
      B = -128
      I2 = -32768
      PRINT *, B, I2
C     A small constant beside a word is a word; two constants aren't.
      I2 = 100
      I4 = I2*1000
      PRINT *, I4
      J4 = 1000
      I4 = I2*J4
      PRINT *, I4
      I4 = 30000+30000
      PRINT *, I4
C     A word wraps. A constant too big for one makes it a longword.
      I2 = 32767
      J2 = I2 + 1
      PRINT *, J2
      I4 = I2 + 40000
      PRINT *, I4
      J2 = I4
      PRINT *, J2
C     Conversions to and from REAL, and truncating division.
      I2 = -7
      J2 = I2/2
      PRINT *, J2
      X = I2
      PRINT *, X
      J2 = 3.9*I2
      PRINT *, J2
      B = 12.7
      PRINT *, B
C     The logical operators, on logicals of every length.
      L1 = .TRUE.
      L2 = .FALSE.
      L4 = .TRUE.
      PRINT *, L1, L2, L4
      PRINT *, L1 .AND. L2, L1 .AND. L4, L1 .OR. L2, .NOT. L2
      PRINT *, L2 .EQV. L2, L1 .NEQV. L4, L1 .XOR. L2, L4 .EQV. L2
C     And on integers, bit by bit.
      I2 = 5
      I4 = 12
      PRINT *, I2 .AND. 3, I2 .OR. 3, .NOT. I2
      PRINT *, I4 .XOR. 10, I4 .EQV. 10, I2 .AND. I4
C     A logical in arithmetic is an integer, of its own length or the
C     other operand's. Two bytes make a word.
      PRINT *, L1 + 1, L4 + L4, L4 + I2
      B = 100
      PRINT *, B + B
C     Comparisons, and bit 0 of a short value as a condition.
      IF (I2 .EQ. 5) PRINT *, 'I2 IS 5'
      IF (B .GT. 99) PRINT *, 'B IS OVER 99'
      IF (L1) PRINT *, 'L1 IS TRUE'
      IF (L2) PRINT *, 'L2 IS TRUE'
      IF (.NOT. L2) PRINT *, 'L2 IS FALSE'
      IF (I2) PRINT *, 'I2 IS ODD'
C     A word DO variable, and word and byte array elements.
      DO 10 K2 = 1, 3
         IV(K2) = K2*K2
         BV(K2) = K2 - 2
         IF (BV(K2)) PRINT *, 'BV', K2, ' IS ODD'
   10 CONTINUE
      PRINT *, IV
      PRINT *, BV
C     Short arguments, by a place with a subscript.
      K2 = 2
      CALL BUMP(IV(K2), BV(K2))
      PRINT *, IV(2), BV(2)
      PRINT *, NEXT2(IV(K2)), ISUM(IV, 3)
      CALL SHORT
      CALL LONG
      END

      SUBROUTINE BUMP(W, C)
      INTEGER*2 W
      BYTE C
      W = W + 1
      C = C - 1
      END

      INTEGER*2 FUNCTION NEXT2(N)
      INTEGER*2 N
      NEXT2 = N + 1
      END

      FUNCTION ISUM(A, N)
      INTEGER*2 A(*)
      ISUM = 0
      DO 10 I = 1, N
   10 ISUM = ISUM + A(I)
      END

C     OPTIONS holds for its unit: INTEGER, LOGICAL, and small constants are
C     words here, and longwords again in the next unit.
      OPTIONS /NOI4
      SUBROUTINE SHORT
      INTEGER M
      LOGICAL T
      M = 32767
      M = M + 1
      N = 40000
      T = .TRUE.
      PRINT *, M, N, T, 5
      END

      SUBROUTINE LONG
      INTEGER M
      M = 32767
      M = M + 1
      PRINT *, M, 5
      END
