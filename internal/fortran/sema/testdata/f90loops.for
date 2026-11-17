C     Fortran 90's construct names, DO with no loop control, EXIT, and
C     CYCLE (Phase 101, subtask 2), in fixed form, where blanks don't
C     count: EXITOUTER is EXIT OUTER.
      PROGRAM F90LOOPS
      INTEGER I, J, K, N
      N = 0
C     A counted loop: CYCLE skips to the step, EXIT leaves.
      DO I = 1, 10
        IF (I .EQ. 2) CYCLE
        IF (I .EQ. 8) EXIT
        N = N + I
      END DO
C     Named loops: EXIT OUTER leaves both, CYCLE OUTER goes on with I.
      OUTER: DO I = 1, 3
        INNER: DO J = 1, 3
          IF (J .EQ. 2) CYCLEOUTER
          IF (I .EQ. 3) THEN
            EXIT OUTER
          END IF
          N = N + 10
        END DO INNER
      END DO OUTER
C     A labeled terminal statement: CYCLE skips it.
      DO 10 I = 1, 4
        IF (I .EQ. 3) CYCLE
   10 N = N + 100
C     A labeled END DO: CYCLE goes to its block.
      DO 20 I = 1, 2
        IF (N .GT. 0) CYCLE
        N = 0
   20 END DO
C     DO WHILE: CYCLE goes to the test.
      K = 0
      DO WHILE (K .LT. 5)
        K = K + 1
        IF (K .EQ. 3) CYCLE
        N = N + 1000
      END DO
C     No loop control: only EXIT leaves.
      K = 0
      FOREVER: DO
        K = K + 1
        IF (K .GE. 4) EXIT FOREVER
      END DO FOREVER
      PRINT *, N, K
      END
