C     STOP from a subroutine with a display of digits, kept as written;
C     RETURN and END in a subroutine and a function, a function's RETURN
C     giving the value last assigned to its name (sections 5.5, 5.9, 5.10).
      PROGRAM STOPS
      INTEGER N, CLAMP
      N = 3
      CALL BUMP(N)
      PRINT *, N
      N = 20
      CALL BUMP(N)
      PRINT *, N
      PRINT *, CLAMP(5), CLAMP(50)
      CALL QUIT(N)
      PRINT *, 'NOT REACHED'
      END

      SUBROUTINE BUMP(K)
      INTEGER K
      IF (K .GT. 10) RETURN
      K = K + 1
      END

      INTEGER FUNCTION CLAMP(K)
      INTEGER K
      CLAMP = K
      IF (K .LE. 10) RETURN
      CLAMP = 10
      END

      SUBROUTINE QUIT(K)
      INTEGER K
      PRINT *, K
      STOP 00098
      END
