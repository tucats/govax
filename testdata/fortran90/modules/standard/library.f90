! 11.3.3.5, procedure libraries: the interface of the external subroutine LLS
! gathered into a module, so its callers may use argument keywords. And
! 11.3.3.6's interfaces for the operators, in a module of their own.

MODULE LIBRARY_LLS
INTERFACE
SUBROUTINE LLS (X, A, F, FLAG)
REAL X (:, :)
! The SIZE in the next statement is an intrinsic function
REAL, DIMENSION (SIZE (X, 2)) :: A, F
INTEGER FLAG
END SUBROUTINE LLS
END INTERFACE
END MODULE LIBRARY_LLS

MODULE OPERATORS
USE VARYING_STRINGS
INTERFACE OPERATOR (//)
FUNCTION JOIN (X, Y)
USE VARYING_STRINGS
TYPE (VARYING) JOIN
TYPE (VARYING), INTENT (IN) :: X, Y
END FUNCTION JOIN
END INTERFACE
INTERFACE OPERATOR (+)
FUNCTION WIDEN (X, Y)
USE VARYING_STRINGS
TYPE (INTERVAL) WIDEN
TYPE (INTERVAL), INTENT (IN) :: X, Y
END FUNCTION WIDEN
END INTERFACE
END MODULE OPERATORS

! LLS itself: A is each column's sum of X, F its largest absolute value, and
! FLAG the number of columns whose sum is negative.
SUBROUTINE LLS (X, A, F, FLAG)
REAL X (:, :)
REAL, DIMENSION (SIZE (X, 2)) :: A, F
INTEGER FLAG
A = SUM (X, DIM = 1)
F = MAXVAL (ABS (X), DIM = 1)
FLAG = COUNT (A < 0.0)
END SUBROUTINE LLS
