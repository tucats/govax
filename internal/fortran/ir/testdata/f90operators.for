      OPTIONS /F90
C     Defined operators and assignment (Phase 103, subtask 8): an operation is
C     a call of the function its operands fit, an extended intrinsic operator
C     too, and a defined assignment is CALL S(X, (Y)), the value a copy.
      PROGRAM OPS
      INTERFACE OPERATOR(.SD.)
        REAL FUNCTION SDIST(X, Y)
        REAL, INTENT(IN) :: X, Y
        END FUNCTION
      END INTERFACE
      INTERFACE OPERATOR(+)
        LOGICAL FUNCTION LPLUS(P, Q)
        LOGICAL, INTENT(IN) :: P, Q
        END FUNCTION
      END INTERFACE
      INTERFACE ASSIGNMENT(=)
        SUBROUTINE YESNO(S, L)
        CHARACTER*(*), INTENT(OUT) :: S
        LOGICAL, INTENT(IN) :: L
        END SUBROUTINE
      END INTERFACE
      LOGICAL T
      CHARACTER*3 W
      D = 1.0 + 2.0 .SD. E
      T = T + T
      W = T
      END
