C     Executable statements and I/O, from the examples of chapters 3,
C     5, 7, and 9 of the VAX FORTRAN Language Reference Manual (1988),
C     and appendix A.
      SUBROUTINE EXEC(X, *, *)
      ASSIGN 10 TO NSTART
      GO TO (12,24,36), INDEX
      GO TO NSTART, (10, 12)
      IF (THETA-CHI) 50,50,100
   10 DO WHILE (X .LT. 5)
          X = X + 1
      END DO
      DO I = 1, 10
          IF (I .EQ. 5) RETURN 1
      END DO
   12 READ (1,1250) J, K, ARRAY(J,K)
 1250 FORMAT (I1,1X,I1,1X,F6.2)
   24 WRITE (6,10) J, K(3), 4, (L+4)/2, N
      WRITE (6, 40) K, L, M, N, O, P
   40 FORMAT (3I6.6/I6,2F8.4)
   36 OPEN (UNIT=1, FILE='DATA.DAT', STATUS='OLD', READONLY)
      READ (1'10, ERR=50) X
      INQUIRE (FILE='DATA.DAT', EXIST=EX)
      CLOSE (1, STATUS='KEEP')
      DEFINE FILE 3 (1000,48,U,NREC)
      FIND (3'1)
      DECODE (12,100,A) K
  100 FORMAT (3I4)
      ENCODE (12,100,B) K(3), K(2), K(1)
      PRINT *, ((ARRAY(I,J), I=1,2), J=1,3)
      TYPE 100, I, J
      PAUSE 'WAIT'
   50 REWIND 5
   60 RETURN 2
      END
