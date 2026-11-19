      OPTIONS /F90
C     Fortran 90's array pointers (Phase 107, subtask 3): a pointer's
C     cell is a class NCA descriptor; => to a whole array keeps its
C     bounds, to a section starts them at 1, to a pointer copies its
C     descriptor, and to the pointer's own section goes through a
C     cell of the statement's; an element is reached through every
C     stride; ASSOCIATED compares POINTER, extents, and strides; a
C     recursive unit's array pointer is in the frame, cleared at entry.
      PROGRAM F90APTR
      INTEGER, TARGET :: A(0:9), M(4, 5)
      INTEGER, POINTER :: P(:), Q(:, :), R(:)
      P => A
      Q => M(2:3, ::2)
      R => P
      P => P(9:1:-1)
      P(2) = Q(1, 2)
      NULLIFY (R)
      IF (ASSOCIATED(P, A(1:3)) .OR. ASSOCIATED(R)) A = P
      END
      OPTIONS /F90
      RECURSIVE SUBROUTINE S(V)
      REAL, TARGET :: V(10)
      REAL, POINTER :: W(:)
      IF (.NOT. ASSOCIATED(W)) W => V(::2)
      W = 0
      END
