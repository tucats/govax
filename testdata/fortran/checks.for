C     /CHECK with nothing to find (Phase 85, subtask 5): every kind of subscript and
C     substring at its bounds and between them, arithmetic up to its limits, and small
C     floating values, in units compiled with the checks, none of which may trip.
      OPTIONS /CHECK=ALL
      PROGRAM CHECKS
      STRUCTURE /S/
         INTEGER F(2)
         CHARACTER*3 T
      END STRUCTURE
      RECORD /S/ R(2)
      INTEGER A(3), B(0:2,-1:1), V(4), I, J, N, K
      CHARACTER*5 S, TX(2)
      REAL X
      DATA A /1, 2, 3/
  100 FORMAT (1X, A, 3I6)
  200 FORMAT (1X, A, 5(1X, A))
C
C     Arrays, at their bounds and with a constant in the subscript.
      I = 1
      J = 3
      PRINT 100, 'ENDS', A(I), A(J), A(I+1)
      DO 10 I = 0, 2
         DO 10 J = -1, 1
            B(I,J) = 10*I + J
   10 CONTINUE
      I = 2
      J = -1
      PRINT 100, 'TWO', B(2,1), B(I,J), B(I-2,J+2)
C
C     Substrings, every form.
      S = 'ABCDE'
      I = 2
      J = 4
      PRINT 200, 'SUB', S(I:J), S(I:), S(:J), S(I:I), S(5:5)
      TX(2) = 'FGHIJ'
      PRINT 200, 'ELEM', TX(I)(I:J), TX(I)(J:)
      S(I:J) = 'XYZ'
      PRINT 200, 'SET', S
C
C     Records and their fields.
      I = 2
      J = 1
      R(I).F(J) = 5
      R(I).T = 'PQR'
      PRINT 100, 'REC', R(I).F(J)
      PRINT 200, 'FLD', R(I).T(I:I)
C
C     Each kind of dummy array, and a passed-length string.
      CALL ADJ(V, 4)
      CALL ASZ(V, 3)
      CALL OLD(V, 2)
      PRINT 100, 'DUMMY', V(1), V(2), V(3)
      PRINT 100, 'DUMMY', V(4)
      CALL TEXT('LMNOP', 5)
C
C     Arithmetic up to its limits, and what isn't arithmetic.
      N = 2147483646
      N = N + 1
      PRINT '(1X, A, I12)', 'MOST', N
      N = -2147483647
      N = N - 1
      PRINT '(1X, A, I12)', 'LEAST', N
      I = -1
      PRINT '(1X, A, I12)', 'SHIFT', ISHFT(I, 4)
      K = 31
      PRINT '(1X, A, I12)', 'BIT', IBSET(0, K)
C
C     A small value, which isn't an underflow.
      X = 1.0E-10
      X = X*X
      IF (X .GT. 0.0) PRINT 200, 'SMALL'
      END
C
      OPTIONS /CHECK=BOUNDS
      SUBROUTINE ADJ(X, N)
      INTEGER N, X(N), I
      DO 10 I = 1, N
         X(I) = 10*I
   10 CONTINUE
      END
C
      OPTIONS /CHECK=BOUNDS
      SUBROUTINE ASZ(X, I)
      INTEGER X(*), I
      X(I) = X(I) + 1
      END
C
      OPTIONS /CHECK=BOUNDS
      SUBROUTINE OLD(X, I)
      INTEGER X(1), I
      X(I) = X(I) + 2
      END
C
      OPTIONS /CHECK=BOUNDS
      SUBROUTINE TEXT(S, N)
      CHARACTER*(*) S
      INTEGER N
      PRINT 100, 'TEXT', S(1:N), S(N:), S(N:N)
  100 FORMAT (1X, A, 1X, A, 1X, A, 1X, A)
      END
