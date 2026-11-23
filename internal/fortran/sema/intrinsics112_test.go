package sema

import (
	"strings"
	"testing"
)

// TestPhase112Intrinsics checks Fortran 95's changes to the intrinsic
// procedures (docs/planning/PHASE-112 - Fortran 95.md, subtask 11) that aren't
// the array functions' (arrayfuncs_test.go has MAXLOC's and MINLOC's DIM=):
// CEILING's and FLOOR's KIND=, folded and not, and CALL CPU_TIME. Each case is
// a main program's lines, compiled /F90, and what the dump must hold, or the
// first message, "IDENT: text" (the text's start).
func TestPhase112Intrinsics(t *testing.T) {
	cases := []struct {
		lines []string
		want  string
	}{
		// Folded in the kind asked for, past the default integer's range
		// under /NOI4 too.
		{[]string{"INTEGER(4), PARAMETER :: K = CEILING(70000.5, KIND=4)", "PRINT *, K"}, "70001"},
		{[]string{"PRINT *, KIND(FLOOR(2.5, KIND=1))"}, "1"},
		{[]string{"PRINT *, KIND(CEILING(2.5, KIND=2)), CEILING(2.5, KIND=2)"}, "PRINT * 2 3"},
		{[]string{"REAL X", "PRINT *, FLOOR(X, KIND=1)"}, "(INTEGER*1 (floor:"},
		{[]string{"REAL X", "PRINT *, CEILING(X, KIND=3)"}, "NOSUCHKIND: INTEGER has no kind 3"},
		{[]string{"REAL X", "INTEGER N", "PRINT *, CEILING(X, KIND=N)"}, "KINDNOTCONST"},

		{[]string{"REAL T", "CALL CPU_TIME(T)"}, "FTN$CPU_TIME_F"},
		{[]string{"DOUBLE PRECISION T", "CALL CPU_TIME(TIME=T)"}, "FTN$CPU_TIME_D"},
		{[]string{"REAL*16 T", "CALL CPU_TIME(T)"}, "FTN$CPU_TIME_H"},
		{[]string{"INTEGER T", "CALL CPU_TIME(T)"}, "SYSARG: "},
		{[]string{"REAL T(2)", "CALL CPU_TIME(T)"}, "SYSARG: "},
		{[]string{"CALL CPU_TIME(1.0)"}, "SYSARG: "},
		{[]string{"CALL CPU_TIME()"}, "INTRINSUBARGS: "},
	}

	for _, c := range cases {
		for _, i4 := range []bool{true, false} {
			prog, ms := analyzeOpts(t, Options{F90: true, NoI4: !i4}, fixed(append(c.lines, "END")...)...)

			got := ""
			if len(ms) > 0 {
				got = ms[0].Ident + ": " + ms[0].Text
			} else {
				got = Dump(prog)
			}

			if !strings.Contains(got, c.want) {
				t.Errorf("%v (/I4 %t): got\n%s\nwant %s", c.lines, i4, got, c.want)
			}
		}
	}
}
