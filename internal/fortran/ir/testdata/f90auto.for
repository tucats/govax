      OPTIONS /F90
C     Fortran 90's automatic arrays (Phase 104, subtask 4a): a subprogram's
C     own arrays whose bounds aren't constants, computed at each entry from
C     dummy arguments, common, and the inquiry functions of 7.1.6.2.
      SUBROUTINE AUTO(A, N, C)
      INTEGER :: N, M
      COMMON /SIZES/ M
      REAL :: A(N, 3)
      CHARACTER(LEN=*) :: C
      REAL :: W(N), V(SIZE(A, 1), 2), X(0:LEN(C)), Y(MAX(N, M))
      INTEGER W2(UBOUND(A, DIM=2))
      W = 1.0
      V = 2.0
      X(0) = 3.0
      Y = 4.0
      W2 = 5
      PRINT *, SIZE(W), SIZE(V), SIZE(X), SIZE(Y), W(1), W2
      END
