package sema

import "testing"

// TestRuntimeLength checks CHARACTER lengths that aren't constant
// (charlength.go; docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md,
// finding 7): a dummy argument's, an automatic object's, in both forms, with
// the names a specification expression may use, and the messages for the
// rest.
func TestRuntimeLength(t *testing.T) {
	sub := func(lines ...string) []string {
		return append(append([]string{"subroutine s(a, n, c, d)", "integer n", "real a(n, 3)",
			"character(len=*) c"}, lines...), "end subroutine")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"a dummy's length", sub("character(len=n) d", "d = 'x'"), ""},
		{"a dummy array's length", sub("character(len=n+1) d(3)", "d(1) = 'x'"), ""},
		{"an automatic scalar", sub("character(len=n) w", "w = c"), ""},
		{"the old form", sub("character*(2*n) w", "w = c"), ""},
		{"an automatic array", sub("character(len=n) w(n, 2)", "w = 'x'"), ""},
		{"an array of run-time length and constant bounds", sub("character(len=n) w(4)", "w(1) = 'x'"), ""},
		{"LEN of a dummy", sub("character(len=len(c)) w", "w = c"), ""},
		{"SIZE of a dummy", sub("character(len=size(a)) w", "w = c"), ""},
		{"common", sub("common /b/ m", "character(len=m) w", "w = c"), ""},
		{"a function", []string{"function f(n)", "character(len=n) w", "w = 'x'", "f = len(w)",
			"end function"}, ""},
		{"a host's variable", []string{"subroutine h(n)", "call inner", "contains", "subroutine inner",
			"character(len=n) w", "w = 'x'", "end subroutine", "end subroutine"}, ""},
		{"a SAVE with no list", sub("character(len=n) w", "save"), ""},
		{"several names", sub("character(len=n) :: w, x*4, y", "w = x // y"), ""},

		{"in a main program", []string{"program p", "integer n", "character(len=n) w", "end program"},
			"LENNOTDUMMY: the CHARACTER length of W must be constant: only a subprogram's names can have a length that isn't"},
		{"a local in it", sub("integer k", "character(len=k) w"),
			"BOUNDVAR: K can't be used in a CHARACTER length: it isn't a dummy argument or in a common block"},
		{"a REAL expression", sub("character(len=n/2.0) w", "w = c"), ""},
		{"a LOGICAL name", sub("logical l", "common /b/ l", "character(len=l) w"),
			"BADBOUND: a CHARACTER length must be an INTEGER or REAL expression"},
		{"a function of the program's", sub("external g", "character(len=g(n)) w"),
			"BOUNDREF: G can't be used in a CHARACTER length: arrays and functions aren't allowed there"},
		{"LEN of an automatic object", sub("character(len=n) w", "character(len=len(w)) v"),
			"BOUNDREF: W can't be used in a CHARACTER length: arrays and functions aren't allowed there"},
		{"in COMMON", sub("character(len=n) w", "common /b/ w"),
			"AUTOLENATTR: W is an automatic object, whose CHARACTER length isn't constant, so it can't be in a common block"},
		{"in EQUIVALENCE", sub("character(len=n) w", "character*4 x", "equivalence (w, x)"),
			"AUTOLENATTR: W is an automatic object, whose CHARACTER length isn't constant, so it can't be in an EQUIVALENCE list"},
		{"saved", sub("character(len=n) w", "save w"),
			"AUTOLENATTR: W is an automatic object, whose CHARACTER length isn't constant, so it can't be saved"},
		{"in DATA", sub("character(len=n) w", "data w /'x'/"),
			"AUTOLENATTR: W is an automatic object, whose CHARACTER length isn't constant, so it can't be given an initial value"},
		{"in a NAMELIST group", sub("character(len=n) w", "namelist /g/ w"),
			"AUTOLENATTR: W is an automatic object, whose CHARACTER length isn't constant, so it can't be in a NAMELIST group"},
		{"an automatic array in COMMON", sub("character(len=n) w(n)", "common /b/ w"),
			"AUTOATTR: W is an automatic array, whose bounds aren't constants, so it can't be in a common block"},
		{"a PARAMETER", sub("character(len=n), parameter :: w = 'abc'"),
			"BADCHARLEN: a CHARACTER length must be an integer constant expression from 1 to 65535, or (*)"},
		{"the function's value", []string{"function f(n)", "character(len=n) f", "f = 'x'", "end function"}, ""},
		{"a function it calls", sub("character(len=n) g", "c = g(1)"),
			"NOTIMPL: a function whose value has a CHARACTER length that isn't constant is not implemented yet"},
		{"IMPLICIT", []string{"subroutine s(n)", "implicit character(len=n) (w)", "w = 'x'", "end subroutine"},
			"NOTIMPL: a CHARACTER length that isn't constant is not implemented yet"},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
