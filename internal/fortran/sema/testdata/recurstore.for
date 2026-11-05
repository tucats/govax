C     The storage of recursive units (docs/PHASE-89 - reentrant FORTRAN.md,
C     subtask 3): locals in the frame but for those SAVE'd, given initial
C     values, in COMMON, or STATIC; an EQUIVALENCE group static if any member
C     must be; the function's value in the frame; a SAVE with no list; the
C     same names in a unit that isn't recursive, all static.
      RECURSIVE INTEGER FUNCTION KEEP(N)
      STRUCTURE /INIT/
        INTEGER COUNT /0/
      END STRUCTURE
      STRUCTURE /BARE/
        INTEGER COUNT
      END STRUCTURE
      RECORD /INIT/ RI
      RECORD /BARE/ RB
      INTEGER A(10), B, C, D, E, F, G, H, T /7/
      INTEGER EQ1, EQ2, EQ3, EQ4
      SAVE B
      STATIC C
      AUTOMATIC D
      DATA E /1/
      COMMON /BLK/ F
      EQUIVALENCE (EQ1, EQ2), (EQ3, EQ4)
      SAVE EQ4
      DO G = 1, N
        A(G) = G + B + C + D + E + F + H + T
      END DO
      EQ1 = RI.COUNT + RB.COUNT
      EQ3 = EQ2
      UNSET = EQ3
      KEEP = A(1) + UNSET
      IF (N .GT. 0) KEEP = KEEP + KEEP(N - 1)
      END

      RECURSIVE FUNCTION VALUE(N) RESULT(R)
      CHARACTER*8 S
      R = N
      S = 'TEXT'
      IF (N .GT. 0) R = R + VALUE(N - 1)
      RETURN
      ENTRY OTHER(N) RESULT(Q)
      Q = N
      END

      RECURSIVE CHARACTER*8 FUNCTION NAME(N)
      INTEGER K
      K = N
      NAME = 'X'
      IF (K .GT. 0) NAME = NAME(K - 1)
      END

      RECURSIVE SUBROUTINE ALL(N)
      INTEGER I, J
      SAVE
      I = N
      J = I
      IF (N .GT. 0) CALL ALL(N - 1)
      END

      SUBROUTINE PLAIN(N)
      INTEGER I, J
      I = N
      J = I
      END
