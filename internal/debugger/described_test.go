package debugger_test

import (
	"strings"
	"testing"
)

// TestFortranDescribedArrays holds EXAMINE of Fortran 90's variables whose own
// storage is a descriptor (docs/planning/PHASE-104 - Fortran 90 dynamic
// arrays.md, finding 14): an allocatable array (class NCA, contiguous), an
// assumed-shape dummy given a section (its class NCA copy, whose strides say
// where each element is), an automatic array (class A), and an automatic
// CHARACTER scalar (class S); and an allocatable array not allocated.
func TestFortranDescribedArrays(t *testing.T) {
	const source = `      OPTIONS /F90
      PROGRAM DA
      INTEGER, ALLOCATABLE :: Q(:, :), U(:)
      INTEGER M(4, 3), I, J
      INTERFACE
        SUBROUTINE SHOW(A, N)
        INTEGER A(:, :), N
        END SUBROUTINE
      END INTERFACE
      DO J = 1, 3
        DO I = 1, 4
          M(I, J) = 10 * I + J
        END DO
      END DO
      ALLOCATE (Q(0:1, 2))
      Q = 7
      Q(1, 2) = 9
      CALL SHOW(M(2:4:2, 1:3:2), 2)
      END

      SUBROUTINE SHOW(A, N)
      INTEGER A(:, :), N
      INTEGER W(N)
      CHARACTER(LEN=N) S
      W = (/ 5, 6 /)
      S = 'XY'
   10 CONTINUE
      END
`
	c := fortranSessionOf(t, source, "SHOW.L10")

	dispatch(t, c, "SET RADIX DECIMAL")

	for _, step := range []struct{ line, output string }{
		// The section's elements, rows 2 and 4 of columns 1 and 3, by its strides.
		{"EXAMINE A", "DA\\SHOW.A(1,1):  21\nDA\\SHOW.A(2,1):  41\nDA\\SHOW.A(1,2):  23\nDA\\SHOW.A(2,2):  43\n"},
		{"EXAMINE A(2,2)", "DA\\SHOW.A(2,2):  43\n"},
		{"EXAMINE W", "DA\\SHOW.W(1):  5\nDA\\SHOW.W(2):  6\n"},
		{"EXAMINE S", "DA\\SHOW.S:  'XY'\n"},
		{"EXAMINE S(2:2)", "DA\\SHOW.S(2:2):  'Y'\n"},
		{"EXAMINE DA.Q", "DA\\DA.Q(0,1):  7\nDA\\DA.Q(1,1):  7\nDA\\DA.Q(0,2):  7\nDA\\DA.Q(1,2):  9\n"},
		{"EXAMINE DA.Q(1,2)", "DA\\DA.Q(1,2):  9\n"},
	} {
		if got := variableOutput(t, c, step.line); got != step.output {
			t.Errorf("%s: got %q, want %q", step.line, got, step.output)
		}
	}

	for _, want := range []struct{ line, says string }{
		{"EXAMINE DA.U", "DA.U is not allocated"},
		{"EXAMINE A(3,1)", "subscript 1 is out of bounds, value is 3, bounds are 1:2"},
		{"EXAMINE DA.Q(2,1)", "subscript 1 is out of bounds, value is 2, bounds are 0:1"},
	} {
		if _, err := sayErr(c, want.line); err == nil || !strings.Contains(err.Error(), want.says) {
			t.Errorf("%s: %v, want %q", want.line, err, want.says)
		}
	}
}
