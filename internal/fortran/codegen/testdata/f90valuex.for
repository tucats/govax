      OPTIONS /F90
C     Array-valued functions (Phase 104, subtask 7b): a value whose bounds
C     are known only when the program runs, made by the function on the heap
C     and described in the caller's class NCA descriptor; a lower bound that
C     isn't 1; bounds from SIZE of an assumed-shape dummy; a recursive
C     function's reference to itself; references in an assignment, a
C     reduction, an output list, as an argument, and in a CALL with
C     alternate returns.
      FUNCTION RAMP(N)
      INTEGER N, I
      REAL RAMP(0:N)
      DO I = 0, N
        RAMP(I) = I
      END DO
      END FUNCTION
      OPTIONS /F90
      SUBROUTINE USE(A, N)
      INTEGER N
      REAL A(N)
      INTERFACE
        FUNCTION RAMP(N)
        INTEGER N
        REAL RAMP(0:N)
        END FUNCTION
      END INTERFACE
      A(1:N) = RAMP(N - 1)
      PRINT *, SUM(RAMP(N)), DOUBLE(A)
      CALL TAKE(RAMP(2), *10)
   10 CONTINUE
      CONTAINS
        FUNCTION DOUBLE(V)
        REAL V(:), DOUBLE(SIZE(V))
        DOUBLE = 2.0 * V
        END FUNCTION
      END SUBROUTINE
      OPTIONS /F90
      RECURSIVE FUNCTION TRI(N) RESULT(R)
      INTEGER N, R(N)
      R(N) = N
      IF (N > 1) R(1:N - 1) = TRI(N - 1)
      END FUNCTION
