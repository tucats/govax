      PROGRAM OPTFW
C     What forwarding and dead code removal must get right (Phase 84's subtask 5):
C     when a constant stored is still what a place holds, what a removed store
C     still had to do, and what a block that looks unreachable may still be.
      EXTERNAL TALLY
      INTEGER I, J, K, L, M, N, IB, NCOND, VALS(5), KEEP, LAST
      INTEGER SHARED, FAULTS
      INTEGER*2 H(2)
      INTEGER*4 W
      EQUIVALENCE (W, H)
      REAL R, S, RX
      INTEGER RI
      EQUIVALENCE (R, RI)
      REAL*8 D, E
      COMPLEX Z, Y
      LOGICAL P, Q, DEBUG
      CHARACTER*6 T
      CHARACTER*2 G(3)
      COMMON /SH/ SHARED
      COMMON /CONDS/ NCOND
  100 FORMAT (1X, A, 3I8)
  110 FORMAT (1X, A, 2F8.3)
C
C     A store to half of a variable ends what's known of the whole.
      W = 65537
      H(2) = 5
      PRINT 100, 'HALF', W
C
C     A store through another type: the bits, not a conversion.
      RI = 16448
      RX = R
      PRINT 110, 'BITS', RX
C
C     A subscript known by then, on each side of a statement, and a store by a
C     subscript that isn't known, which ends what's known of the whole array.
      I = 2
      VALS(1) = 10
      VALS(2) = 20
      VALS(3) = 30
      VALS(I) = VALS(I+1) + 1
      PRINT 100, 'SUBSCRIPT', VALS(2), VALS(1)
      J = VALS(1) / 5
      VALS(J) = 7
      PRINT 100, 'UNKNOWN', VALS(2), VALS(3)
C
C     Both arms agree, then they don't.
      P = J .EQ. 2
      IF (P) THEN
         K = 4
      ELSE
         K = 4
      END IF
      PRINT 100, 'AGREE', K
      IF (P) THEN
         K = 5
      ELSE
         K = 6
      END IF
      PRINT 100, 'DIFFER', K
C
C     A loop changes what looks constant at its top.
      L = 1
      DO 10 M = 1, 3
         N = L
         L = L + 1
   10 CONTINUE
      PRINT 100, 'LOOP', N, L
C
C     A DO count from a constant, and one of no trips.
      N = 3
      L = 0
      DO 20 M = 1, N
         L = L + M
   20 CONTINUE
      PRINT 100, 'COUNT', L, M
      N = 0
      DO 30 M = 5, N
         L = 99
   30 CONTINUE
      PRINT 100, 'NO TRIPS', L, M
C
C     A call changes COMMON, and a local it was given, but not a local it wasn't.
      SHARED = 1
      K = 2
      L = 3
      CALL BUMP(K)
      PRINT 100, 'CALL', SHARED, K, L
C
C     Branches settled: logical, computed, arithmetic. Computed out of range goes on.
      DEBUG = .FALSE.
      IF (DEBUG) THEN
         PRINT 100, 'NEVER'
      END IF
      Q = .TRUE.
      IF (Q .AND. .NOT. DEBUG) PRINT 100, 'LOGICAL'
      K = 2
      GO TO (41, 42, 43), K
   41 PRINT 100, 'WRONG 41'
   42 PRINT 100, 'COMPUTED 42'
      GO TO 44
   43 PRINT 100, 'WRONG 43'
   44 K = 7
      GO TO (45, 46), K
      PRINT 100, 'OUT OF RANGE'
      GO TO 47
   45 PRINT 100, 'WRONG 45'
   46 PRINT 100, 'WRONG 46'
   47 S = -2.5
      IF (S) 48, 49, 50
   48 PRINT 100, 'ARITHMETIC NEGATIVE'
      GO TO 51
   49 PRINT 100, 'WRONG 49'
   50 PRINT 100, 'WRONG 50'
C
C     A label only an ASSIGN goes to, past a settled branch.
   51 ASSIGN 53 TO IB
      GO TO 52
      PRINT 100, 'WRONG 51'
   52 GO TO IB
      PRINT 100, 'WRONG 52'
   53 PRINT 100, 'ASSIGNED'
C
C     Other types, and character lengths known by then.
      D = 1.5D0
      E = D + D
      Z = (1.0, 2.0)
      Y = Z
      PRINT 110, 'DOUBLE', SNGL(E), REAL(Y)
      N = 3
      T = 'ABCDEF'
      T(N:N) = 'X'
      G(N) = T(N:N+1)
      PRINT 120, T, G(3)
  120 FORMAT (1X, 'TEXT', 1X, A, 1X, A)
      N = 4
      M = 3
      PRINT 100, 'CHARS', LEN(T(N:N+1)), INDEX(T, T(N:M+1))
C
C     A function's value in every shape, and one that keeps the last call's.
      PRINT 100, 'FUNC', IFN1(5), IFN2(5), IFN3(5)
      PRINT 100, 'KEPT', KEEP(.TRUE.), KEEP(.FALSE.)
      LAST = KEEP(.FALSE.)
      PRINT 100, 'KEPT', LAST
C
C     Faults that have to stay: a function's value overwritten before anything
C     reads it, and a reserved operand given as bits, loaded as a number.
      NCOND = 0
      CALL LIB$ESTABLISH(TALLY)
      K = 0
      L = FAULTS(K)
      PRINT 100, 'DIVIDE', NCOND, L
      R = '00008000'X
      S = R
      PRINT 100, 'RESERVED', NCOND
      PRINT 110, 'FIXED', S
      END
C
      SUBROUTINE BUMP(K)
      INTEGER K, SHARED
      COMMON /SH/ SHARED
      SHARED = SHARED + 10
      K = K + 20
      END
C
      INTEGER FUNCTION IFN1(N)
      IFN1 = 3
      END
C
      INTEGER FUNCTION IFN2(N)
      IFN2 = N + 1
      RETURN
      END
C
      INTEGER FUNCTION IFN3(N)
      IFN3 = N
      IFN3 = IFN3 * 2
      IF (N .GT. 0) RETURN
      IFN3 = 0
      END
C
      INTEGER FUNCTION KEEP(L)
      LOGICAL L
      IF (L) KEEP = 7
      END
C
      INTEGER FUNCTION FAULTS(K)
      INTEGER K
      FAULTS = 5/K
      FAULTS = 8
      END
C
      INTEGER FUNCTION TALLY(SIGARGS, MECHARGS)
      INTEGER SIGARGS(*), MECHARGS(*), NCOND, LIB$FIXUP_FLT
      COMMON /CONDS/ NCOND
      INCLUDE '($SSDEF)'
      NCOND = NCOND + 1
      TALLY = SS$_CONTINUE
      IF (SIGARGS(2) .EQ. SS$_ROPRAND) THEN
         TALLY = LIB$FIXUP_FLT(SIGARGS, MECHARGS, 1.5)
      END IF
      END
