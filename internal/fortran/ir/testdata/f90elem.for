      OPTIONS /F90
C     Elemental intrinsic functions on arrays (Phase 102, subtask 8): one
C     in line, one called from MTHRTL for each element, a scalar argument
C     computed once, ICHAR of a CHARACTER array, and MERGE, numeric and
C     CHARACTER.
      PROGRAM F90ELM
      REAL A(4), B(4)
      INTEGER K(4)
      LOGICAL L(4)
      CHARACTER*2 S(4)
      B = MAX(ABS(A), X * 2.0)
      B = SQRT(A)
      K = ICHAR(S)
      B = MERGE(A, B, A .GT. B)
      S = MERGE(S, 'XX', L)
      END
