C Recursive routines (Phase 89, subtask 4): each activation with storage of its
C own. A function recursing by its own name and by RESULT, mutual recursion
C through ENTRY, an adjustable array sorted in place, CHARACTER, REAL*16, and
C COMPLEX*16 values through hidden arguments, locals and DO loops that must
C survive the calls inside them, a statement function, alternate returns, a
C SAVE'd local and COMMON that every activation shares, a deep recursion, and
C an array in each frame.
      PROGRAM RECURS
      INTEGER FACTOR, ACK, FIB, ISUM, NCALLS, NTEST, I, N
      INTEGER A(12), P(4)
      LOGICAL ISEVEN, ISODD
      CHARACTER*20 REV
      REAL*16 ONES
      COMPLEX*16 CPOW, Z
      DOUBLE PRECISION FRAMES
      COMMON /CNT/ NCALLS, NTEST
      DATA A /50, 3, 91, 17, 8, 64, 3, 22, 77, 1, 45, 30/

      WRITE (6, 100) (I, FACTOR(I), I = 1, 12, 11)
  100 FORMAT (' FACTOR(', I2, ') =', I10)
      WRITE (6, 110) ACK(2, 3), ACK(3, 3)
  110 FORMAT (' ACK(2,3) =', I3, '   ACK(3,3) =', I3)
      WRITE (6, 120) ISEVEN(10), ISODD(10), ISEVEN(7), ISODD(7)
  120 FORMAT (' EVEN/ODD 10:', 2L2, '   7:', 2L2)

      CALL QSORT(A, 1, 12, 12)
      WRITE (6, 130) A
  130 FORMAT (' SORTED:', 12I3)

      WRITE (6, 140) REV('ABCDEF'), REV('X')
  140 FORMAT (' REVERSED: "', A, '" "', A, '"')
      WRITE (6, 150) ONES(10), ONES(100)
  150 FORMAT (' ONES:', F6.0, F33.0)
      Z = CPOW((0.0D0, 1.0D0), 7)
      WRITE (6, 160) Z
  160 FORMAT (' I**7: (', F4.1, ',', F5.1, ')')

      WRITE (6, 170) FIB(20)
  170 FORMAT (' FIB(20) =', I6)
      DO 10 I = 1, 4
         P(I) = I
   10 CONTINUE
      N = 0
      CALL PERMS(P, 1, 3, N)
      WRITE (6, 180) N
  180 FORMAT (' PERMUTATIONS:', I3)

      CALL FIND(5, *20)
      WRITE (6, *) 'FIND RETURNED NORMALLY'
      GO TO 30
   20 WRITE (6, 190) NTEST
  190 FORMAT (' FIND TOOK THE ALTERNATE RETURN AT DEPTH', I2)

   30 CALL COUNT(5, N)
      WRITE (6, 200) N
  200 FORMAT (' SAVED:', I4)
      NCALLS = 0
      CALL DOWN(3)
      WRITE (6, 210) NCALLS
  210 FORMAT (' COMMON:', I3)

      WRITE (6, 220) ISUM(10000)
  220 FORMAT (' SUM TO 10000 =', I10)
      WRITE (6, 230) FRAMES(10)
  230 FORMAT (' FRAMES:', F8.1)
      END

C Without RESULT, the function's name with an argument list is a call.
      RECURSIVE INTEGER FUNCTION FACTOR(N)
      IF (N .LE. 1) THEN
         FACTOR = 1
      ELSE
         FACTOR = N * FACTOR(N - 1)
      END IF
      END

C With RESULT; a call as an argument of a call.
      RECURSIVE FUNCTION ACK(M, N) RESULT(R)
      INTEGER M, N, R
      IF (M .EQ. 0) THEN
         R = N + 1
      ELSE IF (N .EQ. 0) THEN
         R = ACK(M - 1, 1)
      ELSE
         R = ACK(M - 1, ACK(M, N - 1))
      END IF
      END

C Each entry point calls the other: two values in one place.
      RECURSIVE LOGICAL FUNCTION ISEVEN(N)
      LOGICAL ISODD
      IF (N .EQ. 0) THEN
         ISEVEN = .TRUE.
      ELSE
         ISEVEN = ISODD(N - 1)
      END IF
      RETURN
      ENTRY ISODD(N)
      IF (N .EQ. 0) THEN
         ISODD = .FALSE.
      ELSE
         ISODD = ISEVEN(N - 1)
      END IF
      END

