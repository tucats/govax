C     Statement functions (Phase 79, subtask 10, the manual's section 6.2.1),
C     expanded in line: a dummy argument's name only gives it its type, whatever
C     else the unit calls that name; the manual's VOLUME, SINH (an intrinsic's
C     name), AVG, and CSF; every type of value; functions using those before
C     them, one reference inside another's argument; actual arguments that are
C     constants, variables, elements, expressions, and CHARACTER values of other
C     lengths, cut or padded to the dummy's; the value converted to the
C     function's type and length; a value that goes through a dummy procedure
C     and into a routine that changes its argument. Z = SWAP(Z) reads Z's parts
C     after neither is written.
      PROGRAM STMTFN
      EXTERNAL BUMP
      REAL GRADE, TEST1, TEST2, XLAB, R, H, M(3), X, SINH, Y, COMMONV
      INTEGER I, IDX, TWICE, K
      INTEGER*2 N2, NEXT2
      LOGICAL BIG, L
      DOUBLE PRECISION DBL, D
      COMPLEX C, SWAP, W, CVAL
      CHARACTER*10 CSF, A, B
      CHARACTER*4 SHORT
      CHARACTER*3 FIRST3
      CHARACTER*6 PADDED, T6
      COMMON /BLK/ COMMONV
      VOLUME(RADIUS) = 4.189*RADIUS**3
      SINH(X) = (EXP(X)-EXP(-X))*0.5
      AVG(R, H, Y) = (R + H + Y) / 3.0
      TWICE(I) = I*2
      IDX(I) = M(I) * 10.0
      NEXT2(N2) = N2 + 1
      BIG(H) = H .GT. 2.0
      DBL(D) = D*D
      SWAP(W) = CMPLX(AIMAG(W), REAL(W))
      CVAL(R) = R
      CSF(A,B) = A(6:10)//B(1:5)
      SHORT(A) = A
      FIRST3(T6) = T6
      PADDED(R) = 'X'
      FIRSTV(R) = R
      AREA(R) = 3.0*R*R
      RING(R, H) = AREA(R) - AREA(H)
      PUSH(R) = COMMONV + R
      CALLS(R) = BUMP(R)
C     The manual's
      PRINT *, VOLUME(1.0), VOLUME(2.0)
      PRINT *, SINH(0.0)
      TEST1 = 3.0
      TEST2 = 4.0
      XLAB = 5.0
      GRADE = AVG(TEST1, TEST2, XLAB)
      PRINT *, GRADE
      IF (AVG(1.0, 2.0, 3.0) .LT. AVG(4.0, 5.0, 6.0)) PRINT *, 'LESS'
      PRINT *, CSF('ABCDEFGHIJ', 'KLMNOPQRST')
C     The names of dummy arguments, whatever they are elsewhere: H is a REAL
C     variable, I an INTEGER one, and neither is touched
      H = 99.0
      I = 77
      PRINT *, BIG(1.0), BIG(3.0), TWICE(21), H, I
C     An INTEGER function of a REAL element
      M(1) = 1.5
      M(2) = 2.5
      M(3) = 3.5
      K = 2
      PRINT *, IDX(K), IDX(K + 1)
C     Word arguments: a constant that fits, a variable
      N2 = 4
      PRINT *, NEXT2(N2), NEXT2(32)
C     REAL*8, COMPLEX, and a REAL value made COMPLEX
      PRINT *, DBL(1.5D0)
      C = (1.0, 2.0)
      C = SWAP(C)
      PRINT *, C
      PRINT *, SWAP((3.0, 4.0))
      PRINT *, CVAL(7.0)
C     Using those before, one inside another's argument
      PRINT *, RING(2.0, 1.0)
      PRINT *, AREA(AREA(1.0))
      PRINT *, RING(M(1) + 0.5, AREA(1.0) - 2.0)
C     CHARACTER values: the function's length, and the dummy's
      PRINT *, '[', SHORT('ABCDEFGHIJ'), ']'
      PRINT *, '[', SHORT('AB'), ']'
      PRINT *, '[', FIRST3('ABCDEF'), ']'
      PRINT *, '[', FIRST3('AB'), ']'
      PRINT *, '[', CSF('12345', 'VWXYZ'), ']'
      PRINT *, '[', PADDED(1.0), ']'
      PRINT *, LEN(SHORT('X')), LEN(CSF('X', 'Y'))
      A = 'ABCDEFGHIJ'
      A = SHORT(A)
      PRINT *, '[', A, ']'
      PRINT *, '[', SHORT(CSF('ABCDEFGHIJ', 'KLMNOPQRST')) // '!', ']'
C     Variables in the expression, read when it is used
      COMMONV = 1.0
      PRINT *, PUSH(1.0)
      COMMONV = 2.0
      PRINT *, PUSH(1.0)
C     A function called from the expression, changing its argument; a statement
C     function's value is a copy, wherever it goes
      Y = 1.0
      PRINT *, CALLS(Y), Y
      CALL SHOW(AVG(1.0, 2.0, 3.0), SHORT('ABCDEFGHIJ'))
      CALL CHANGE(FIRSTV(Y))
      PRINT *, Y
C     In the conditions of statements
      K = 0
10    IF (TWICE(K) .LT. 6) THEN
        K = K + 1
        GO TO 10
      END IF
      PRINT *, K
      DO WHILE (AREA(REAL(K)) .LT. 100.0)
        K = K + 1
      END DO
      PRINT *, K
      DO 20 K = TWICE(1), TWICE(2), TWICE(1)
        PRINT *, K
20    CONTINUE
      L = BIG(M(1)) .OR. BIG(M(3))
      PRINT *, L
      END

      FUNCTION BUMP(V)
      V = V + 1.0
      BUMP = V * 10.0
      END

      SUBROUTINE SHOW(V, S)
      CHARACTER*(*) S
      PRINT *, V, LEN(S), S
      END

      SUBROUTINE CHANGE(V)
      V = -1.0
      END
