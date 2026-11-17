C     Fortran 90's kinds (Phase 100, subtask 4): the selectors of every
C     type, kinds named by PARAMETERs, kinds on constants (a real's digits
C     read at its kind's precision), IMPLICIT with a kind, and a FUNCTION
C     with a kind selector.
      PROGRAM F90KINDS
      IMPLICIT REAL(KIND=8) (D), LOGICAL(1) (L)
      INTEGER, PARAMETER :: DP = 8, SHORT = 2
      INTEGER(SHORT) :: I2 = 7_2
      INTEGER(KIND=4) I4
      REAL(DP) :: X = 1.1_DP, Y = 1.1
      REAL(16) Q
      COMPLEX(KIND=DP) :: Z = (1.0_8, 2)
      COMPLEX(4) C
      LOGICAL(KIND=2) :: FLAG = .TRUE._2
      CHARACTER(LEN=5) S
      CHARACTER(3, 1) T
      CHARACTER(KIND=1, LEN=2) :: U = 1_'AB'
      CHARACTER(10) :: V*4
      CHARACTER(LEN=*), PARAMETER :: NAME = 'KINDS'
      REAL(KIND=8) R8F
      I4 = I2 + 32767_4
      Q = X * 2.5_16
      C = Z
      D = Q
      LTRUE = FLAG
      S = NAME
      T = S
      V = U
      PRINT *, R8F(D), Y, LTRUE, T, V
      END
      REAL(KIND=8) FUNCTION R8F(A)
      REAL(8) A
      R8F = A
      END
      CHARACTER(LEN=*) FUNCTION CF()
      CF = 'X'
      END
