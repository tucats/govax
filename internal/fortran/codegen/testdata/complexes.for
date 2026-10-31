C     COMPLEX values split into their parts (Phase 78, subtask 4).
      PROGRAM CPLX
      COMPLEX C, D, A(4), CF
      COMPLEX*16 Z, W
      REAL X
      C = (1.0, 2.0)
      D = C
      C = C*D
      A(I) = C + X
      A(I+1) = A(I)/D
      Z = C
      W = Z*Z
      W = W/Z - 1
      X = C
      IF (C .EQ. D) X = 1.0
      IF (C .NE. (0.0, 1.0)) X = 2.0
      CALL S(C, C*D, A(I), (C))
      D = CF(-C)
      END
      COMPLEX FUNCTION CF(V)
      COMPLEX V(*)
      CF = V(2)*V(N)
      END
      OPTIONS /G_FLOATING
      SUBROUTINE GS(Z)
      COMPLEX*16 Z
      Z = Z*Z
      END
