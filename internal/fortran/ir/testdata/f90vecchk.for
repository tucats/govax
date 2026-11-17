      OPTIONS /CHECK=BOUNDS
C     Vector subscripts under /CHECK=BOUNDS (Phase 102, subtask 6): each of a
C     vector's values is checked where it is used; the vector's own section
C     once, before the loops.
      PROGRAM F90VCK
      REAL A(10), B(3)
      INTEGER IV(5)
      B = A(IV(2:4))
      END
