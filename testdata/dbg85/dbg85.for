C DBG85.FOR - the program behind Phase 85's debugger probe (README.md). It
C makes no I/O statement, so its MACRO file needs no run-time library, and
C VMS's MACRO and LINK can build it. Each name has a form the compiler writes.
      PROGRAM DBG85
      REAL X, V(3)
      DOUBLE PRECISION DD
      INTEGER*2 H
      INTEGER I, M(2,3)
      LOGICAL L
      CHARACTER*8 NAME
      COMMON /BLK/ IC, RC
      V(1) = 1.5
      V(2) = 2.5
      V(3) = 3.5
      X = 0.25
      DD = 2.5D0
      H = 7
      I = 11
      L = .TRUE.
      NAME = 'ABCDEFGH'
      IC = 42
      RC = 9.5
      M(2,3) = 23
      CALL SUB85(V, 3, X)
      CALL TXT85(NAME)
      END
      SUBROUTINE SUB85(A, N, TOTAL)
      REAL A(N), TOTAL
      INTEGER I
      TOTAL = 0.0
      DO 10 I = 1, N
         TOTAL = TOTAL + A(I)
 10   CONTINUE
      RETURN
      END
      SUBROUTINE TXT85(S)
      CHARACTER*(*) S
      S(1:1) = 'Z'
      RETURN
      END
