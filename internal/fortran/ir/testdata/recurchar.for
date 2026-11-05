C Phase 89, subtask 11: a CHARACTER function whose RESULT holds its value calls
C itself: the call's temporary and descriptor are as long as RESULT's variable
C (the prefix's CHARACTER*12), not the function's name's implicit REAL.
      RECURSIVE CHARACTER*12 FUNCTION F(N) RESULT(S)
      INTEGER N
      CHARACTER*12 R
      IF (N .LE. 0) THEN
         S = 'END'
      ELSE
         R = F(N - 1)
         S = '<' // R
      END IF
      END
