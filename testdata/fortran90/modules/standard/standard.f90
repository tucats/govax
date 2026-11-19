! The standard's examples of modules (ISO/IEC 1539:1991, 11.3.3 and
! 12.1.2.2.2), made to run: a subroutine for each, called in the standard's
! order. The modules are in globals.f90, strings.f90, library.f90, and
! renames.f90; the other units that use them in users.f90 and work.f90.

PROGRAM STANDARD
CALL IDENTICAL_COMMON
CALL GLOBAL_DATA
CALL DERIVED_TYPES
CALL GLOBAL_WORK
CALL PROCEDURE_LIBRARY
CALL OPERATOR_EXTENSIONS
CALL RENAME
CALL HOST_A
END PROGRAM STANDARD

! 11.3.3.1: the common blocks, set here and in ADD_TO_TALLY through the
! module, and seen by OLD_TALLY, which declares /TALLY/ itself.
SUBROUTINE IDENTICAL_COMMON
USE MY_COMMON
COUNT = 0; TOTAL = 0; HISTORY = 0.0
LOWEST = 100.0; HIGHEST = -100.0
CALL ADD_TO_TALLY (7)
CALL ADD_TO_TALLY (-2)
CALL ADD_TO_TALLY (12)
PRINT 1, COUNT, TOTAL, LOWEST, HIGHEST
1 FORMAT (' 11.3.3.1: ', I2, ' values, total ', I3, ', from ', F5.1, ' to ', F5.1)
PRINT 2, HISTORY
2 FORMAT (' History:', 5F6.1)
CALL OLD_TALLY
END SUBROUTINE IDENTICAL_COMMON

! 11.3.3.2: some of the data by an ONLY list; MORE_DATA uses the rest.
SUBROUTINE GLOBAL_DATA
USE DATA_MODULE, ONLY: A, B, D
INTEGER K
A = (/ (REAL (K), K = 1, 10) /)
B = SUM (A)
D = (0.0, 0.0)
D (2, 3) = (1.0, -1.0)
CALL MORE_DATA
END SUBROUTINE GLOBAL_DATA

! 11.3.3.3: an array of the type, given to a subroutine that makes the
! dense matrix.
SUBROUTINE DERIVED_TYPES
USE SPARSE
TYPE (NONZERO) ELEMENTS (3)
INTEGER K
ELEMENTS = (/ NONZERO (2.5, 1, 1), NONZERO (-1.0, 2, 3), NONZERO (4.0, 3, 2) /)
PRINT 1, (ELEMENTS (K) % I, ELEMENTS (K) % J, ELEMENTS (K) % A, K = 1, 3)
1 FORMAT (' 11.3.3.3:', 3 (' (', I1, ',', I1, ')', F5.1))
CALL MATRIX_OF (ELEMENTS)
END SUBROUTINE DERIVED_TYPES

! 11.3.3.4: the standard's PROGRAM GLOBAL_WORK, as a subroutine.
SUBROUTINE GLOBAL_WORK
CALL CONFIGURE_ARRAYS   ! Perform the appropriate allocations
CALL COMPUTE            ! Use the arrays in computations
END SUBROUTINE GLOBAL_WORK

! 11.3.3.5: LLS called with argument keywords, in any order.
SUBROUTINE PROCEDURE_LIBRARY
USE LIBRARY_LLS
REAL ABC (2, 3), D (3), XX (3)
INTEGER IFLAG
ABC = RESHAPE ((/ 1.0, -4.0, 2.0, 2.0, -3.0, -1.0 /), (/ 2, 3 /))
CALL LLS (X = ABC, A = D, F = XX, FLAG = IFLAG)
PRINT 1, D, XX, IFLAG
1 FORMAT (' 11.3.3.5: A =', 3F5.1, ', F =', 3F5.1, ', FLAG =', I2)
CALL LLS (FLAG = IFLAG, F = XX, A = D, X = ABC (:, 2:3))
PRINT 2, D (1:2), XX (1:2), IFLAG
2 FORMAT (' Columns 2 and 3: A =', 2F5.1, ', F =', 2F5.1, ', FLAG =', I2)
END SUBROUTINE PROCEDURE_LIBRARY

! 11.3.3.6: // and + on derived types; VARYING and INTERVAL come through
! OPERATORS, which uses VARYING_STRINGS.
SUBROUTINE OPERATOR_EXTENSIONS
USE OPERATORS
TYPE (VARYING) FIRST, SECOND, BOTH
TYPE (INTERVAL) SPAN
FIRST = VARYING (6, 'Module')
SECOND = VARYING (10, ' operators')
BOTH = FIRST // SECOND // VARYING (1, '!')
PRINT 1, BOTH % LENGTH, BOTH % TEXT (1:BOTH % LENGTH)
1 FORMAT (' 11.3.3.6: ', I2, ' characters: ', A)
SPAN = INTERVAL (1.0, 2.5) + INTERVAL (-0.5, 0.5) + INTERVAL (0.0, 1.0)
PRINT 2, SPAN
2 FORMAT (' Interval: [', F4.1, ',', F4.1, ']')
END SUBROUTINE OPERATOR_EXTENSIONS

! 11.3.3.8: the standard's PROGRAM RENAME, as a subroutine.
SUBROUTINE RENAME
USE J; USE K
! Module J's entity JX is accessible under names JX and KX
! Module J's entity JY is accessible under names JY and KY
! Module K's entity KZ is accessible under name KZ
! Module J's entity JZ and K's entity JZ are different entities
! and must not be referenced
JX = 1.0; JY = 2.0; KZ = 3.0
KX = KX + 10.0
PRINT 1, JX, KY, KZ
1 FORMAT (' 11.3.3.8: JX =', F5.1, ', KY =', F4.1, ', KZ =', F4.1)
CALL SEE_JZ
END SUBROUTINE RENAME

! 12.1.2.2.2: the standard's PROGRAM A, as a subroutine, with its comments.
SUBROUTINE HOST_A
USE B; USE C; USE D
BX = 1.0; Q = 2.0; IX = 3; JX = 4
CX = 5.0; DX = 6.0; DY = 7.0; DZ = 8.0
CALL INNER_PROC (-1.0)
PRINT 3, DX, Q
3 FORMAT (' In A: DX =', F5.1, ', Q =', F4.1)
CONTAINS
SUBROUTINE INNER_PROC (Q)
USE C                ! Not needed
USE B, ONLY: BX      ! Entities accessible are BX, IX, and JX
                     ! if no other IX or JX
                     ! is accessible to INNER_PROC
                     ! Q is local to INNER_PROC,
                     ! since Q is a dummy argument
USE D, X => DX       ! Entities accessible are DX, DY, and DZ
                     ! X is local name for DX in INNER_PROC
                     ! X and DX denote same entity if no other
                     ! entity DX is local to INNER_PROC
USE E, ONLY: EX      ! EX is accessible in INNER_PROC, not in program A
                     ! EY and EZ are not accessible in INNER_PROC
                     ! or in program A
USE G                ! FX and GX are accessible in INNER_PROC
FX = 9.0; GX = 10.0; EX = 11.0
PRINT 1, BX, IX, JX, Q, CX
1 FORMAT (' 12.1.2.2.2: BX =', F4.1, ', IX =', I2, ', JX =', I2, ', Q =', F5.1, ', CX =', F4.1)
PRINT 2, X, DY + DZ, EX, FX + GX
2 FORMAT (' X =', F5.1, ', DY + DZ =', F5.1, ', EX =', F5.1, ', FX + GX =', F5.1)
X = X * 2.0
END SUBROUTINE INNER_PROC
END SUBROUTINE HOST_A
