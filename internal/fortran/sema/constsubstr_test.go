package sema

import (
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
)

// TestConstantSubstrings checks Fortran 90's substrings of constants (the
// standard's constant subobjects, R701 and R702; found by subtask 8 of
// docs/planning/PHASE-108 - Fortran 90 intrinsic procedures.md): a named
// constant's and an array constant's element's, folded when the bounds are
// constants, read from the constant's datum when they aren't; never a variable;
// and under /NOF90 section 2.2.4's error, as before. A literal constant's
// substring is the same, under /F90.
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

// TestLiteralSubstrings checks a literal constant's substring, 'ABCDEF'(2:4)
// (R609's parent string a scalar constant; Phase 111, subtask 8a, found by
// fcalc): folded when its bounds are constants, read from the constant's datum
// when they aren't, checked against the constant's length. The parser makes
// one only under /F90; under /NOF90 the parenthesis ends the statement, as
// before.
func TestLiteralSubstrings(t *testing.T) {
	for _, c := range []struct {
		f90        bool
		stmt, want string // want: the item's dump, or the first message's identifier
	}{
		{true, "PRINT *, 'ABCDEF'(2:4)", "'BCD'"},
		{true, "PRINT *, 'ABCDEF'(:2)", "'AB'"},
		{true, "PRINT *, 'ABCDEF'(I:I)", "'ABCDEF'(I:I)"},
		{true, "PRINT *, 'ABC'(2:9)", "SUBRANGE"},
		{true, "PRINT *, 'ABC'(2)", "EXTRATEXT"},
		{false, "PRINT *, 'ABCDEF'(2:4)", "EXTRATEXT"},
	} {
		lines := []string{"INTEGER I", c.stmt, "END"}

		var (
			prog *Program
			ms   []diag.Message
		)

		if c.f90 {
			prog, ms = analyzeFreeWith(t, Options{F90: true}, lines...)
		} else {
			prog, ms = analyzeOpts(t, Options{}, fixed(lines...)...)
		}

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
