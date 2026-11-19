! 11.3.3.4: the standard's two subroutines on the global allocatable arrays,
! the computation filled in.

SUBROUTINE CONFIGURE_ARRAYS
! Process to set up work arrays
USE WORK_ARRAYS
READ (*, *) N
ALLOCATE (A (N), B (N, N), C (N, N, 2 * N))
END SUBROUTINE CONFIGURE_ARRAYS

SUBROUTINE COMPUTE
USE WORK_ARRAYS
INTEGER K
A = (/ (K, K = 1, N) /)
B = SPREAD (A, DIM = 2, NCOPIES = N)
C (:, :, 1) = B + TRANSPOSE (B)
C (:, :, 2:) = 1.0
PRINT 1, N, SIZE (C), SUM (B), SUM (C (:, :, 1)), SUM (C)
1 FORMAT (' 11.3.3.4: N =', I2, ', SIZE (C) =', I3, ', sums', 3F6.1)
DEALLOCATE (A, B, C)
PRINT '('' Allocated after:'', 3L2)', ALLOCATED (A), ALLOCATED (B), ALLOCATED (C)
END SUBROUTINE COMPUTE
