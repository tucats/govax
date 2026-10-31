C     Exponentiation (Phase 78, subtask 7): small constant exponents in
C     line, in the order of the OTS$ routines, and the rest by OTS$ calls,
C     for every type; G_floating's routines in a unit of its own.
      PROGRAM POWERS
      INTEGER I, J, N, IA(3)
      INTEGER*2 I2, J2
      REAL X, Y, A(3)
      DOUBLE PRECISION D
      REAL*16 Q
      COMPLEX C
      COMPLEX*16 Z
C     INTEGER powers in line, exponents from 1 to 8
      I = 3
      PRINT *, I**1, I**2, I**3, I**4
      PRINT *, I**5, I**6, I**7, I**8
C     ... and by OTS$POWJJ: a larger exponent, a negative one, zero
      PRINT *, I**9, I**(-1), I**0, (-I)**3
      DO 10 N = -2, 2
         PRINT *, N, 2**N, (-1)**N, I**N
   10 CONTINUE
C     INTEGER*2: by OTS$POWII, in line in a word (wrapping), and with
C     an INTEGER*4 by OTS$POWJJ
      I2 = 10
      J2 = 3
      PRINT *, I2**J2, I2**2, I2**5, I2**I
C     REAL: in line, by OTS$POWRJ, and to a REAL power by OTS$POWRR
      X = 1.5
      PRINT *, X**2, X**3, X**8
      J = -2
      PRINT *, X**J, X**(-1), X**9
      Y = 0.5
      PRINT *, 4.0**Y, X**1.0, I**Y
C     A base computed once, ** before unary minus, powers in subscripts'
C     arrays and in a condition
      PRINT *, (X + 1.0)**2, -X**2, 2.0**(-X)
      DO 20 J = 1, 3
         A(J) = J
         IA(J) = J**2
   20 CONTINUE
      PRINT *, A(J-1)**3, IA, IA(J-1)**2
      IF (X**2 .GT. 2.0 .AND. IA(2)**3 .EQ. 64) PRINT *, 'BOTH'
C     REAL*8: in line, by OTS$POWDJ and OTS$POWDD, and with a REAL*4 by
C     OTS$POWRD and OTS$POWDR
      D = 1.5D0
      PRINT *, D**3, D**(-1), D**0.5D0
      PRINT *, X**D, D**X
C     REAL*16: in line, by OTS$POWHJ_R3, and by OTS$POWHH_R3
      Q = 2
      PRINT *, Q**3, Q**10, Q**(-3)
      PRINT *, Q**Y
C     COMPLEX: always called, OTS$POWCJ, OTS$POWCC, and their COMPLEX*16
C     forms; COMPLEX*8 to a REAL*8 power is COMPLEX*16
      C = (1.0, 1.0)
      PRINT *, C**2, C**3, C**(-1)
      PRINT *, (C + (3.0, -1.0))**Y, (C + (3.0, -1.0))**(0.5, 0.0)
      Z = (1D0, 1D0)
      PRINT *, Z**2, Z**(-2)
      PRINT *, (C + (3.0, -1.0))**0.5D0
      PRINT *, (Z + (3D0, -1D0))**(0.5D0, 0D0)
      CALL GPOWER(X)
      END

C     The REAL*8 and COMPLEX*16 powers in G_floating: OTS$POWGJ, OTS$POWGG
C     (a REAL*4 with a REAL*8 too, G_floating having no OTS$POWRD), and
C     OTS$POWCGJ_R3 and OTS$POWCGCG_R3.
      OPTIONS /G_FLOATING
      SUBROUTINE GPOWER(X)
      REAL X
      DOUBLE PRECISION G
      COMPLEX*16 Z
      G = 1.5D0
      PRINT *, G**3, G**(-1), G**0.5D0
      PRINT *, X**G, G**X
      Z = (1D0, 1D0)
      PRINT *, Z**2, Z**(-2)
      PRINT *, (Z + (3D0, -1D0))**(0.5D0, 0D0)
      END
