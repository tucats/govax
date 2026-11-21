package sema

import "testing"

// TestComplexParts checks the parts of a COMPLEX*16 (docs/planning/PHASE-111 -
// Fortran 90 milestone.md, subtask 2a): under /F90, REAL and AIMAG of one are
// REAL*8, the standard's (fortran90Readings); under /NOF90 REAL of one is
// Table D-3's REAL*4 and AIMAG takes only a COMPLEX*8. And CMPLX of constants
// with KIND= is computed in that kind, not in COMPLEX*8 first.
func TestComplexParts(t *testing.T) {
	lines := []string{
		"      SUBROUTINE S(Z, Q)",
		"      COMPLEX*16 Z, Q",
		"      DOUBLE PRECISION, PARAMETER :: THIRD = 1D0/3D0",
		"      COMPLEX*16, PARAMETER :: C = CMPLX(THIRD, 2D0, 8)",
		"      COMPLEX*16, PARAMETER :: C4 = CMPLX(THIRD, 2D0)",
		"      Q = REAL(Z)",
		"      END",
	}

	for _, f90 := range []bool{false, true} {
		prog, ms := analyzeOpts(t, Options{F90: f90}, lines...)
		if len(ms) != 0 {
			t.Fatalf("/F90 %t: %v", f90, messageText(ms))
		}

		u := prog.Units[0]

		want := "REAL"
		if f90 {
			want = "REAL*8"
		}

		// Q = REAL(Z) is REAL(Z) converted to COMPLEX*16.
		assign := u.Body[0].(*Assign)
		if part := assign.Value.(*Convert).X.Type().String(); part != want {
			t.Errorf("/F90 %t: REAL(Z) is %s, want %s", f90, part, want)
		}

		third := u.Lookup("THIRD").Value.Real
		if c := u.Lookup("C").Value; c.Real.Cmp(third) != 0 {
			t.Errorf("/F90 %t: CMPLX(THIRD, 2D0, 8)'s real part is %v, want %v", f90, c.Real, third)
		}

		// Without KIND=, the parts are REAL*4's: not THIRD.
		if c4 := u.Lookup("C4").Value; c4.Real.Cmp(third) == 0 {
			t.Errorf("/F90 %t: CMPLX(THIRD, 2D0) kept REAL*8's digits", f90)
		}
	}

	aimag := []string{"      SUBROUTINE S(Z, X)", "      COMPLEX*16 Z", "      X = AIMAG(Z)", "      END"}

	if _, ms := analyzeOpts(t, Options{F90: true}, aimag...); len(ms) != 0 {
		t.Errorf("/F90: AIMAG of a COMPLEX*16: %s", messageText(ms))
	}

	if _, ms := analyzeOpts(t, Options{}, aimag...); len(ms) != 1 || ms[0].Ident != "BADARG" {
		t.Errorf("/NOF90: AIMAG of a COMPLEX*16: %s, want BADARG", messageText(ms))
	}
}
