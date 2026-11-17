C     Overlap in Fortran 90's array assignments (Phase 102, subtask 5):
C     a section read ahead of where it is written runs up, one read behind
C     runs down, one read both ways goes through an array temporary ($An),
C     and an element of the target is computed once before the loops.
      PROGRAM F90OVL
      REAL A(10), B(4, 4)
      CHARACTER*4 C(3)
      A(1:9) = A(2:10)
      A(2:10) = A(1:9)
      B(1:3, 2:4) = B(2:4, 1:3)
      A(2:9) = A(1:8) + A(3:10)
      A = A(1) + A
      C = C(2)
      A(10:1:-1) = A
      END