C An adjustable array; I, J, P, and the DO loop's count are this activation's.
      RECURSIVE SUBROUTINE QSORT(A, LO, HI, N)
      INTEGER N, A(N), LO, HI, P, I, J, T
      IF (LO .GE. HI) RETURN
      P = A(HI)
      I = LO - 1
      DO 10 J = LO, HI - 1
         IF (A(J) .LE. P) THEN
            I = I + 1
            T = A(I)
            A(I) = A(J)
            A(J) = T
         END IF
   10 CONTINUE
      T = A(I + 1)
      A(I + 1) = A(HI)
      A(HI) = T
      CALL QSORT(A, LO, I, N)
      CALL QSORT(A, I + 2, HI, N)
      END

C A CHARACTER value through a hidden argument, a passed-length dummy, and a
C concatenation.
      RECURSIVE CHARACTER*20 FUNCTION REV(S)
      CHARACTER*(*) S
      CHARACTER*20 T
      IF (LEN(S) .LE. 1) THEN
         REV = S
      ELSE
         T = REV(S(2:))
         REV = T(1:LEN(S)-1) // S(1:1)
      END IF
      END

C REAL*16 through a hidden argument: 2**N - 1, all ones.
      RECURSIVE REAL*16 FUNCTION ONES(N)
      IF (N .EQ. 0) THEN
         ONES = 0
      ELSE
         ONES = 2 * ONES(N - 1) + 1
      END IF
      END

C COMPLEX*16 through a hidden argument.
      RECURSIVE COMPLEX*16 FUNCTION CPOW(Z, N)
      COMPLEX*16 Z
      IF (N .EQ. 0) THEN
         CPOW = (1.0D0, 0.0D0)
      ELSE
         CPOW = Z * CPOW(Z, N - 1)
      END IF
      END

C A must outlive the second call.
      RECURSIVE INTEGER FUNCTION FIB(N)
      INTEGER A
      IF (N .LT. 2) THEN
         FIB = N
      ELSE
         A = FIB(N - 1)
         FIB = A + FIB(N - 2)
      END IF
      END

C Every permutation of P(K..N), printed from the deepest activation; the DO
C loop's index and count belong to each activation.
      RECURSIVE SUBROUTINE PERMS(P, K, N, NSEEN)
      INTEGER P(*), K, N, NSEEN, I, T
      IF (K .EQ. N) THEN
         NSEEN = NSEEN + 1
         WRITE (6, 100) (P(I), I = 1, N)
  100    FORMAT ('   ', 4I2)
         RETURN
      END IF
      DO 10 I = K, N
         T = P(K)
         P(K) = P(I)
         P(I) = T
         CALL PERMS(P, K + 1, N, NSEEN)
         T = P(K)
         P(K) = P(I)
         P(I) = T
   10 CONTINUE
      END

C Alternate returns passed back up; a statement function.
      RECURSIVE SUBROUTINE FIND(N, *)
      INTEGER NCALLS, NTEST, HALF
      COMMON /CNT/ NCALLS, NTEST
      HALF(K) = K / 2
      IF (N .EQ. HALF(4)) THEN
         NTEST = N
         RETURN 1
      END IF
      CALL FIND(N - 1, *10)
      RETURN
   10 RETURN 1
      END

C A SAVE'd local is one place, shared by every activation: M is this one's.
      RECURSIVE SUBROUTINE COUNT(K, N)
      INTEGER SEEN
      SAVE SEEN
      DATA SEEN /0/
      SEEN = SEEN + 1
      M = SEEN
      IF (K .GT. 1) CALL COUNT(K - 1, N)
      IF (K .EQ. 5) N = M * 100 + SEEN
      END

C COMMON is shared too; L is this activation's.
      RECURSIVE SUBROUTINE DOWN(K)
      INTEGER NCALLS, NTEST
      COMMON /CNT/ NCALLS, NTEST
      NCALLS = NCALLS + 1
      L = NCALLS
      IF (K .GT. 0) CALL DOWN(K - 1)
      IF (L .NE. NCALLS - K) WRITE (6, *) 'DOWN: BAD', K, L, NCALLS
      END

C Ten thousand activations.
      RECURSIVE INTEGER FUNCTION ISUM(N)
      IF (N .EQ. 0) THEN
         ISUM = 0
      ELSE
         ISUM = N + ISUM(N - 1)
      END IF
      END

C An array of 200 REAL*8s in each frame, checked after the call below it.
      RECURSIVE DOUBLE PRECISION FUNCTION FRAMES(N)
      DOUBLE PRECISION W(200), INNER
      DO 10 I = 1, 200
         W(I) = N
   10 CONTINUE
      INNER = 0
      IF (N .GT. 1) INNER = FRAMES(N - 1)
      DO 20 I = 1, 200
         IF (W(I) .NE. N) WRITE (6, *) 'FRAMES: BAD', N, I, W(I)
   20 CONTINUE
      FRAMES = INNER + W(200)
      END
