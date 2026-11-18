      OPTIONS /F90
C     CHARACTER lengths that aren't constant (Phase 104, subtask 4b): a dummy
C     argument's, kept in a cell; an automatic scalar's, in its class S
C     descriptor; an automatic array's, in its class A descriptor's first word.
      SUBROUTINE AUTOLEN(S, N, C)
      INTEGER :: N
      CHARACTER(LEN=N) :: S
      CHARACTER(LEN=*) :: C
      CHARACTER(LEN=LEN(C)+1) :: T
      CHARACTER*(N) :: W(N)
      T = C
      S = T
      W(1) = S
      PRINT *, LEN(S), LEN(T), LEN(W), T, W(1)
      CALL SHOW(T, S)
      END
