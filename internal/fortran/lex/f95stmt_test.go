package lex

import (
	"reflect"
	"testing"
)

// TestF95Prefixes checks PURE, ELEMENTAL, and RECURSIVE on a unit's first
// statement, in any order and on either side of the type, in both source forms
// (docs/planning/PHASE-112 - Fortran 95.md, subtask 2).
func TestF95Prefixes(t *testing.T) {
	cases := []struct {
		text, want string
		free       bool
	}{
		{"PURE FUNCTION F(X)", "FUNCTION (pure): F ( X )", true},
		{"pure integer function sq(x)", "FUNCTION (INTEGER) (pure): FUNCTION SQ ( X )", true},
		{"elemental real function clip(x, lo, hi)", "FUNCTION (REAL) (elemental): FUNCTION CLIP ( X , LO , HI )", true},
		{"elemental subroutine swap(a, b)", "SUBROUTINE (elemental): SWAP ( A , B )", true},
		{"recursive pure subroutine s", "SUBROUTINE (recursive) (pure): S", true},
		{"integer pure recursive function f(n)", "FUNCTION (INTEGER) (recursive) (pure): FUNCTION F ( N )", true},
		{"pure real(8) elemental function g(x)", "FUNCTION (REAL) (pure) (elemental): ( 8 ) FUNCTION G ( X )", true},
		{"pure function twice(x) result(y)", "FUNCTION (pure): TWICE ( X ) RESULT ( Y )", true},
		{"PURE INTEGER FUNCTION SQ(X)", "FUNCTION (INTEGER) (pure): FUNCTION SQ ( X )", false},
		{"ELEMENTALSUBROUTINESWAP(A,B)", "SUBROUTINE (elemental): SWAP ( A , B )", false},
		{"TYPE(POINT) PURE FUNCTION MID(A, B)", "FUNCTION (TYPE (type-name)) (pure): ( POINT ) FUNCTION MID ( A , B )", false},
	}

	for _, c := range cases {
		_, dumps, idents := lexF90(t, c.free, true, line(c.free, c.text))

		if len(idents) != 0 {
			t.Errorf("%s: messages %v", c.text, idents)
		}

		if dumps[0] != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.text, dumps[0], c.want)
		}
	}
}

// line is text as a line of the form.
func line(free bool, text string) string {
	if free {
		return text
	}

	return fixed(text)
}

// TestF95PrefixErrors checks a prefix given twice (DUPPREFIX, the header kept),
// a prefix away from a unit's start (no header there), and the recovery from
// words the lexer doesn't know before FUNCTION or SUBROUTINE (UNRECPREFIX).
func TestF95PrefixErrors(t *testing.T) {
	cases := []struct {
		lines  []string
		want   string
		idents []string
	}{
		{[]string{"pure pure function f(x)"}, "FUNCTION (pure): F ( X )", []string{"DUPPREFIX@006"}},
		{[]string{"pure integer pure function f(x)"}, "FUNCTION (INTEGER) (pure): FUNCTION F ( X )", []string{"DUPPREFIX@001"}},
		{[]string{"recursive integer recursive function f(n)"}, "FUNCTION (INTEGER) (recursive): FUNCTION F ( N )", []string{"DUPPREFIX@001"}},
		{[]string{"impure elemental subroutine s(a)"}, "SUBROUTINE (elemental): S ( A )", []string{"UNRECPREFIX@001"}},
		{[]string{"impure subroutine s(a)"}, "SUBROUTINE: S ( A )", []string{"UNRECPREFIX@001"}},
		{[]string{"non_recursive function f(x)"}, "FUNCTION: F ( X )", []string{"UNRECPREFIX@001"}},
		{[]string{"continue", "pure subroutine s"}, "unrecognized statement:", []string{"UNRECSTMT@001"}},
		{[]string{"pure x"}, "unrecognized statement:", []string{"UNRECSTMT@001"}},
		{[]string{"impure x"}, "unrecognized statement:", []string{"UNRECSTMT@001"}},
	}

	for _, c := range cases {
		_, dumps, idents := lexF90(t, true, true, c.lines...)

		if got := dumps[len(dumps)-1]; got != c.want || !reflect.DeepEqual(idents, c.idents) {
			t.Errorf("%v:\n got %s %v\nwant %s %v", c.lines, got, idents, c.want, c.idents)
		}
	}
}

// TestForall checks FORALL's statement and construct, a construct name on it
// and on a WHERE construct, END FORALL, and FORALL as an array's name.
func TestForall(t *testing.T) {
	cases := []struct {
		text, want string
		free       bool
	}{
		{"forall (i = 1:n) a(i,i) = real(i)", "FORALL: ( I = 1 : N ) / assignment: A ( I , I ) = REAL ( I )", true},
		{"forall (i = 1:n, j = 1:n, a(i,j) /= 0.0) b(i,j) = 1.0 / a(i,j)",
			"FORALL: ( I = 1 : N , J = 1 : N , A ( I , J ) /= 0.0 ) / assignment: B ( I , J ) = 1.0 / A ( I , J )", true},
		{"forall (j = 1:8) pat(j)%p => obj(j)", "FORALL: ( J = 1 : 8 ) / pointer assignment: PAT ( J ) % P => OBJ ( J )", true},
		{"forall (i = 1:n)", "block FORALL: ( I = 1 : N )", true},
		{"named: forall (i = 1:8:2)", "NAMED: block FORALL: ( I = 1 : 8 : 2 )", true},
		{"end forall", "END FORALL:", true},
		{"end forall named", "END FORALL: NAMED", true},
		{"outer: where (a > 0)", "OUTER: block WHERE: ( A > 0 )", true},
		{"FORALL (I=1:N) X(I) = 0", "FORALL: ( I = 1 : N ) / assignment: X ( I ) = 0", false},
		{"ENDFORALL", "END FORALL:", false},
		{"FORALL(I) = 3", "assignment: FORALL ( I ) = 3", false},
	}

	for _, c := range cases {
		_, dumps, idents := lexF90(t, c.free, true, line(c.free, "continue"), line(c.free, c.text))

		if len(idents) != 0 {
			t.Errorf("%s: messages %v", c.text, idents)
		}

		if dumps[1] != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.text, dumps[1], c.want)
		}
	}

	_, _, idents := lexF90(t, true, true, "continue", "name: where (a > 0) a = 1")
	if !reflect.DeepEqual(idents, []string{"NOTCONSTRUCT@001"}) {
		t.Errorf("a named WHERE statement: %v, want NOTCONSTRUCT", idents)
	}
}
