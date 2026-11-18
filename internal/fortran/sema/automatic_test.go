package sema

import "testing"

// TestAutomatic checks automatic arrays (arrays.go; docs/planning/PHASE-104 -
// Fortran 90 dynamic arrays.md, finding 7): a subprogram's own arrays whose
// bounds use its dummy arguments, its common blocks, its host's variables, and
// the inquiry and elemental functions 7.1.6.2 allows, and the messages for the
// rest.
func TestAutomatic(t *testing.T) {
	sub := func(lines ...string) []string {
		return append(append([]string{"subroutine s(a, n, c)", "integer n", "real a(n, 3)",
			"character(len=*) c"}, lines...), "end subroutine")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"a dummy's value", sub("real w(n)", "w = 1"), ""},
		{"bounds of every kind", sub("real w(-n:n, 2, n*2)", "w(0, 1, 1) = 1"), ""},
		{"common", sub("common /b/ m", "real w(m)", "w = 1"), ""},
		{"SIZE of a dummy", sub("real w(size(a, 1))", "w = 1"), ""},
		{"LBOUND and UBOUND", sub("real w(lbound(a, 1):ubound(a, dim=1))", "w = 1"), ""},
		{"LEN of a dummy", sub("character*1 w(len(c))", "w = 'x'"), ""},
		{"MAX and MOD", sub("real w(max(n, 1), mod(n, 3) + 1)", "w = 1"), ""},
		{"a function", []string{"function f(n)", "real w(n)", "w = 1", "f = w(1)", "end function"}, ""},
		{"a recursive unit", []string{"recursive subroutine r(n)", "real w(n)", "if (n > 0) call r(n - 1)",
			"end subroutine"}, ""},
		{"a SAVE with no list", sub("real w(n)", "save"), ""},
		{"a host's variable", []string{"subroutine h(n)", "call inner", "contains", "subroutine inner",
			"real w(n)", "w = 1", "end subroutine", "end subroutine"}, ""},

		{"in a main program", []string{"program p", "integer n", "real w(n)", "end program"},
			"ADJNOTDUMMY: the bounds of W must be constants: only a dummy argument can be an adjustable array"},
		{"a local in its bounds", sub("integer k", "real w(k)"),
			"BOUNDVAR: K can't be used in an array's bounds: it isn't a dummy argument or in a common block"},
		{"in COMMON", sub("real w(n)", "common /b/ w"),
			"AUTOATTR: W is an automatic array, whose bounds aren't constants, so it can't be in a common block"},
		{"in EQUIVALENCE", sub("real w(n)", "equivalence (w, x)"),
			"AUTOATTR: W is an automatic array, whose bounds aren't constants, so it can't be in an EQUIVALENCE list"},
		{"saved", sub("real w(n)", "save w"),
			"AUTOATTR: W is an automatic array, whose bounds aren't constants, so it can't be saved"},
		{"STATIC", sub("real w(n)", "static w"),
			"AUTOATTR: W is an automatic array, whose bounds aren't constants, so it can't be STATIC"},
		{"in DATA", sub("real w(n)", "data w /1.0/"),
			"AUTOATTR: W is an automatic array, whose bounds aren't constants, so it can't be given an initial value"},
		{"in a NAMELIST group", sub("real w(n)", "namelist /g/ w"),
			"AUTOATTR: W is an automatic array, whose bounds aren't constants, so it can't be in a NAMELIST group"},
		{"SUM in its bounds", sub("real w(sum(a(:, 1)))"),
			"BOUNDREF: SUM can't be used in an array's bounds: arrays and functions aren't allowed there"},
		{"a function of the program's", sub("external g", "real w(g(n))"),
			"BOUNDREF: G can't be used in an array's bounds: arrays and functions aren't allowed there"},
		{"SIZE of another automatic array", sub("real w(n), v(size(w))"),
			"BOUNDREF: W can't be used in an array's bounds: arrays and functions aren't allowed there"},
		{"SIZE of a local of constant bounds", sub("real x(5), v(size(x))", "v = 1"), ""},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
