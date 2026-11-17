C     Fortran 90's declarations (Phase 100, subtask 3): the :: form,
C     attributes, PARAMETER with ::, initial values given with =, a scalar
C     value broadcast to an array, implied SAVE, and the attribute
C     statements with and without ::.
      PROGRAM F90DECL
      INTEGER, PARAMETER :: N = 3, M = N * 2
      CHARACTER*(*), PARAMETER :: NAME = 'XYZ'
      REAL*8, DIMENSION(N), SAVE :: X, Y(2)
      REAL :: A(M) = 1.5
      INTEGER :: K = N + 1, L
      CHARACTER*4 :: C = 'AB', D*2 = 'PQRS'
      LOGICAL, VOLATILE :: FLAG = .TRUE.
      DIMENSION :: Z(N)
      SAVE :: L
      REAL, EXTERNAL :: F
      INTRINSIC :: SQRT
      L = K + LEN(NAME)
      Z(1) = F(SQRT(X(1))) + Y(1) + A(M)
      IF (FLAG) PRINT *, C, D, Z(1)
      END
      SUBROUTINE S(P, Q, R)
      REAL, INTENT(IN) :: P
      REAL, INTENT(INOUT), OPTIONAL :: Q
      INTENT(OUT) R
      OPTIONAL :: R
      REAL, TARGET :: T(4)
      TARGET :: U
      R = P + Q
      T(1) = R
      U = T(1)
      END
