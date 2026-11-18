      OPTIONS /F90
C     Generic names (Phase 103, subtask 7): each reference becomes a call of
C     the specific procedure its arguments fit, by its own name; a reference
C     no specific fits goes to the intrinsic function the name extends.
      PROGRAM GEN
      INTERFACE SWAP
        SUBROUTINE SWAPR(A, B)
        REAL A, B
        END SUBROUTINE
        SUBROUTINE SWAPI(I, J)
        INTEGER I, J
        END SUBROUTINE
      END INTERFACE
      INTERFACE SQRT
        LOGICAL FUNCTION SQRTL(L)
        LOGICAL L
        END FUNCTION
      END INTERFACE
      REAL X, Y
      INTEGER M, N
      LOGICAL Q
      CALL SWAP(X, Y)
      CALL SWAP(J=M, I=N)
      Q = SQRT(Q)
      Y = SQRT(X)
      END
