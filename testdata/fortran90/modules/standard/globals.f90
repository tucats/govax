! The standard's examples of modules (ISO/IEC 1539:1991, 11.3.3), made to run:
! the modules that hold only data and types.

! 11.3.3.1, identical common blocks: every unit that uses MY_COMMON has the
! same /TALLY/ and /LIMITS/, declared once.
MODULE MY_COMMON
INTEGER COUNT, TOTAL
REAL HISTORY (5), LOWEST, HIGHEST
COMMON /TALLY/ COUNT, TOTAL, HISTORY
COMMON /LIMITS/ LOWEST, HIGHEST
END MODULE MY_COMMON

! 11.3.3.2, global data.
MODULE DATA_MODULE
SAVE
REAL A (10), B, C (20,20)
INTEGER :: I=0
INTEGER, PARAMETER :: J=10
COMPLEX D (J,J)
END MODULE

! 11.3.3.3, derived types.
MODULE SPARSE
TYPE NONZERO
REAL A
INTEGER I, J
END TYPE
END MODULE

! 11.3.3.4, global allocatable arrays.
MODULE WORK_ARRAYS
! An example set of work arrays
INTEGER N
REAL, ALLOCATABLE, SAVE :: A (:), B (:, :), C (:, :, :)
END MODULE WORK_ARRAYS
