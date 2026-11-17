package sema

import "testing"

// TestUnitF90 checks /F90 and OPTIONS's /[NO]F90: each unit records its own, and
// neither OPTIONS qualifier is NOTIMPL (docs/planning/PHASE-99 - Fortran 90.md,
// subtask 2).
func TestUnitF90(t *testing.T) {
	lines := []string{
		"      PROGRAM P", "      END",
		"      OPTIONS /F90", "      SUBROUTINE S", "      END",
		"      OPTIONS /NOF90", "      SUBROUTINE T", "      END",
	}

	cases := []struct {
		opts Options
		want []bool
	}{
		{Options{}, []bool{false, true, false}},
		{Options{F90: true}, []bool{true, true, false}},
	}

	for _, c := range cases {
		prog, messages := analyzeOpts(t, c.opts, lines...)
		if len(messages) != 0 {
			t.Fatalf("F90 %v: messages %v", c.opts.F90, messages)
		}

		for i, u := range prog.Units {
			if u.F90 != c.want[i] {
				t.Errorf("F90 %v: unit %s's F90 is %v, want %v", c.opts.F90, u.Name, u.F90, c.want[i])
			}
		}
	}
}
