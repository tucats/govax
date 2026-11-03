C     POWBITS - the exact bits of what VMS's OTS$ power routines and a few of
C     MTHRTL's complex functions return, in hex, for testdata/ftn86's second
C     round: FTN86.COM's first found 1.5**(-2) printing differently on VMS, and
C     no order of operations govax tried fits VMS's other values.
C
      PROGRAM POWBITS
      REAL X, XS(6), F
      DOUBLE PRECISION D
      REAL*16 Q
      COMPLEX C, CS(3)
      INTEGER J, K, IF, ID(2), IQ(4), IC(2)
      EQUIVALENCE (F, IF), (D, ID), (Q, IQ), (C, IC)
      DATA XS /1.5, 3.0, 7.0, 0.1, 0.6666667, 1.1/
      DATA CS /(3.0, 4.0), (1.0, 1.0), (0.5, -2.5)/
C
      DO 20 K = 1, 6
         X = XS(K)
         DO 10 J = -8, 9
            IF (J .EQ. 0) GO TO 10
            F = X ** J
            WRITE (6, 100) 'F', K, J, IF
            D = DBLE(X) ** J
            WRITE (6, 110) 'D', K, J, ID
            Q = QEXT(X) ** J
            WRITE (6, 120) 'H', K, J, IQ
   10    CONTINUE
   20 CONTINUE
C
      DO 30 K = 1, 3
         C = CSIN(CS(K))
         WRITE (6, 110) 'CSIN', K, 0, IC
         C = CCOS(CS(K))
         WRITE (6, 110) 'CCOS', K, 0, IC
         C = CEXP(CS(K))
         WRITE (6, 110) 'CEXP', K, 0, IC
         C = CLOG(CS(K))
         WRITE (6, 110) 'CLOG', K, 0, IC
   30 CONTINUE
      CALL GBITS
      STOP
  100 FORMAT (1X, A4, 2I4, 1X, Z8.8)
  110 FORMAT (1X, A4, 2I4, 2(1X, Z8.8))
  120 FORMAT (1X, A4, 2I4, 4(1X, Z8.8))
      END
C
      OPTIONS /G_FLOATING
      SUBROUTINE GBITS
      DOUBLE PRECISION G
      REAL XS(6)
      INTEGER J, K, IG(2)
      EQUIVALENCE (G, IG)
      DATA XS /1.5, 3.0, 7.0, 0.1, 0.6666667, 1.1/
      DO 20 K = 1, 6
         DO 10 J = -8, 9
            IF (J .EQ. 0) GO TO 10
            G = DBLE(XS(K)) ** J
            WRITE (6, 100) 'G', K, J, IG
   10    CONTINUE
   20 CONTINUE
  100 FORMAT (1X, A4, 2I4, 2(1X, Z8.8))
      END
