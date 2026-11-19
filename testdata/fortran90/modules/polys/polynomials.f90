! Polynomials as a data abstraction (the standard's 11.3.3.7): a type whose
! components are private, made by POLY_OF and the operators, read by DEGREE,
! .AT., and SHOW. The module is PRIVATE by default; its PUBLIC statements
! name what its users may have, KINDS's WP and MAXDEG among them.
MODULE POLYNOMIALS
  USE KINDS
  IMPLICIT NONE
  PRIVATE
  PUBLIC :: WP, MAXDEG, POLY, POLY_OF, DEGREE, DERIV, SHOW, CREATED
  PUBLIC :: OPERATOR (+), OPERATOR (*), OPERATOR (.AT.)

  TYPE POLY
    PRIVATE
    INTEGER N                     ! the degree; -1 for the zero polynomial
    REAL (WP) C (0:MAXDEG)        ! C(K) is the coefficient of X**K
  END TYPE POLY

  INTEGER :: CREATED = 0          ! the polynomials made so far

  INTERFACE OPERATOR (+)
    MODULE PROCEDURE ADD
  END INTERFACE
  INTERFACE OPERATOR (*)
    MODULE PROCEDURE MULTIPLY, SCALE
  END INTERFACE
  INTERFACE OPERATOR (.AT.)
    MODULE PROCEDURE VALUE_AT
  END INTERFACE

CONTAINS

  ! A polynomial from its coefficients, lowest first.
  FUNCTION POLY_OF (COEFFS) RESULT (P)
    REAL (WP), INTENT (IN) :: COEFFS (:)
    TYPE (POLY) P
    P % C = 0
    P % C (0:SIZE (COEFFS) - 1) = COEFFS
    CALL SETTLE (P, SIZE (COEFFS) - 1)
  END FUNCTION POLY_OF

  ! P's degree is the highest K from TOP down whose coefficient isn't zero;
  ! each polynomial made is settled once, and counted.
  SUBROUTINE SETTLE (P, TOP)
    TYPE (POLY), INTENT (INOUT) :: P
    INTEGER, INTENT (IN) :: TOP
    INTEGER K
    P % N = -1
    DO K = TOP, 0, -1
      IF (P % C (K) /= 0) THEN
        P % N = K
        EXIT
      END IF
    END DO
    CREATED = CREATED + 1
  END SUBROUTINE SETTLE

  FUNCTION ADD (P, Q) RESULT (R)
    TYPE (POLY), INTENT (IN) :: P, Q
    TYPE (POLY) R
    R % C = P % C + Q % C
    CALL SETTLE (R, MAX (P % N, Q % N, 0))
  END FUNCTION ADD

  FUNCTION MULTIPLY (P, Q) RESULT (R)
    TYPE (POLY), INTENT (IN) :: P, Q
    TYPE (POLY) R
    INTEGER I
    R % C = 0
    DO I = 0, P % N
      R % C (I:I + Q % N) = R % C (I:I + Q % N) + P % C (I) * Q % C (0:Q % N)
    END DO
    CALL SETTLE (R, MAX (P % N + Q % N, 0))
  END FUNCTION MULTIPLY

  FUNCTION SCALE (A, P) RESULT (R)
    REAL (WP), INTENT (IN) :: A
    TYPE (POLY), INTENT (IN) :: P
    TYPE (POLY) R
    R % C = A * P % C
    CALL SETTLE (R, MAX (P % N, 0))
  END FUNCTION SCALE

  ! P's value at X, by Horner's rule.
  FUNCTION VALUE_AT (P, X) RESULT (V)
    TYPE (POLY), INTENT (IN) :: P
    REAL (WP), INTENT (IN) :: X
    REAL (WP) V
    INTEGER K
    V = 0
    DO K = P % N, 0, -1
      V = V * X + P % C (K)
    END DO
  END FUNCTION VALUE_AT

  FUNCTION DERIV (P) RESULT (D)
    TYPE (POLY), INTENT (IN) :: P
    TYPE (POLY) D
    INTEGER K
    D % C = 0
    DO K = 1, P % N
      D % C (K - 1) = K * P % C (K)
    END DO
    CALL SETTLE (D, MAX (P % N - 1, 0))
  END FUNCTION DERIV

  INTEGER FUNCTION DEGREE (P)
    TYPE (POLY), INTENT (IN) :: P
    DEGREE = P % N
  END FUNCTION DEGREE

  ! P's terms, highest first, each coefficient with its sign.
  SUBROUTINE SHOW (LABEL, P)
    CHARACTER (LEN = *), INTENT (IN) :: LABEL
    TYPE (POLY), INTENT (IN) :: P
    INTEGER K
    IF (P % N < 0) THEN
      PRINT '(1X, A, '' = 0'')', LABEL
    ELSE
      PRINT 1, LABEL, (P % C (K), K, K = P % N, 0, -1)
    END IF
1   FORMAT (1X, A, ' =', 9 (SP, F6.1, SS, ' X**', I1))
  END SUBROUTINE SHOW
END MODULE POLYNOMIALS
