package ir

import "testing"

// TestCaseForm holds caseForm to its thresholds (the author's answer 6 of
// docs/planning/PHASE-101 - Fortran 90 control constructs.md), on both sides
// of each.
func TestCaseForm(t *testing.T) {
	cases := []struct {
		tests int
		span  int64
		want  form
	}{
		{4, 16, tableForm},  // 4 slots a test: dense enough
		{4, 17, chainForm},  // one slot more: a chain, too few tests to search
		{3, 3, chainForm},   // too few tests for a table however dense
		{4, 4, tableForm},   // fewest tests a table takes
		{8, 32, tableForm},  // a table before a search
		{8, 33, searchForm}, // too sparse: a search
		{7, 1000, chainForm},
		{8, 1000, searchForm},
		{9, 0, searchForm}, // no bounded ranges at all
		{0, 0, chainForm},
	}

	for _, c := range cases {
		if got := caseForm(c.tests, c.span); got != c.want {
			t.Errorf("caseForm(%d, %d) = %d, want %d", c.tests, c.span, got, c.want)
		}
	}
}
