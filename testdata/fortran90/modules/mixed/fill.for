C     fill.for: the fixed-form subroutine of mixed (mixed.f90 says why),
C     called from free form, and including limits.inc too.
      SUBROUTINE FILL(A, N)
      IMPLICIT NONE
      INCLUDE 'limits.inc'
      INTEGER N, A(N), K
      IF (N .GT. NMAX) STOP 'too many'
      DO 10 K = 1, N
         A(K) = K*K
   10 CONTINUE
      END
