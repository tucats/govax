C     Recursive routines (docs/PHASE-89 - reentrant FORTRAN.md, subtask 2):
C     RECURSIVE before and after the type, a function's name as a call, RESULT
C     on FUNCTION and ENTRY, entry points called, and OPTIONS /RECURSIVE.
      PROGRAM RECUR
      INTEGER FACT, FIB, ACK
      PRINT *, FACT(5), FIB(10), ACK(2, 3)
      CALL WALK(4)
      END

      RECURSIVE INTEGER FUNCTION FACT(N)
      FACT = 1
      IF (N .GT. 1) FACT = N * FACT(N - 1)
      END

      INTEGER RECURSIVE FUNCTION FIB(N) RESULT(F)
      IF (N .LT. 2) THEN
        F = N
      ELSE
        F = FIB(N - 1) + FIB(N - 2)
      END IF
      RETURN
      ENTRY LUCAS(N) RESULT(L)
      L = 2
      IF (N .GT. 0) L = FIB(N - 1) + FIB(N + 1)
      END

      OPTIONS /RECURSIVE
      INTEGER FUNCTION ACK(M, N)
      IF (M .EQ. 0) THEN
        ACK = N + 1
      ELSE IF (N .EQ. 0) THEN
        ACK = ACK(M - 1, 1)
      ELSE
        ACK = ACK(M - 1, ACK(M, N - 1))
      END IF
      END

      RECURSIVE SUBROUTINE WALK(N)
      IF (N .GT. 0) CALL STEP(N)
      RETURN
      ENTRY STEP(K)
      PRINT *, K
      CALL WALK(K - 1)
      END
