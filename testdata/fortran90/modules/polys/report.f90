! A polynomial's degree and values, from a unit outside the modules, which
! has only what its ONLY list names: CREATED by a name of its own.
SUBROUTINE REPORT (P)
  USE POLYNOMIALS, ONLY: POLY, WP, DEGREE, SHOW, OPERATOR (.AT.), MADE => CREATED
  IMPLICIT NONE
  TYPE (POLY), INTENT (IN) :: P
  INTEGER K
  CALL SHOW ('Report', P)
  PRINT 1, DEGREE (P), (P .AT. REAL (K, WP), K = 0, 3)
1 FORMAT (' Degree', I2, '; at 0 to 3:', 4F7.1)
  PRINT 2, MADE
2 FORMAT (' Made so far:', I3)
END SUBROUTINE REPORT
