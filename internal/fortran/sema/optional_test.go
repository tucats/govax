package sema

import "testing"

// TestKeywordsAndOptional checks keyword arguments, OPTIONAL dummies, and
// PRESENT (optional.go): nothing for a program that is right, and each message
// for a mistake.
func TestKeywordsAndOptional(t *testing.T) {
	inner := func(call string, more ...string) []string {
		lines := append([]string{"program p", "real x"}, more...)

		return append(lines, call, "contains", "subroutine s(a, b, c)", "real a", "real, optional :: b",
			"integer, optional :: c", "end subroutine", "end program")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"positional", inner("call s(x, 1.0, 2)"), ""},
		{"trailing ones left out", inner("call s(x)"), ""},
		{"by keyword, in any order", inner("call s(c=2, a=x)"), ""},
		{"both", inner("call s(x, c=2)"), ""},
		{"a null argument", inner("call s(x, , 2)"), ""},
		{"no such keyword", inner("call s(x, d=1)"), "KEYWORDNAME: S has no argument D; its arguments are A, B, and C"},
		{"a keyword twice", inner("call s(x, b=1.0, b=2.0)"), "KEYWORDTWICE: the argument B of S is given twice"},
		{"a dummy given twice", inner("call s(x, a=x)"), "KEYWORDTWICE: the argument A of S is given twice"},
		{"positional after a keyword", inner("call s(b=1.0, x)"),
			"KEYWORDORDER: an argument of S without a keyword can't follow one with a keyword"},
		{"a required one left out", inner("call s(b=1.0)"), "MISSINGARG: S needs its argument A"},
		{"too many", inner("call s(x, 1.0, 2, 3)"), "ARGCOUNT: S takes 3 dummy arguments (line 5), but is given 4 here"},
		{"too many, with a keyword", inner("call s(x, 1.0, 2, 3, c=1)"),
			"ARGCOUNT: S takes 3 dummy arguments (line 5), but is given 5 here"},
		{"a keyword's type", inner("call s(x, c=1.0)"),
			"ARGTYPE: argument 3 of S is a REAL, but its dummy argument C is an INTEGER (line 5)"},
		{"no explicit interface", []string{"program p", "call t(x, n=1)", "end program"},
			"KEYWORDNOINTERFACE: T has no explicit interface here, so its arguments can't be given by keyword"},
		{"a function by keyword", []string{"program p", "y = f(b=2.0, a=1.0)", "contains", "real function f(a, b)",
			"real a, b", "f = a - b", "end function", "end program"}, ""},
		{"an interface body's names", []string{"program p", "interface", "subroutine t(u, v)", "real u",
			"real, optional :: v", "end subroutine", "end interface", "call t(v=1.0, u=2.0)", "end program"}, ""},
		{"PRESENT", []string{"subroutine s(a)", "real, optional :: a", "if (present(a)) a = 1", "end subroutine"}, ""},
		{"PRESENT by keyword", []string{"subroutine s(a)", "real, optional :: a", "l = present(a=a)", "end subroutine"}, ""},
		{"PRESENT of the host's", []string{"subroutine h(a)", "real, optional :: a", "call s", "contains", "subroutine s",
			"if (present(a)) print *, a", "end subroutine", "end subroutine"}, ""},
		{"PRESENT of a dummy not OPTIONAL", []string{"subroutine s(a)", "l = present(a)", "end subroutine"},
			"NOTOPTIONAL: the argument of PRESENT must be the name of an OPTIONAL dummy argument"},
		{"PRESENT of a variable", []string{"program p", "l = present(x)", "end program"},
			"NOTOPTIONAL: the argument of PRESENT must be the name of an OPTIONAL dummy argument"},
		{"PRESENT of an expression", []string{"subroutine s(a)", "real, optional :: a", "l = present(a + 1)", "end subroutine"},
			"NOTOPTIONAL: the argument of PRESENT must be the name of an OPTIONAL dummy argument"},
		{"a program's own PRESENT", []string{"program p", "real present", "present = 1", "end program"}, ""},
		{"a recursive one's call of itself by keyword", []string{"recursive subroutine r(n, m)", "integer n",
			"integer, optional :: m", "if (n > 0) call r(m=1, n=n-1)", "end subroutine"}, ""},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestIntent checks INTENT's messages (intent.go).
func TestIntent(t *testing.T) {
	unit := func(decl string, body ...string) []string {
		return append(append([]string{"subroutine s(a, v, n)", "real a, v(3)", "integer n", decl}, body...), "end subroutine")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"IN read", unit("intent(in) a", "print *, a"), ""},
		{"IN assigned", unit("intent(in) a", "a = 1"), "INTENTIN: A is an INTENT(IN) dummy argument, which can't be given a value"},
		{"IN element assigned", unit("intent(in) v", "v(2) = 1"),
			"INTENTIN: V is an INTENT(IN) dummy argument, which can't be given a value"},
		{"IN as a DO variable", unit("intent(in) n", "do n = 1, 2", "end do"),
			"INTENTIN: N is an INTENT(IN) dummy argument, which can't be given a value"},
		{"IN read in", unit("intent(in) a", "read *, a"),
			"INTENTIN: A is an INTENT(IN) dummy argument, which can't be given a value"},
		{"IN passed to an implicit interface", unit("intent(in) a", "call t(a)"), ""},
		{"IN text read from", []string{"subroutine s(c)", "character(len=*), intent(in) :: c", "read (c(2:), *) x",
			"read (c, *) y", "decode (4, '(F4.0)', c(1:4)) z", "end subroutine"}, ""},
		{"IN text written to", []string{"subroutine s(c)", "character(len=*), intent(in) :: c", "write (c(2:), *) 1",
			"end subroutine"}, "INTENTIN: C is an INTENT(IN) dummy argument, which can't be given a value"},
		{"IN passed to INTENT(OUT)", []string{"subroutine s(a)", "real, intent(in) :: a", "call t(a)", "contains",
			"subroutine t(b)", "real, intent(out) :: b", "b = 1", "end subroutine", "end subroutine"},
			"INTENTIN: A is an INTENT(IN) dummy argument, which can't be given a value"},
		{"OUT defined first", unit("intent(out) a", "a = 1", "print *, a"), ""},
		{"OUT used first", unit("intent(out) a", "print *, a", "a = 1"),
			"INTENTOUTREF: A is an INTENT(OUT) dummy argument, used here before any statement gives it a value"},
		{"OUT on one line", unit("intent(out) n", "n = n + 1"), ""},
		{"OUT passed first", unit("intent(out) a", "call t(a)", "print *, a"), ""},
		{"OUT never defined", unit("intent(out) v", "print *, v(1)"),
			"INTENTOUTREF: V is an INTENT(OUT) dummy argument, used here before any statement gives it a value"},
		{"OUT and PRESENT", []string{"subroutine s(a)", "real, intent(out), optional :: a", "if (present(a)) a = 1",
			"end subroutine"}, ""},
		// An inquiry of a dummy's shape, length, or kind isn't a use of its value
		// (Phase 111, subtask 5a), nor does it give it one.
		{"OUT's size asked", unit("intent(out) v", "n = size(v)", "v = 0"), ""},
		{"OUT's bounds asked", unit("intent(out) v", "n = lbound(v, 1) + ubound(v, 1) + size(shape(v))", "v = 0"), ""},
		{"OUT's kind asked", unit("intent(out) a", "n = kind(a) + digits(a)", "x = huge(a) + epsilon(a)", "a = 1"), ""},
		{"OUT's length asked", []string{"subroutine s(c)", "character(len=*), intent(out) :: c", "n = len(c)", "c = 'x'",
			"end subroutine"}, ""},
		{"OUT used after its size", unit("intent(out) v", "n = size(v)", "print *, v(1)", "v = 0"),
			"INTENTOUTREF: V is an INTENT(OUT) dummy argument, used here before any statement gives it a value"},
		{"OUT set by an internal procedure", []string{"subroutine s(a)", "real, intent(out) :: a", "call t",
			"print *, a", "contains", "subroutine t", "a = 1", "end subroutine", "end subroutine"}, ""},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
