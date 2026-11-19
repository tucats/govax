! Newton's method on the polynomials. ROOTS uses POLYNOMIALS whole and is
! PUBLIC by default, so its users have POLY, the operators, and the rest
! through it, as well as NEWTON.
MODULE ROOTS
  USE POLYNOMIALS
  IMPLICIT NONE
CONTAINS
  ! A root of P from X0, to a step smaller than TOL (1E-10 if TOL isn't
  ! given); ITS, if it's given, the steps taken.
  FUNCTION NEWTON (P, X0, TOL, ITS) RESULT (X)
    TYPE (POLY), INTENT (IN) :: P
    REAL (WP), INTENT (IN) :: X0
    REAL (WP), INTENT (IN), OPTIONAL :: TOL
    INTEGER, INTENT (OUT), OPTIONAL :: ITS
    REAL (WP) X, DX, LIMIT
    TYPE (POLY) DP
    INTEGER I
    LIMIT = 1.0E-10_WP
    IF (PRESENT (TOL)) LIMIT = TOL
    DP = DERIV (P)
    X = X0
    DO I = 1, 50
      DX = (P .AT. X) / (DP .AT. X)
      X = X - DX
      IF (ABS (DX) < LIMIT) EXIT
    END DO
    IF (PRESENT (ITS)) ITS = I
  END FUNCTION NEWTON
END MODULE ROOTS
