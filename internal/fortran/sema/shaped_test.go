package sema

import "testing"

// TestAssumedShape checks assumed-shape dummy arguments (shaped.go;
// docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md, findings 9 and 10):
// their declarations, the actual arguments an explicit interface lets them take,
// and the messages for the rest, with the NOTIMPLs subtask 6 will take away.
func TestAssumedShape(t *testing.T) {
	// host is a program whose internal procedure S has the assumed-shape dummy
	// V of the declaration given, and the statements given.
	host := func(decl string, lines ...string) []string {
		head := []string{"program p", "real a(10), b(3, 4), x", "integer iv(3)", "character*4 c(5)",
			"character*2 d(5)"}

		return append(append(append(head, lines...), "contains", "subroutine s(v)"), decl, "end subroutine", "end program")
	}

	// file is a program calling the external subroutine T, whose dummy is
	// V(:), through no interface.
	file := func(lines ...string) []string {
		return append(append([]string{"program p", "real a(10)"}, lines...),
			"end program", "subroutine t(v)", "real v(:)", "end subroutine")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"a whole array", host("real v(:)", "call s(a)"), ""},
		{"sections", host("real v(:)", "call s(a(2:9:3))", "call s(a(10:1:-1))", "call s(b(2, :))"), ""},
		{"a matrix", host("real v(0:, -1:)", "call s(b)", "call s(b(1:3:2, 4:1:-1))"), ""},
		{"a CHARACTER one", host("character*(*) v(:)", "call s(c)", "call s(c(1:5:2))"), ""},
		{"of its own length", host("character*4 v(:)", "call s(c)"), ""},
		{"an INTENT(OUT) one given a section", host("real, intent(out) :: v(:)", "call s(a(1:5))"), ""},
		{"an element", host("real v(:)", "call s(a(2))"),
			"ARGRANK: argument 1 of S is a scalar, but its dummy argument V is an array of rank 1 (line 8)"},
		{"a scalar", host("real v(:)", "call s(x)"),
			"ARGRANK: argument 1 of S is a scalar, but its dummy argument V is an array of rank 1 (line 8)"},
		{"another rank", host("real v(:)", "call s(b)"),
			"ARGRANK: argument 1 of S is an array of rank 2, but its dummy argument V is an array of rank 1 (line 8)"},
		{"another length", host("character*4 v(:)", "call s(d)"),
			"ARGLENGTH: argument 1 of S is 2 characters long, but its dummy argument V is 4 (line 8)"},
		{"a vector subscript to INTENT(OUT)", host("real, intent(out) :: v(:)", "call s(a(iv))"),
			"NOTDEFINABLE: argument 1 of S must be a variable, since its dummy argument V is INTENT(OUT) (line 8)"},
		{"a vector subscript", host("real v(:)", "call s(a(iv))", "call s(b(iv, 2))"), ""},
		{"an expression", host("real v(:)", "call s(a + 1.0)", "call s(sqrt(a(1:3)))", "call s((/ 1.0, x /))"), ""},
		{"a whole array in parentheses", host("real v(:)", "call s((a))"), ""},
		{"in parentheses to INTENT(INOUT)", host("real, intent(inout) :: v(:)", "call s((a))"),
			"NOTDEFINABLE: argument 1 of S must be a variable, since its dummy argument V is INTENT(INOUT) (line 8)"},
		{"a substring of a section", host("character*(*) v(:)", "call s(c(2:4)(2:3))", "call s(c(iv)(1:2))"), ""},
		{"a concatenation", host("character*(*) v(:)", "call s(c // d)"), ""},
		{"an expression of another rank", host("real v(:)", "call s(b * 2.0)"),
			"ARGRANK: argument 1 of S is an array of rank 2, but its dummy argument V is an array of rank 1 (line 8)"},
		{"a run-time length not of names", []string{"subroutine q(w, n)", "character*(*) w(:)", "integer n",
			"call s(w(:)(1:n))", "call s(w // w)", "call s(w(:)(1:n) // 'x')", "contains", "subroutine s(v)",
			"character*(*) v(:)", "end subroutine", "end subroutine"},
			"NOTIMPL: a CHARACTER array expression of a length known only when the program runs, not made of names, as an assumed-shape dummy's actual argument is not implemented yet"},

		{"a generic's assumed-shape specific", []string{"program p", "interface g", "subroutine g1(v)", "real v(:)",
			"end subroutine", "subroutine g2(v)", "integer v(:)", "end subroutine", "end interface",
			"real a(10)", "integer k(4)", "call g(a(1:9:2))", "call g(a * 2.0)", "call g(k(k))", "end program"}, ""},
		{"a generic's other specific", []string{"program p", "interface g", "subroutine g1(v)", "real v(10)",
			"end subroutine", "end interface", "real a(20)", "call g(a(1:10))", "end program"},
			"NOTIMPL: an array section or array expression as an actual argument is not implemented yet"},

		{"an assumed-size array", []string{"subroutine q(w)", "real w(*)", "call s(w)", "contains",
			"subroutine s(v)", "real v(:)", "end subroutine", "end subroutine"},
			"NOSHAPE: the assumed-size array W has no shape, so it can't be used as a whole here"},
		{"passed on whole", []string{"subroutine q(w)", "real w(:)", "call s(w)", "call s(w(10:1:-2))", "contains",
			"subroutine s(v)", "real v(:)", "end subroutine", "end subroutine"}, ""},
		{"to a dummy that isn't assumed-shape", []string{"subroutine q(w)", "real w(:)", "call t(w)", "end subroutine"},
			"NOTIMPL: an assumed-shape array as the actual argument of a dummy that isn't assumed-shape (Phase 104, subtask 6) is not implemented yet"},
		{"as a format", []string{"subroutine q(w)", "character*8 w(:)", "print w", "end subroutine"},
			"NOTIMPL: an assumed-shape array as a format (Phase 104, subtask 6) is not implemented yet"},
		{"as an internal file", []string{"subroutine q(w)", "character*8 w(:)", "write (w, '(a)') 'x'", "end subroutine"},
			"NOTIMPL: an assumed-shape array as an internal file (Phase 104, subtask 6) is not implemented yet"},
		{"in an output list", []string{"subroutine q(w)", "real w(:)", "print *, w, w(2:)", "end subroutine"}, ""},
		{"SIZEOF", []string{"subroutine q(w)", "real w(:)", "print *, sizeof(w)", "end subroutine"}, ""},
		{"its bounds in an automatic array's", []string{"subroutine q(w, n)", "real w(n:)", "real t(size(w), lbound(w, 1))",
			"t = 1", "end subroutine"}, ""},

		{"through no interface", file("call t(a)"),
			"EXPLICITREQ: T has the assumed-shape dummy argument V (line 5), so a reference to it needs an explicit interface"},
		{"through an interface body", file("interface", "subroutine t(w)", "real w(:)", "end subroutine", "end interface",
			"call t(a(1:9:2))"), ""},
		{"an interface body of another shape", file("interface", "subroutine t(w)", "real w(10)", "end subroutine",
			"end interface", "call t(a)"),
			"INTERFACEDIFF: this interface body for T doesn't agree with its definition (line 10): its dummy argument 1 (W) is an explicit-shape array here, an assumed-shape array there"},

		{"not a dummy", []string{"subroutine q", "real w(0:)", "end subroutine"},
			"SHAPENOTDUMMY: W has an assumed shape, (lb:), so it must be a dummy argument"},
		{"colons and bounds", []string{"subroutine q(w)", "real w(:, 3)", "end subroutine"},
			"DEFERREDMIX: the bounds of W must be all colons or none"},
		{"a lower bound not allowed", []string{"subroutine q(w)", "integer k", "real w(k:)", "end subroutine"},
			"BOUNDVAR: K can't be used in an array's bounds: it isn't a dummy argument or in a common block"},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
