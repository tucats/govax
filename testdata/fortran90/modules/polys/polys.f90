! Polynomials end to end: three modules in files of their own (KINDS, then
! POLYNOMIALS using it, then ROOTS using that), a main program that uses
! only ROOTS, and two external subroutines, one in fixed form; the module
! variable CREATED is read and changed from four objects.
PROGRAM POLYS
  USE ROOTS
  IMPLICIT NONE
  TYPE (POLY) P, Q, R
  REAL (WP) ROOT
  INTEGER STEPS
  P = POLY_OF ((/ -2.0_WP, 0.0_WP, 1.0_WP /))          ! X**2 - 2
  Q = POLY_OF ((/ 1.0_WP, 1.0_WP /))                   ! X + 1
  CALL SHOW ('P', P)
  CALL SHOW ('Q', Q)
  CALL SHOW ('P + Q', P + Q)
  CALL SHOW ('P * Q', P * Q)
  CALL SHOW ('2 P', 2.0_WP * P)
  CALL SHOW ('P''', DERIV (P))
  R = P * Q + DERIV (DERIV (P * Q))
  CALL SHOW ('R', R)
  PRINT 1, DEGREE (R), R .AT. 2.0_WP, R .AT. -1.0_WP
1 FORMAT (' Degree', I2, ', R(2) =', F6.1, ', R(-1) =', F6.1)
  ROOT = NEWTON (P, 1.0_WP, ITS = STEPS)
  PRINT 2, ROOT, STEPS
2 FORMAT (' Root of P near 1:', F12.8, ' in', I2, ' steps')
  ROOT = NEWTON (P, -3.0_WP, 1.0E-3_WP)
  PRINT 3, ROOT
3 FORMAT (' Root of P near -3:', F8.3)
  PRINT 4, CREATED
4 FORMAT (' Polynomials made:', I3)
  CALL REPORT (P * P)
  CALL TABLE ((/ 3.0_WP, -1.0_WP, 0.5_WP, 0.0_WP /), 4)
  PRINT 4, CREATED
END PROGRAM POLYS
