      OPTIONS /F90
C     Array temporaries of a size known only when the program runs (Phase
C     104, subtask 4c): an assignment through one, a CHARACTER one of a
C     length known only then, a WHERE's mask kept in one, a constructor's,
C     the target's own vector subscript copied, a CHARACTER function's value
C     computed once, and MERGE of lengths known only then.
      SUBROUTINE RTEMP(A, N, C, IV, F)
      INTEGER :: N, IV(N)
      REAL :: A(N)
      CHARACTER(LEN=*) :: C(N)
      CHARACTER(LEN=4) :: F
      A(N:1:-1) = A
      C(2:N) = C(1:N-1)
      WHERE (A > 0.0)
        A = -A
      ELSEWHERE
        A = 1.0
      END WHERE
      A = A + (/ (REAL(I), I = 1, N) /)
      IV(IV) = 0
      C = F(N) // C
      C = MERGE(C, C(N:1:-1), A > 0.0)
      END
