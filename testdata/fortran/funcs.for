C     Functions whose values go through a hidden first argument (Phase 79,
C     subtask 5): CHARACTER of a fixed length, of the default length, with
C     the length after the name, and passed-length, the manual's REPEAT at
C     two lengths in two units; REAL*16 and COMPLEX*16, typed every way. A
C     function that writes its name before it reads its argument shows that
C     H = F(H) keeps the two apart.
      PROGRAM FUNCS
      CHARACTER*5 NAME5, REV, S5
      CHARACTER*10 REPEAT, MANY
      CHARACTER INITL
      CHARACTER*2 PAIR
      CHARACTER*4 PASSON, FILLED
      CHARACTER*3 PICK, WORDS(2)
      INTEGER LENOF, I
      REAL*16 QSUM, QTWICE, QLEN, QTY, Q
      COMPLEX*16 ZSQ, Z
      DOUBLE COMPLEX ZNEG
      DOUBLE PRECISION D
C     Fixed length, a substring of the function's name
      PRINT *, NAME5(2)
      S5 = NAME5(3)
      PRINT *, S5
C     The value and the argument are one variable
      S5 = 'HELLO'
      S5 = REV(S5)
      PRINT *, S5
C     Passed length: as long as this unit says
      MANY = REPEAT('A')
      PRINT *, MANY
      PRINT *, LEN(REPEAT('B'))
      CALL TWO
C     The default length, and a length after the name
      PRINT *, INITL('HELLO')
      PRINT *, PAIR('X')
C     In expressions: as an argument, in a concatenation, a comparison, and
C     the intrinsic functions
      PRINT *, INITL(NAME5(1))
      PRINT *, LENOF(NAME5(2))
      PRINT *, '[' // PAIR('Y') // INITL('Z') // ']'
      IF (PAIR('Y') .EQ. 'YY') PRINT *, 'EQUAL'
      IF (PAIR('Y') .NE. 'YY') PRINT *, 'NOT EQUAL'
      PRINT *, ICHAR(INITL('A'))
      PRINT *, INDEX(NAME5(3), '-')
C     The function's name passed on, by its own descriptor or a new one
      PRINT *, '[' // PASSON('Z') // ']'
      PRINT *, '[' // FILLED('W') // ']'
C     Dummy arguments after the hidden one
      WORDS(1) = 'ONE'
      WORDS(2) = 'TWO'
      DO 10 I = 1, 2
10    PRINT *, PICK(WORDS, I)
C     REAL*16
      Q = QSUM(1.5Q0, 2.25Q0)
      PRINT *, Q
      PRINT *, QSUM(1Q0, 2Q0)
      Q = QSUM(Q + 1, 2Q0) * 2
      PRINT *, Q
      Q = 7.5Q0
      Q = QTWICE(Q)
      PRINT *, Q
      PRINT *, QLEN(2.5)
      PRINT *, QTY(-7.0)
C     COMPLEX*16
      Z = (1D0, 2D0)
      Z = ZSQ(Z)
      PRINT *, Z
      Z = ZSQ(Z) + (1D0, 0D0)
      PRINT *, Z
      PRINT *, ZNEG((1D0, -2D0))
      D = ZSQ((2D0, 1D0))
      PRINT *, D
      PRINT *, DIMAG(ZSQ((2D0, 1D0)))
      END

C     The manual's other program unit: another length
      SUBROUTINE TWO
      CHARACTER HOLD*6, REPEAT*2
      HOLD = REPEAT('A') // REPEAT('B') // REPEAT('C')
      PRINT *, HOLD
      END

      CHARACTER*5 FUNCTION NAME5(I)
      NAME5 = 'ABCDE'
      NAME5(I:I) = '-'
      END

      CHARACTER*5 FUNCTION REV(S)
      CHARACTER*5 S
      REV = ' '
      DO 10 I = 1, 5
10    REV(I:I) = S(6-I:6-I)
      RETURN
      END

C     Section 6.2.2.2's example (CARG declared, where the manual's has a
C     misprint)
      CHARACTER*(*) FUNCTION REPEAT(CARG)
      CHARACTER*1 CARG
      DO 10 I = 1, LEN(REPEAT)
10    REPEAT(I:I) = CARG
      RETURN
      END

      CHARACTER FUNCTION INITL(S)
      CHARACTER*(*) S
      INITL = S
      END

      CHARACTER FUNCTION PAIR*2(C)
      CHARACTER*(*) C
      PAIR = C // C
      END

      INTEGER FUNCTION LENOF(S)
      CHARACTER*(*) S
      LENOF = LEN(S)
      END

      CHARACTER*(*) FUNCTION PASSON(C)
      CHARACTER*(*) C
      CALL SETTO(PASSON, C)
      END

      CHARACTER*4 FUNCTION FILLED(C)
      CHARACTER*(*) C
      CALL SETTO(FILLED, C)
      END

      SUBROUTINE SETTO(S, C)
      CHARACTER*(*) S, C
      S = C
      END

      CHARACTER*3 FUNCTION PICK(A, I)
      CHARACTER*3 A(*)
      PICK = A(I)
      END

      REAL*16 FUNCTION QSUM(A, B)
      REAL*16 A, B
      QSUM = A + B
      END

      REAL*16 FUNCTION QTWICE(X)
      REAL*16 X
      QTWICE = 0
      QTWICE = X * 2
      END

C     REAL*16 by a length after the name, the type from its first letter
      FUNCTION QLEN*16(X)
      QLEN = X
      END

C     REAL*16 by a type statement
      FUNCTION QTY(X)
      REAL*16 QTY
      QTY = X
      END

      COMPLEX*16 FUNCTION ZSQ(X)
      COMPLEX*16 X
      ZSQ = (0D0, 0D0)
      ZSQ = X * X
      END

      DOUBLE COMPLEX FUNCTION ZNEG(X)
      DOUBLE COMPLEX X
      ZNEG = -X
      END
