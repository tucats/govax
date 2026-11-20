package fortran_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/sema"
)

// Fused nests in the MACRO file (docs/planning/PHASE-110 - Fortran 90 compiler
// usability.md, subtask 8): under /OPTIMIZE two array assignments that may share one
// nest of loops have one, as do a WHERE construct's arms; without it, or under
// /CHECK=BOUNDS, each has its own. testdata/fortran90's optfuse.f90 runs them.

const fusion = `subroutine fu(a, b, c, n)
  integer n
  real a(n), b(n), c(n)
  a = b + 1.0
  c = a * 2.0
  where (a > 2.0)
    b = a
  elsewhere
    b = 0.0
  end where
end subroutine
`

func TestFusedNests(t *testing.T) {
	for _, c := range []struct {
		optimize, bounds bool
		loops            int
	}{
		{false, false, 4},
		{true, false, 2},
		{true, true, 4},
	} {
		r := compileFree("FU", fusion, nil, func(o *fortran.Options) {
			o.Optimize = c.optimize
			o.Checks = sema.Checks{Bounds: c.bounds}
		})
		if r.Errors > 0 {
			t.Fatal(messageList(r))
		}

		text := strings.Join(r.MACRO, "\n")
		if got := strings.Count(text, "dimension 1's body"); got != c.loops {
			t.Errorf("optimize %v, bounds %v: %d loops, want %d:\n%s", c.optimize, c.bounds, got, c.loops, text)
		}
	}
}
