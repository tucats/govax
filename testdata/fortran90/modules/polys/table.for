C     Polynomials from a FORTRAN-77 unit in fixed form: USE brings the
C     module's names in all the same, and the unit changes the module's
C     variable as its own. Compiled /NOF90, as a .FOR file is, it has no
C     defined operators (VAX FORTRAN reads A.B.C as a field), so it calls
C     the module's functions.
      SUBROUTINE TABLE (CS, N)
      USE POLYNOMIALS
      INTEGER N, I
      DOUBLE PRECISION CS(N)
      DO 10 I = 1, N
         WRITE (6, 100) I, DEGREE (POLY_OF (CS(1:I)))
   10 CONTINUE
  100 FORMAT (' Degree of the first', I2, ' terms:', I2)
      CALL SHOW ('T', POLY_OF (CS))
      CREATED = CREATED + 100
      END
