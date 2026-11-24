package sema

import "testing"

// TestBoundFromComponent checks a bound or CHARACTER length that reads a
// component of a dummy argument, D%N (the standard's 7.1.6.2: a subobject of a
// dummy is a specification expression's primary), which
// testdata/fortran90/sparse.f90's function value Y(A%N) found refused (docs/
// planning/PHASE-107 - Fortran 90 pointers.md, subtask 8): the base is checked
// as a bound's name is, the component's type once it is known.
func TestBoundFromComponent(t *testing.T) {
	// sub is a subroutine of the dummies D, of the type SHAPE, and K, with
	// lines between its declarations and its END.
	sub := func(lines ...string) []string {
		head := []string{"subroutine s(d, k)", "type shape", "integer :: n, dims(2)", "real :: x",
			"character(len=3) :: c", "type(shape), pointer :: next", "end type shape",
			"type(shape) :: d", "integer :: k"}

		return append(append(head, lines...), "end subroutine")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"an automatic array", sub("real :: a(d%n)", "a = 0.0"), ""},
		{"both bounds and an expression", sub("real :: a(-d%n:d%n * 2, k)", "a = 0.0"), ""},
		{"a component's element", sub("real :: a(d%dims(k))", "a = 0.0"), ""},
		{"a REAL component", sub("real :: a(d%x)", "a = 0.0"), ""},
		{"through a pointer component", sub("real :: a(d%next%n)", "a = 0.0"), ""},
		{"a CHARACTER length", sub("character(len=d%n) :: w", "w = 'x'"), ""},
		{"a CHARACTER component", sub("real :: a(d%c)", "a = 0.0"),
			"BADBOUND: an array's bound must be an INTEGER or REAL expression"},
		{"a local's component", sub("type(shape) :: e", "real :: a(e%n)", "a = 0.0"),
			"BOUNDVAR: E can't be used in an array's bounds: it isn't a dummy argument or in a common block"},
		{"a component of an element (Phase 113)", []string{"subroutine s(d, k)", "type shape", "integer :: n", "end type shape",
			"type(shape) :: d(3)", "integer :: k", "real :: a(d(1)%n, d(k)%n)", "a = 0.0", "end subroutine"}, ""},
		{"a component of a local's element", []string{"subroutine s(d)", "type shape", "integer :: n", "end type shape",
			"type(shape) :: d(3), e(2)", "real :: a(e(1)%n)", "a = 0.0", "end subroutine"},
			"BOUNDVAR: E can't be used in an array's bounds: it isn't a dummy argument or in a common block"},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
