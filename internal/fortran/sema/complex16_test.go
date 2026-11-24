package sema

import (
	"strings"
	"testing"
)

// complex16Unit declares the names TestComplex16's expressions use: Q is a
// COMPLEX(16), Fortran 90's COMPLEX*32 (docs/planning/PHASE-100 - Fortran
// 90 declarations and kinds.md, subtask 7).
var complex16Unit = []string{
	"      SUBROUTINE E",
	"      COMPLEX(16) Q, R",
	"      COMPLEX*16 Z",
	"      COMPLEX C",
	"      REAL*16 H",
	"      REAL X",
	"      INTEGER I",
}

// complex16Value analyzes "R = text" in complex16Unit and returns the
// value's dump and the messages.
func complex16Value(t *testing.T, text string) (string, []string) {
	t.Helper()

	lines := append(append([]string{}, complex16Unit...), "      R = "+text, "      END")
	prog, ms := analyzeOpts(t, Options{F90: true}, lines...)

	var idents []string
	for _, m := range ms {
		idents = append(idents, m.Ident+": "+m.Text)
	}

	a, ok := prog.Units[0].Body[0].(*Assign)
	if !ok {
		return "", idents
	}

	return DumpExpr(a.Value), idents
}

// TestComplex16 checks COMPLEX(16)'s types: constants, mixed operations, the
// in-line intrinsic functions, and folding.
func TestComplex16(t *testing.T) {
	cases := []struct{ text, want string }{
		// A part with the kind 16 makes a COMPLEX*32 constant; 1.0Q0 is
		// still a COMPLEX*16's (VAX FORTRAN's rule).
		{"(1.0_16, 2) / (3, 4)", "(0.44Q0, 0.08Q0)"},
		{"(1.0Q0, 2) / (3, 4)", "(0.4400000000000000022204460492503131Q0, 0.07999999999999999993061106096092772Q0)"},
		// Mixed operations: COMPLEX*32 is highest.
		{"Q + Z", "(+:X Q (COMPLEX*32 Z))"},
		{"Q * H", "(*:X Q (COMPLEX*32 H))"},
		{"Q / I", "(/:X Q (COMPLEX*32 I))"},
		{"Z + H", "(COMPLEX*32 (+:X Z (COMPLEX*16 H)))"},
		// The conversions and parts, in line.
		{"CMPLX(H, X, KIND=16)", "(cmplx:COMPLEX*32 CMPLX H (REAL*16 X))"},
		{"CMPLX(Q)", "(COMPLEX*32 (COMPLEX Q))"},
		{"DCMPLX(Q)", "(COMPLEX*32 (COMPLEX*16 Q))"},
		{"REAL(Q)", "(COMPLEX*32 (REAL*16 Q))"},
		{"AIMAG(Q)", "(COMPLEX*32 (imag:REAL*16 AIMAG Q))"},
		{"CONJG(Q)", "(conjg:COMPLEX*32 CONJG Q)"},
		{"INT(Q)", "(COMPLEX*32 (INTEGER Q))"},
		// Folded as the code computes it.
		{"(1.0_16, 2.0_16) * (3.0_16, 4.0_16)", "(-5.0Q0, 10.0Q0)"},
		{"(1.0_16, 2.0_16) / (3.0_16, 4.0_16)", "(0.44Q0, 0.08Q0)"},
	}

	for _, c := range cases {
		got, ms := complex16Value(t, c.text)
		if len(ms) != 0 || got != c.want {
			t.Errorf("%s: got %s %v, want %s", c.text, got, ms, c.want)
		}
	}
}

// TestComplex16Reductions checks that SUM, PRODUCT, DOT_PRODUCT, and MATMUL
// take COMPLEX(16) values (Phase 113, subtask 2), and that a specific name of
// another COMPLEX type refuses one as any other wrong type (BADARG).
func TestComplex16Reductions(t *testing.T) {
	cases := map[string]string{
		"      H = SUM((/ Q, Q /))":                                   "",
		"      R = PRODUCT((/ Q, R /), MASK=(/ .TRUE., .FALSE. /))":   "",
		"      R = DOT_PRODUCT((/ Q, Q /), (/ Z, Z /))":               "",
		"      CALL S(MATMUL(RESHAPE((/ Q /), (/ 1, 1 /)), (/ Q /)))": "",
		"      H = DREAL(Q)":                                          "BADARG",
		"      R = CSIN(Q)":                                           "BADARG",
	}

	for text, want := range cases {
		lines := append(append([]string{}, complex16Unit...), text, "      END")
		_, ms := analyzeOpts(t, Options{F90: true}, lines...)

		if want == "" && len(ms) != 0 || want != "" && (len(ms) != 1 || ms[0].Ident != want) {
			t.Errorf("%s: got %v, want %q", text, ms, want)
		}
	}
}

// TestComplex16IO checks that COMPLEX(16) items and namelist entities are
// accepted in every kind of statement (Phase 109, subtask 3).
func TestComplex16IO(t *testing.T) {
	for _, text := range []string{
		"      PRINT *, Q",
		"      READ *, I, Q",
		"      WRITE (6) (Q, I=1,2)",
		"      WRITE (6, '(2F10.3)') Q",
		"      NAMELIST /G/ Q\n      WRITE (6, G)",
	} {
		lines := append([]string{}, complex16Unit...)
		lines = append(lines, strings.Split(text, "\n")...)
		lines = append(lines, "      END")

		if _, ms := analyzeOpts(t, Options{F90: true}, lines...); len(ms) != 0 {
			t.Errorf("%s: got %v", text, ms)
		}
	}
}
