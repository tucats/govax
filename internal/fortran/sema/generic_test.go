package sema

import "testing"

// TestGenerics checks generic interfaces (generic.go): which specific a
// reference chooses, and each message.
func TestGenerics(t *testing.T) {
	swap := []string{"interface swap", "subroutine swapr(a, b)", "real a, b", "end subroutine",
		"subroutine swapi(i, j)", "integer i, j", "end subroutine", "end interface"}
	program := func(lines ...string) []string {
		return append(append(append([]string{"program p"}, swap...), lines...), "end program")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"each specific", program("real x, y", "integer m, n", "call swap(x, y)", "call swap(m, n)",
			"call swapr(x, y)"), ""},
		{"by keyword", program("integer m, n", "call swap(j=m, i=n)"), ""},
		{"none fits", program("real x", "integer m", "call swap(x, m)"),
			"GENERICNONE: no specific procedure of the generic name SWAP takes a REAL and an INTEGER"},
		{"rank counts", program("real x(3), y", "call swap(x, y)"),
			"GENERICNONE: no specific procedure of the generic name SWAP takes a REAL array of rank 1 and a REAL"},
		{"kind counts", program("real*8 x, y", "call swap(x, y)"),
			"GENERICNONE: no specific procedure of the generic name SWAP takes a REAL*8 and a REAL*8"},
		{"as an argument", program("call t(swap)"),
			"GENERICARG: the generic name SWAP can't be an actual argument; pass one of its specific procedures"},
		{"the standard's ambiguous pair", []string{"program p", "interface a", "subroutine ar(x)", "real x",
			"end subroutine", "subroutine ai(j)", "real j", "end subroutine", "end interface", "end program"},
			"GENERICAMBIG: the specific procedures AR (line 3) and AI of the generic name A can't be told apart by their arguments"},
		{"told apart by an optional count", []string{"program p", "interface f", "subroutine f1(x)", "real x",
			"end subroutine", "subroutine f2(x, n)", "real x", "integer n", "end subroutine", "end interface", "end program"}, ""},
		{"an optional dummy doesn't tell", []string{"program p", "interface f", "subroutine f1(x)", "real x",
			"end subroutine", "subroutine f2(x, n)", "real x", "integer, optional :: n", "end subroutine", "end interface",
			"end program"},
			"GENERICAMBIG: the specific procedures F1 (line 3) and F2 of the generic name F can't be told apart by their arguments"},
		{"a function and a subroutine", []string{"program p", "interface g", "real function g1(x)", "real x",
			"end function", "subroutine g2(n)", "integer n", "end subroutine", "end interface", "end program"},
			"GENERICMIX: G1 and G2, specific procedures of the generic name G, must both be functions or both subroutines"},
		{"a function", []string{"program p", "interface twice", "real function twicer(x)", "real x", "end function",
			"integer function twicei(n)", "integer n", "end function", "end interface", "y = twice(1.0) + twice(2)",
			"end program"}, ""},
		{"an intrinsic extended", []string{"program p", "interface sqrt", "logical function sqrtl(l)", "logical l",
			"end function", "end interface", "logical q", "q = sqrt(.true.)", "y = sqrt(2.0)", "end program"}, ""},
		{"an intrinsic's own meaning", []string{"program p", "interface sqrt", "real function mysqrt(x)", "real x",
			"end function", "end interface", "end program"},
			"GENERICINTRINSIC: the specific procedure MYSQRT takes arguments the intrinsic function SQRT already takes"},
		{"a name declared otherwise", []string{"program p", "real swap", "interface swap", "subroutine s(x)", "real x",
			"end subroutine", "end interface", "end program"},
			"GENERICDECL: SWAP is already a name in this unit (line 2), so it can't be a generic name"},
		{"a specific's own name", []string{"program p", "interface twice", "real function twice(x)", "real x",
			"end function", "integer function twicei(n)", "integer n", "end function", "end interface",
			"y = twice(1.0) + twice(2)", "end program"}, ""},
		{"two blocks", []string{"program p", "interface g", "subroutine g1(x)", "real x", "end subroutine",
			"end interface", "interface g", "subroutine g2(n)", "integer n", "end subroutine", "end interface",
			"call g(1)", "call g(1.0)", "end program"}, ""},
		{"the host's", program("call s", "contains", "subroutine s", "call swap(1, 2)", "end subroutine"), ""},
		{"added to in an internal procedure", program("call s", "contains", "subroutine s", "interface swap",
			"subroutine swapl(k, l)", "logical k, l", "end subroutine", "end interface", "logical b",
			"call swap(b, b)", "call swap(1, 2)", "end subroutine"), ""},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
