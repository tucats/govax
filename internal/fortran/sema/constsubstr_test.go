package sema

import "testing"

// TestConstantSubstrings checks Fortran 90's substrings of constants (the
// standard's constant subobjects, R701 and R702; found by subtask 8 of
// docs/planning/PHASE-108 - Fortran 90 intrinsic procedures.md): a named
// constant's and an array constant's element's, folded when the bounds are
// constants, read from the constant's datum when they aren't; never a variable;
// and under /NOF90 section 2.2.4's error, as before.
func TestConstantSubstrings(t *testing.T) {
	decls := []string{"CHARACTER*6 C", "CHARACTER*9 M(2)", "INTEGER I",
		"PARAMETER (C = 'ABCDEF', M = (/'January  ', 'February '/))"}

	for _, c := range []struct {
		f90        bool
		stmt, want string // want: the item's dump, or the first message's identifier
	}{
		{true, "PRINT *, C(2:4)", "'BCD'"},
		{true, "PRINT *, M(2)(1:3)", "'Feb'"},
		{true, "PRINT *, C(I:I)", "'ABCDEF'(I:I)"},
		{true, "PRINT *, M(I)(1:3)", "M[I](1:3)"},
		{true, "PRINT *, C(5:4)", "'ABCDEF'(5:4)"},
		{true, "PRINT *, C(2:9)", "SUBRANGE"},
		{true, "C(1:2) = 'XY'", "BADUSE"},
		{true, "M(I)(1:2) = 'XY'", "BADUSE"},
		{false, "PRINT *, C(2:4)", "BADUSE"},
		{false, "PRINT *, M(2)(1:3)", "SUBSTR"},
	} {
		prog, ms := analyzeOpts(t, Options{F90: c.f90}, fixed(append(decls, c.stmt, "END")...)...)

		got := ""
		switch {
		case len(ms) > 0:
			got = ms[0].Ident
		default:
			if p, ok := prog.Units[0].Body[0].(*IO); ok {
				got = DumpExpr(p.Items[0].Value)
			}
		}

		if got != c.want {
			t.Errorf("%s (/F90 %t): got %s (%v), want %s", c.stmt, c.f90, got, ms, c.want)
		}
	}
}
