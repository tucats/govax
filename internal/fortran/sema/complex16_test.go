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

// TestComplex16NotImpl checks what COMPLEX(16) hasn't yet: the intrinsic
// functions but ABS, SQRT, EXP, LOG, SIN, and COS (Phase 108), and I/O.
func TestComplex16NotImpl(t *testing.T) {
	cases := map[string]string{
		"      H = SUM((/ Q, Q /))":  "an intrinsic function of a COMPLEX(16) argument",
		"      PRINT *, Q":           "I/O of a COMPLEX(16) value",
		"      READ *, I, Q":         "I/O of a COMPLEX(16) value",
		"      WRITE (6) (Q, I=1,2)": "I/O of a COMPLEX(16) value",
	}

	for text, want := range cases {
		lines := append(append([]string{}, complex16Unit...), text, "      END")
		_, ms := analyzeOpts(t, Options{F90: true}, lines...)

		if len(ms) != 1 || ms[0].Ident != "NOTIMPL" || !strings.Contains(ms[0].Text, want) {
			t.Errorf("%s: got %v, want NOTIMPL: %s", text, ms, want)
		}
	}

	// A namelist with a COMPLEX(16) in it.
	lines := append(append([]string{}, complex16Unit...), "      NAMELIST /G/ Q", "      WRITE (6, G)", "      END")
	if _, ms := analyzeOpts(t, Options{F90: true}, lines...); len(ms) != 1 || ms[0].Ident != "NOTIMPL" {
		t.Errorf("a namelist of a COMPLEX(16): got %v", ms)
	}
}
