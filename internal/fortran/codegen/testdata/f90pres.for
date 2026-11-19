      OPTIONS /F90
C     Pointer results (Phase 107, subtask 6b): a scalar pointer's
C     address comes back in R0 (SP), a CHARACTER one's descriptor in R0
C     and R1 (CP), an array one's descriptor through a hidden first
C     argument, the caller's (AP), each cleared at entry; the caller's
C     references read the target through a pointer of the statement's,
C     and => copies the association.
      FUNCTION SP(V)
      REAL, TARGET :: V(10)
      REAL, POINTER :: SP
      SP => V(3)
      END
      OPTIONS /F90
      FUNCTION CP(S)
      CHARACTER(LEN=8), TARGET :: S
      CHARACTER(LEN=4), POINTER :: CP
      CP => S(2:5)
      END
      OPTIONS /F90
      FUNCTION AP(V, N)
      REAL, TARGET :: V(10)
      REAL, POINTER :: AP(:)
      AP => V(1:N:2)
      IF (N > 9) ALLOCATE (AP(N))
      END
      OPTIONS /F90
      SUBROUTINE USER(V, S, X)
      INTERFACE
      FUNCTION SP(V)
      REAL, TARGET :: V(10)
      REAL, POINTER :: SP
      END FUNCTION
      FUNCTION CP(S)
      CHARACTER(LEN=8), TARGET :: S
      CHARACTER(LEN=4), POINTER :: CP
      END FUNCTION
      FUNCTION AP(V, N)
      REAL, TARGET :: V(10)
      REAL, POINTER :: AP(:)
      END FUNCTION
      END INTERFACE
      REAL, TARGET :: V(10)
      CHARACTER(LEN=8), TARGET :: S
      REAL, POINTER :: P, Q(:)
      CHARACTER(LEN=4), POINTER :: C
      X = SP(V) + SUM(AP(V, 5))
      P => SP(V)
      C => CP(S)
      Q => AP(V, 4)
      X = X + LEN(CP(S) // 'A')
      END
