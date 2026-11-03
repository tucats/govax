C     NOF77.FOR - /NOF77, for one unit, by an OPTIONS statement: a DO whose count is
C     zero runs once, OPEN's defaults, and appendix A.6's EXTERNAL. The manual's example
C     is the model for the last. The units after it have FORTRAN-77's rules.
      OPTIONS /NOF77
      PROGRAM NOF77
      EXTERNAL SIN, *TAN, SINDEG
      N = 0
      DO 10 I = 1, N
        PRINT 100, 'VARIABLE LIMIT', I
 10   CONTINUE
      PRINT 100, 'I AFTER', I
      DO 20 I = 1, 0
        PRINT 100, 'CONSTANT LIMIT', I
 20   CONTINUE
      PRINT 100, 'I AFTER', I
      DO 30 I = 3, 1, -1
        PRINT 100, 'A REAL LOOP', I
 30   CONTINUE
      DO I = 5, 4
        PRINT 100, 'END DO', I
      END DO
      OPEN (1, FILE='NOF77.TMP')
      WRITE (1, 110) 1, 2
      REWIND 1
      READ (1, 120) K
      PRINT 100, 'BLANKS ARE ZEROS', K
      CLOSE (1, STATUS='DELETE')
      CALL TRIG(0.0, SIN, X)
      PRINT 130, 'SIN', X
      CALL TRIG(0.0, TAN, X)
      PRINT 130, 'THE PROGRAM''S TAN', X
      CALL TRIG(0.0, SINDEG, X)
      PRINT 130, 'SINDEG', X
      CALL RULES
100   FORMAT (1X, A, I4)
110   FORMAT (I1, 1X, I1)
120   FORMAT (I3)
130   FORMAT (1X, A, F6.2)
      END
      SUBROUTINE TRIG(X, F, Y)
      Y = F(X)
      RETURN
      END
      FUNCTION TAN(X)
      TAN = 5.0
      RETURN
      END
      FUNCTION SINDEG(X)
      SINDEG = 7.0
      RETURN
      END
C     The OPTIONS statement above is that unit's alone.
      SUBROUTINE RULES
      N = 0
      DO 10 I = 1, N
        PRINT *, 'THIS MUST NOT PRINT'
 10   CONTINUE
      OPEN (1, FILE='NOF77.TMP', STATUS='NEW')
      WRITE (1, 110) 1, 2
      REWIND 1
      READ (1, 120) K
      PRINT 100, 'BLANKS ARE IGNORED', K
      CLOSE (1, STATUS='DELETE')
100   FORMAT (1X, A, I4)
110   FORMAT (I1, 1X, I1)
120   FORMAT (I3)
      END
