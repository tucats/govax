! Units that use the standard's modules from a file of their own.

! 11.3.3.1: one value added to the tally, through the module's common blocks.
SUBROUTINE ADD_TO_TALLY (K)
USE MY_COMMON
INTEGER K
COUNT = COUNT + 1
TOTAL = TOTAL + K
HISTORY (COUNT) = K
LOWEST = MIN (LOWEST, REAL (K))
HIGHEST = MAX (HIGHEST, REAL (K))
END SUBROUTINE ADD_TO_TALLY

! The same common block, declared the FORTRAN-77 way without the module.
SUBROUTINE OLD_TALLY
COMMON /TALLY/ N, NSUM, H (5)
PRINT '('' Seen by OLD_TALLY:'', I3, I4, F6.1)', N, NSUM, H (N)
END SUBROUTINE OLD_TALLY

! 11.3.3.2: access to all of the data.
SUBROUTINE MORE_DATA
USE DATA_MODULE
I = I + 1
C = 0.0
C (J, J) = B
PRINT 1, I, J, B, C (10, 10), D (2, 3)
1 FORMAT (' 11.3.3.2: I =', I2, ', J =', I3, ', B =', F6.1, ', C(10,10) =', F6.1, &
          ', D(2,3) = (', F4.1, ',', F5.1, ')')
CALL RENAMED_DATA
END SUBROUTINE MORE_DATA

! Access to all of it with some renaming to avoid name conflicts: this unit's
! own A is not the module's.
SUBROUTINE RENAMED_DATA
USE DATA_MODULE, AMODULE => A, DMODULE => D
REAL A
A = AMODULE (10) - AMODULE (1)
PRINT 1, A, DMODULE (2, 3) + DMODULE (3, 2), I
1 FORMAT (' Renamed: A =', F5.1, ', D sum = (', F4.1, ',', F5.1, '), I =', I2)
END SUBROUTINE RENAMED_DATA

! 11.3.3.3: the dense matrix the nonzero elements describe.
SUBROUTINE MATRIX_OF (E)
USE SPARSE
TYPE (NONZERO) E (3)
REAL M (3, 3)
INTEGER K
M = 0.0
DO K = 1, 3
  M (E (K) % I, E (K) % J) = E (K) % A
END DO
PRINT 1, (M (K, :), K = 1, 3)
1 FORMAT (' Dense:', 3F5.1)
END SUBROUTINE MATRIX_OF

! 11.3.3.8: the two JZs, each by a name of its own.
SUBROUTINE SEE_JZ
USE J, ONLY: JZ
USE K, ONLY: KJZ => JZ
JZ = 5.0; KJZ = -5.0
PRINT 1, JZ, KJZ
1 FORMAT (' The two JZs:', 2F5.1)
END SUBROUTINE SEE_JZ
