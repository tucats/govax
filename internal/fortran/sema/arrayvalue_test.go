package sema

import "testing"

// TestArrayValue is array-valued functions' declarations, references, and
// messages (arrayvalue.go; docs/planning/PHASE-104 - Fortran 90 dynamic
// arrays.md, subtask 7a).
func TestArrayValue(t *testing.T) {
	// file is a program referencing the external function F, whose value is
	// REAL F(3), through the lines given, then F itself.
	file := func(lines ...string) []string {
		return append(append([]string{"program p", "real a(3), x"}, lines...),
			"end program", "function f(n)", "integer n", "real f(3)", "f = n", "end function")
	}

	iface := []string{"interface", "function f(n)", "integer n", "real f(3)", "end function", "end interface"}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"through an interface body", file(append(iface, "a = f(1) + a", "print *, f(2)", "x = sum(f(3))")...), ""},
		{"an internal function", []string{"program p", "real a(3)", "a = g(1.0)", "contains", "function g(x)",
			"real x, g(3)", "g = x", "end function", "end program"}, ""},
		{"with RESULT", []string{"function g(x) result(r)", "real x", "real, dimension(2, 2) :: r", "r = x",
			"r(1, 2) = 0", "end function"}, ""},
		{"DIMENSION of the value", []string{"function g(x)", "real x", "dimension g(4)", "g = x", "end function"}, ""},
		{"a CHARACTER value", []string{"function g(x)", "character*3 g(2)", "g = 'abc'", "end function"}, ""},
		{"a recursive one's reference to itself", []string{"recursive function g(n) result(r)", "integer n, r(2)",
			"r = 0", "if (n > 0) r = g(n - 1) + 1", "end function"}, ""},
		{"through a dummy procedure's interface", []string{"subroutine s(fn)", "interface", "function fn(i)",
			"integer i, fn(2)", "end function", "end interface", "print *, fn(1) * 2", "end subroutine"}, ""},
		{"a generic's specific", []string{"program p", "interface g", "function g1(i)", "integer i, g1(2)",
			"end function", "end interface", "integer k(2)", "k = g(3)", "end program"}, ""},

		{"where a scalar must be", file(append(iface, "x = f(1)")...),
			"NOTSCALAR: an array is used where a scalar is required"},
		{"of another shape", file(append(iface, "a(1:2) = f(1)")...),
			"NOTCONFORM: the shapes (2) and (3) don't conform"},
		{"through no interface", file("a = f(1)"),
			"EXPLICITVALUE: the value of F (line 5) is an array, so a reference to it needs an explicit interface"},
		{"an interface body of another shape", file("interface", "function f(n)", "integer n", "real f(4)",
			"end function", "end interface", "a = 1"),
			"INTERFACEDIFF: this interface body for F doesn't agree with its definition (line 11): its value has 4 elements in dimension 1 here, 3 there"},
		{"an interface body of another rank", file("interface", "function f(n)", "integer n", "real f",
			"end function", "end interface", "a = 1"),
			"INTERFACEDIFF: this interface body for F doesn't agree with its definition (line 11): its value is a scalar here, an array of rank 1 there"},
		{"the name with RESULT", []string{"function g(x) result(r)", "real x, r", "dimension g(2)", "r = x", "end function"},
			"BADUSE: G is the name of this function, so it can't be an array"},
		{"the length (*)", []string{"function g(x)", "character*(*) g(2)", "g = 'x'", "end function"},
			"STARARRAY: G has the length (*), so a function's value of that name can't be an array"},
		{"bounds not constant", []string{"function g(n)", "integer n", "real g(n)", "g = 1", "end function"}, ""},
		{"bounds from a dummy's size", []string{"function g(v)", "real v(:), g(2:size(v)+1)", "g = v", "end function"}, ""},
		{"run-time bounds through an interface body", []string{"program p", "interface", "function g(n)", "integer n",
			"real g(n)", "end function", "end interface", "real a(5)", "a = g(5) + 1", "print *, size(g(3)), g(2)",
			"end program"}, ""},
		{"a run-time value of another rank", []string{"program p", "interface", "function g(n)", "integer n",
			"real g(n)", "end function", "end interface", "real a(5, 5)", "a = g(5)", "end program"},
			"NOTCONFORM: the shapes (5, 5) and (:) don't conform"},
		{"a run-time value where a scalar must be", []string{"program p", "interface", "function g(n)", "integer n",
			"real g(n)", "end function", "end interface", "real x", "x = g(5)", "end program"},
			"NOTSCALAR: an array is used where a scalar is required"},
		{"a recursive one with run-time bounds", []string{"recursive function g(n) result(r)", "integer n, r(n)",
			"r = 1", "if (n > 1) r(2:) = g(n - 1) + 1", "end function"}, ""},
		{"a length not constant", []string{"function g(n)", "integer n", "character(len=n) g(2)", "g = 'x'", "end function"}, ""},
		{"an ENTRY", []string{"function g(n)", "integer n", "real g(2), h(2)", "entry h(n)", "g = 1", "end function"}, ""},
		{"an ENTRY of a made value", []string{"function g(n, k)", "integer n, k", "real g(n), h(n)", "entry h(k, n)",
			"g = 1", "end function"}, ""},
		{"an ENTRY of a made length", []string{"function g(n)", "integer n", "character(len=n) g, h", "entry h(n)",
			"g = 'x'", "end function"}, ""},
		{"an ENTRY of another extent", []string{"function g(n)", "integer n", "real g(2), h(3)", "entry h(n)", "g = 1",
			"end function"},
			"ENTRYVALUE: the entry point H's value must be the same as the function G's: its value has 3 elements in dimension 1, the function's 2"},
		{"an ENTRY of another rank", []string{"function g(n)", "integer n", "real g(2), h", "entry h(n)", "g = 1",
			"end function"},
			"ENTRYVALUE: the entry point H's value must be the same as the function G's: its value is a scalar, the function's an array of rank 1"},
		{"an ENTRY of other bounds", []string{"function g(n, m)", "integer n, m", "real g(n), h(m)", "entry h(n, m)",
			"g = 1", "end function"},
			"ENTRYVALUE: the entry point H's value must be the same as the function G's: its value's bounds in dimension 1 are (1:M), the function's (1:N)"},
		{"an ENTRY of another length", []string{"function g(n)", "integer n", "character(len=n) g", "character(len=n+1) h",
			"entry h(n)", "g = 'x'", "end function"},
			"ENTRYVALUE: the entry point H's value must be the same as the function G's: its value's length is N+1, the function's N"},
		{"an ENTRY that can't make the value", []string{"function g(n)", "integer n", "real g(n), h(n)", "entry h",
			"g = 1", "end function"},
			"ENTRYMAKE: the entry point H has no dummy argument N, which G's value's bounds or length use: it couldn't make the value"},
		{"a made length through an interface", []string{"program p", "interface", "function f(n)", "integer n",
			"character(len=n) f", "end function", "end interface", "character*10 c", "c = f(3) // 'x'",
			"print *, f(2), len(f(4))", "end program"}, ""},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
