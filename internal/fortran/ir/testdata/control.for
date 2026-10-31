C     Phase 79, subtask 4: STOP and PAUSE with and without a display,
C     PAUSE in a logical IF, and RETURN and END in a subprogram.
      PROGRAM CONTROL
      INTEGER N
      N = 0
      PAUSE
      PAUSE 'CHECK THE TAPE'
      PAUSE 00123
      CALL BUMP(N)
      IF (N .GT. 0) PAUSE 'N IS POSITIVE'
      DO 20 I = 1, 2
   20 PAUSE 'IN THE LOOP'
      DO 30 I = 1, 2
         N = N + I
   30 STOP 'LOOP END'
      IF (N .GT. 5) STOP 'TOO BIG'
      IF (N .LT. 0) STOP 98
      STOP
      END

      SUBROUTINE BUMP(K)
      INTEGER K
      K = K + 1
      IF (K .GT. 10) RETURN
      K = K + 1
      END
