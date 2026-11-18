      OPTIONS /F90
C     Array-valued functions (Phase 104, subtask 7a): a function whose value
C     is an array of a shape known when compiling, assigned whole and by
C     element; references through an interface body, from an internal
C     function, and a recursive one's to itself with RESULT; a value in an
C     expression, as an argument, in an output list; a CHARACTER array value.
      FUNCTION SPAN(N)
      INTEGER N, SPAN(3)
      SPAN = N
      SPAN(2) = 2 * N
      END FUNCTION
      OPTIONS /F90
      SUBROUTINE USE(A)
      REAL A(3)
      INTEGER K(3)
      CHARACTER*4 C(2)
      INTERFACE
        FUNCTION SPAN(N)
        INTEGER N, SPAN(3)
        END FUNCTION
      END INTERFACE
      K = SPAN(4) + 1
      A = HALF(A) * 2.0
      PRINT *, SPAN(1), SUM(HALF(A))
      CALL TAKE(HALF(A))
      C = TAGS('AB')
      CONTAINS
        FUNCTION HALF(V)
        REAL V(:), HALF(3)
        HALF = V / 2.0
        END FUNCTION
        FUNCTION TAGS(S)
        CHARACTER*(*) S
        CHARACTER*4 TAGS(2)
        TAGS(1) = S // 'X'
        TAGS(2) = 'Y' // S
        END FUNCTION
      END SUBROUTINE
      OPTIONS /F90
      RECURSIVE FUNCTION STEP(N) RESULT(R)
      INTEGER N, R(2)
      IF (N .LE. 1) THEN
        R = (/ 0, 1 /)
      ELSE
        R = STEP(N - 1)
        R = (/ R(2), R(1) + R(2) /)
      END IF
      END FUNCTION
