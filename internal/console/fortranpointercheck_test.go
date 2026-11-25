package console

import "testing"

import "github.com/tucats/govax/internal/testproc"

// pointerCheckPrograms are /CHECK=POINTERS's programs (docs/planning/PHASE-110 -
// Fortran 90 compiler usability.md, subtask 6), run as checkPrograms are. A
// pointer's or an allocatable array's test is in the expression that reaches
// the place, so the programs that test ASSOCIATED or ALLOCATED first must run.
// plain is "" where what the program does without the check isn't defined
// (reading or writing at an address near 0).
var pointerCheckPrograms = []checkProgram{
	{
		// A nullified scalar pointer's target written.
		name: "PTRWRITE", qualifier: "/CHECK=POINTERS", checked: "%FOR-F-NOTASSOC", plain: "%SYSTEM-F-ACCVIO",
		source: `
      OPTIONS /F90
      INTEGER, POINTER :: P
      INTEGER, TARGET :: N
      P => N
      NULLIFY (P)
      P = 5
      PRINT *, 'AFTER'
      END`,
	},
	{
		// A pointer never assigned is a nullified one, its target read.
		name: "PTRREAD", qualifier: "/CHECK=POINTERS", checked: "%FOR-F-NOTASSOC", plain: "",
		source: `
      OPTIONS /F90
      REAL, POINTER :: P
      REAL X
      X = P + 1.0
      PRINT *, 'AFTER', X
      END`,
	},
	{
		// A nullified array pointer's element, which /CHECK=BOUNDS alone
		// takes for a subscript out of range.
		name: "PTRARRAY", qualifier: "/CHECK=(BOUNDS,POINTERS)", checked: "%FOR-F-NOTASSOC", plain: "",
		refused: "SUBRNG",
		source: `
      OPTIONS /F90
      REAL, POINTER :: Q(:)
      REAL X
      NULLIFY (Q)
      X = Q(1)
      PRINT *, 'AFTER', X
      END`,
	},
	{
		// The pointer component at the end of a list.
		name: "PTRCOMP", qualifier: "/CHECK=POINTERS", checked: "%FOR-F-NOTASSOC", plain: "",
		source: `
      OPTIONS /F90
      TYPE NODE
         INTEGER V
         TYPE(NODE), POINTER :: NEXT
      END TYPE
      TYPE(NODE), TARGET :: HEAD
      HEAD%V = 1
      NULLIFY (HEAD%NEXT)
      PRINT *, HEAD%NEXT%V
      PRINT *, 'AFTER'
      END`,
	},
	{
		// An allocatable array never allocated, an element written.
		name: "UNALLOC", qualifier: "/CHECK=POINTERS", checked: "%FOR-F-UNALLOCATED", plain: "",
		source: `
      OPTIONS /F90
      REAL, ALLOCATABLE :: A(:)
      N = 2
      A(N) = 1.0
      PRINT *, 'AFTER'
      END`,
	},
	{
		// After DEALLOCATE: without the check SUM reads the freed block.
		name: "FREED", qualifier: "/CHECK=POINTERS", checked: "%FOR-F-UNALLOCATED", plain: "AFTER",
		source: `
      OPTIONS /F90
      REAL, ALLOCATABLE :: A(:)
      ALLOCATE (A(4))
      A = 1.0
      DEALLOCATE (A)
      PRINT *, SUM(A)
      PRINT *, 'AFTER'
      END`,
	},
	{
		// SIZE of an array never allocated, whose bounds mean nothing.
		name: "SIZEOF", qualifier: "/CHECK=POINTERS", checked: "%FOR-F-UNALLOCATED", plain: "AFTER",
		source: `
      OPTIONS /F90
      REAL, ALLOCATABLE :: A(:, :)
      N = SIZE(A)
      PRINT *, 'AFTER', N
      END`,
	},
	{
		// The whole array in an output list.
		name: "WHOLEIO", qualifier: "/CHECK=POINTERS", checked: "%FOR-F-UNALLOCATED", plain: "",
		source: `
      OPTIONS /F90
      INTEGER, ALLOCATABLE :: A(:)
      PRINT *, A
      PRINT *, 'AFTER'
      END`,
	},
	{
		// The whole array to an assumed-shape dummy, by its own descriptor.
		name: "WHOLEARG", qualifier: "/CHECK=POINTERS", checked: "%FOR-F-UNALLOCATED", plain: "AFTER",
		source: `
      OPTIONS /F90
      REAL, ALLOCATABLE :: A(:)
      CALL SHOW (A)
      PRINT *, 'AFTER'
      CONTAINS
      SUBROUTINE SHOW (V)
      REAL V(:)
      PRINT *, SIZE (V)
      END SUBROUTINE
      END`,
	},
	{
		// The second operand of .AND. isn't reached, so neither is its test;
		// and a list walked to its end.
		name: "PTRGUARD", qualifier: "/CHECK=POINTERS", checked: "FINE           6", plain: "FINE           6",
		refused: "NOTASSOC",
		source: `
      OPTIONS /F90
      TYPE NODE
         INTEGER V
         TYPE(NODE), POINTER :: NEXT
      END TYPE
      TYPE(NODE), TARGET :: N1, N2, N3
      TYPE(NODE), POINTER :: Q
      INTEGER, POINTER :: P
      INTEGER S
      NULLIFY (P)
      IF (ASSOCIATED (P) .AND. P .GT. 0) PRINT *, 'WRONG'
      N1%V = 1
      N2%V = 2
      N3%V = 3
      N1%NEXT => N2
      N2%NEXT => N3
      NULLIFY (N3%NEXT)
      S = 0
      Q => N1
      DO WHILE (ASSOCIATED (Q))
         S = S + Q%V
         Q => Q%NEXT
      END DO
      Q => N1
      DO WHILE (ASSOCIATED (Q%NEXT) .AND. Q%V .LT. 9)
         Q => Q%NEXT
      END DO
      IF (Q%V .EQ. 3) PRINT *, 'FINE', S
      END`,
	},
	{
		// ALLOCATED keeps the program from SIZE and the element.
		name: "ALLOCGUARD", qualifier: "/CHECK=POINTERS", checked: "FINE", plain: "FINE",
		refused: "UNALLOCATED",
		source: `
      OPTIONS /F90
      REAL, ALLOCATABLE :: A(:)
      IF (ALLOCATED (A) .AND. SIZE (A) .GT. 0) PRINT *, 'WRONG'
      IF (ALLOCATED (A)) THEN
         IF (A(1) .GT. 0.0) PRINT *, 'WRONG'
      END IF
      PRINT *, 'FINE'
      END`,
	},
	{
		// Pointers and allocatable arrays used as they should be, in a loop
		// whose test the optimizer may move: nothing to report.
		name: "PTRGOOD", qualifier: "/CHECK=POINTERS", checked: "FINE   55.00000", plain: "FINE   55.00000",
		refused: "%FOR-F",
		source: `
      OPTIONS /F90
      REAL, ALLOCATABLE, TARGET :: A(:)
      REAL, POINTER :: P(:), R
      REAL S
      ALLOCATE (A(10))
      DO I = 1, 10
         A(I) = I
      END DO
      P => A(1:10)
      S = 0.0
      DO I = 1, 10
         S = S + P(I)
      END DO
      R => A(10)
      R = R + 0.0
      IF (SIZE (P) .EQ. 10) PRINT *, 'FINE', S
      DEALLOCATE (A)
      END`,
	},
	{
		// OPTIONS takes the check away again.
		name: "PTROPT", qualifier: "/CHECK=ALL", checked: "AFTER", plain: "AFTER",
		refused: "UNALLOCATED",
		source: `
      OPTIONS /F90 /CHECK=NOPOINTERS
      REAL, ALLOCATABLE :: A(:)
      ALLOCATE (A(3))
      DEALLOCATE (A)
      N = SIZE(A)
      PRINT *, 'AFTER'
      END`,
	},
}

// TestFortranCheckPointers compiles and runs each of pointerCheckPrograms, with the
// check and without, under both optimizer settings.
func TestFortranCheckPointers(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	runCheckPrograms(t, pointerCheckPrograms)
}
