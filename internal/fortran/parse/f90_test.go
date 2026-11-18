package parse

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/source"
)

// TestF90Nesting checks the statements Fortran 90's statements gather
// (docs/planning/PHASE-99 - Fortran 90.md, subtask 5; Phase 103, subtask 2): a
// module is one unit, its interface block holding its bodies, its derived type
// its components, and its CONTAINS its procedures, a module procedure's own
// CONTAINS inside that, and the unit's own END in its body; after the module, a main program with Fortran 90's forms of old
// statements and construct names.
func TestF90Nesting(t *testing.T) {
	file, ms := parseLines(t, "N.FOR",
		fixed("MODULE M"),
		fixed("INTERFACE G"),
		fixed("FUNCTION F(X)"),
		fixed("END FUNCTION F"),
		fixed("MODULE PROCEDURE A"),
		fixed("END INTERFACE"),
		fixed("TYPE, PUBLIC :: POINT"),
		fixed("REAL :: X, Y"),
		fixed("END TYPE"),
		fixed("CONTAINS"),
		fixed("SUBROUTINE A"),
		fixed("CONTAINS"),
		fixed("SUBROUTINE B"),
		fixed("END SUBROUTINE B"),
		fixed("END SUBROUTINE A"),
		fixed("END MODULE M"),
		fixed("PROGRAM P"),
		fixed("REAL(8) X"),
		fixed("DIMENSION :: A(10)"),
		fixed("IMPLICIT REAL(8) (A-H), INTEGER (I-N)"),
		fixed("OUTER: DO I = 1, 10"),
		fixed("IF (I .GT. 3) EXIT OUTER"),
		fixed("END DO OUTER"),
		fixed("END PROGRAM P"),
	)

	if len(ms) != 0 {
		t.Fatalf("messages: %v", ms)
	}

	want := strings.Join([]string{
		"== MODULE M",
		"1           MODULE M",
		"2           INTERFACE G",
		"              == FUNCTION F",
		"3             FUNCTION F(X)",
		"4             END FUNCTION F",
		"5             MODULE PROCEDURE A",
		"6           END INTERFACE",
		"7           not yet: derived TYPE",
		"8             REAL :: X, Y",
		"9             not yet: END TYPE",
		"10          CONTAINS",
		"              == SUBROUTINE A",
		"11            SUBROUTINE A",
		"12            CONTAINS",
		"                == SUBROUTINE B",
		"13              SUBROUTINE B",
		"14              END SUBROUTINE B",
		"15            END SUBROUTINE A",
		"16          END MODULE M",
		"== main program P",
		"17          PROGRAM P",
		"18          REAL(KIND=8) X",
		"19          DIMENSION :: A(10)",
		"20          IMPLICIT REAL(KIND=8) (A-H), INTEGER (I-N)",
		"21          OUTER: DO I = 1, 10",
		"22          IF (.GT. I 3) EXIT OUTER",
		"23          END DO OUTER",
		"24          END PROGRAM P",
		"",
	}, "\n")

	if got := ast.Dump(file); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestNamedEnds checks END's word and name against the unit, and that a
// procedure inside another says which it is.
func TestNamedEnds(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string // the message and its text, "" for none
	}{
		{"matching", []string{"PROGRAM P", "END PROGRAM P"}, ""},
		{"no name", []string{"SUBROUTINE S", "END SUBROUTINE"}, ""},
		{"plain", []string{"FUNCTION F()", "END"}, ""},
		{"block data", []string{"BLOCK DATA B", "END BLOCK DATA B"}, ""},
		{"a main program with no PROGRAM statement", []string{"X = 1", "END PROGRAM"}, ""},
		{"the wrong word", []string{"SUBROUTINE S", "END FUNCTION S"},
			"ENDKIND: the END FUNCTION statement can't end the SUBROUTINE S"},
		{"the wrong name", []string{"FUNCTION F()", "END FUNCTION G"},
			"ENDNAME: the END statement names G, but it ends the FUNCTION F"},
		{"a name for a main program with none", []string{"X = 1", "END PROGRAM X"},
			"ENDNONAME: the END statement names X, but the main program has no name"},
		{"a name for an unnamed block data", []string{"BLOCK DATA", "END BLOCK DATA B"},
			"ENDNONAME: the END statement names B, but the BLOCK DATA has no name"},
		{"an inner procedure's plain END", []string{"SUBROUTINE S", "CONTAINS", "SUBROUTINE T", "END", "END"},
			"ENDNOKIND: the END of the SUBROUTINE T, which is inside another unit, must be END SUBROUTINE"},
		{"an inner procedure's wrong name", []string{"SUBROUTINE S", "CONTAINS", "SUBROUTINE T", "END SUBROUTINE U", "END"},
			"ENDNAME: the END statement names U, but it ends the SUBROUTINE T"},
		{"a module's", []string{"MODULE M", "END MODULE N"},
			"ENDNAME: the END statement names N, but it ends the MODULE M"},
	}

	for _, c := range cases {
		var lines []string
		for _, l := range c.lines {
			lines = append(lines, fixed(l))
		}

		_, ms := parseLines(t, "E.FOR", lines...)

		var got string
		if len(ms) > 0 {
			got = ms[0].Ident + ": " + ms[0].Text
		}

		if got != c.want || len(ms) > 1 {
			t.Errorf("%s: got %v, want %q", c.name, ms, c.want)
		}
	}
}

// TestF90Declarations checks Fortran 90's declarations (docs/planning/PHASE-100 -
// Fortran 90 declarations and kinds.md, subtask 3): the :: form with attributes
// and = values, the attribute statements with and without ::, INTENT's specs
// (IN OUT in free form), and the bounds with a colon.
func TestF90Declarations(t *testing.T) {
	var messages diag.List

	lines := []string{
		"subroutine s(p, q, r)",
		"  integer, parameter :: n = 10, m = n*2",
		"  real*8, dimension(n), save :: x, y(2) = 0",
		"  character*(*), parameter :: name = 'xyz'",
		"  character*8, volatile :: c = 'ab', d*2",
		"  real, intent(in out), optional :: p",
		"  intent(out) :: q",
		"  intent (in) r",
		"  optional q",
		"  target :: t(4), u",
		"  real, allocatable :: a(:, :), b(0:)",
		"  dimension :: z(3)",
		"  save :: k",
		"  external :: f",
		"end subroutine s",
	}

	program := source.Read(source.Form{}, &source.File{Spec: "D.F90", Lines: lines, Free: true}, nil, &messages)
	file := Parse(lex.Lex(program.Statements, &messages), &messages)

	if ms := messages.Messages(); len(ms) != 0 {
		t.Fatalf("messages: %v", ms)
	}

	want := strings.Join([]string{
		"== SUBROUTINE S",
		"1           SUBROUTINE S(P, Q, R)",
		"2           INTEGER, PARAMETER :: N = 10, M = (* N 2)",
		"3           REAL*8, DIMENSION(N), SAVE :: X, Y(2) = 0",
		"4           CHARACTER*(*), PARAMETER :: NAME = 'xyz'",
		"5           CHARACTER*8, VOLATILE :: C = 'ab', D*2",
		"6           REAL, INTENT(INOUT), OPTIONAL :: P",
		"7           INTENT(OUT) :: Q",
		"8           INTENT(IN) R",
		"9           OPTIONAL Q",
		"10          TARGET :: T(4), U",
		"11          REAL, ALLOCATABLE :: A(:, :), B(0:)",
		"12          DIMENSION :: Z(3)",
		"13          SAVE :: K",
		"14          EXTERNAL :: F",
		"15          END SUBROUTINE S",
		"",
	}, "\n")

	if got := ast.Dump(file); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	errors := []struct{ line, want string }{
		{"REAL :: X /1.0/", "SLASHINIT"},
		{"REAL, BOGUS :: X", "BADATTR"},
		{"REAL, INTENT(UP) :: X", "SYNTAX"},
		{"REAL, SAVE X", "SYNTAX"},
		{"SAVE ::", "SYNTAX"},
	}

	for _, e := range errors {
		_, ms := parseLines(t, "E.FOR", fixed(e.line), fixed("END"))
		if got := strings.Join(idents(ms), " "); got != e.want {
			t.Errorf("%s: got %q, want %q", e.line, got, e.want)
		}
	}
}

// TestF90TypeSelectors checks Fortran 90's kind and length selectors
// (docs/planning/PHASE-100 - Fortran 90 declarations and kinds.md, subtask 4):
// every type's ([KIND=] k), CHARACTER's lengths and kinds in each order the
// standard's R506 allows, a selector before FUNCTION and in IMPLICIT, and
// kinds on constants.
func TestF90TypeSelectors(t *testing.T) {
	var messages diag.List

	lines := []string{
		"character(len=*) function f(n)",
		"  implicit real(kind=8) (d), character(len=4) (c)",
		"  integer(2) :: i = 7_2",
		"  real(kind=dp) x",
		"  complex(kind(0.0)) z",
		"  logical(1), save :: l = .true._1",
		"  character(10) a",
		"  character(*) b",
		"  character(len=*) e",
		"  character(5, 1) g",
		"  character(5, kind=1) h",
		"  character(kind=1, len=n) o",
		"  character(len=3, kind=1) :: p = 1_'abc'",
		"  character(kind=1) q",
		"  x = 1.5_dp",
		"end function f",
	}

	program := source.Read(source.Form{}, &source.File{Spec: "K.F90", Lines: lines, Free: true}, nil, &messages)
	file := Parse(lex.Lex(program.Statements, &messages), &messages)

	if ms := messages.Messages(); len(ms) != 0 {
		t.Fatalf("messages: %v", ms)
	}

	want := strings.Join([]string{
		"== FUNCTION F",
		"1           CHARACTER(LEN=*) FUNCTION F(N)",
		"2           IMPLICIT REAL(KIND=8) (D), CHARACTER(LEN=4) (C)",
		"3           INTEGER(KIND=2) :: I = (kind 7 2)",
		"4           REAL(KIND=DP) X",
		"5           COMPLEX(KIND=KIND(0.0)) Z",
		"6           LOGICAL(KIND=1), SAVE :: L = (kind .TRUE. 1)",
		"7           CHARACTER(LEN=10) A",
		"8           CHARACTER(LEN=*) B",
		"9           CHARACTER(LEN=*) E",
		"10          CHARACTER(LEN=5, KIND=1) G",
		"11          CHARACTER(LEN=5, KIND=1) H",
		"12          CHARACTER(LEN=N, KIND=1) O",
		"13          CHARACTER(LEN=3, KIND=1) :: P = (kind 'abc' 1)",
		"14          CHARACTER(KIND=1) Q",
		"15          X = (kind 1.5 DP)",
		"16          END FUNCTION F",
		"",
	}, "\n")

	if got := ast.Dump(file); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	errors := []struct{ line, want string }{
		{"CHARACTER(LEN=3, 1) C", "SYNTAX"},
		{"CHARACTER(KIND=1, 3) C", "SYNTAX"},
		{"CHARACTER(LEN=3, LEN=4) C", "SYNTAX"},
		{"CHARACTER(3, 1, 1) C", "SYNTAX"},
		{"REAL(8)*4 X", "SYNTAX"},
		{"REAL() X", "SYNTAX"},
	}

	for _, e := range errors {
		_, ms := parseLines(t, "E.FOR", fixed(e.line), fixed("END"))
		if got := strings.Join(idents(ms), " "); got != e.want {
			t.Errorf("%s: got %q, want %q", e.line, got, e.want)
		}
	}
}

// TestKeywordArguments checks keyword arguments in a reference's list (Phase
// 100, subtask 6): a name and = before an expression, in either form, beside
// arguments without one and null ones; == is still a comparison.
func TestKeywordArguments(t *testing.T) {
	var messages diag.List

	lines := []string{
		"program k",
		"  i = int(x, kind=2) + selected_real_kind(r = 40)",
		"  j = f(a, , b=c(1), d=e == g)",
		"end program k",
	}

	program := source.Read(source.Form{}, &source.File{Spec: "K.F90", Lines: lines, Free: true}, nil, &messages)
	file := Parse(lex.Lex(program.Statements, &messages), &messages)

	if ms := messages.Messages(); len(ms) != 0 {
		t.Fatalf("messages: %v", ms)
	}

	want := strings.Join([]string{
		"== main program K",
		"1           PROGRAM K",
		"2           I = (+ INT(X, KIND=2) SELECTED_REAL_KIND(R=40))",
		"3           J = F(A, , B=C(1), D=(.EQ. E G))",
		"4           END PROGRAM K",
		"",
	}, "\n")

	if got := ast.Dump(file); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestSelectCaseForms parses the CASE construct's statements in free form,
// where CASE DEFAULT and a construct name after it are two names, and a
// name that only begins with DEFAULT is no DEFAULT.
func TestSelectCaseForms(t *testing.T) {
	var messages diag.List

	lines := []string{
		"program p",
		"  pick: select case (i + 1)",
		"  case (1, 3:5, :0, 10:) pick",
		"  case ('a':'z')",
		"  case default pick",
		"  case default",
		"  end select pick",
		"  end select",
		"end program p",
	}

	program := source.Read(source.Form{}, &source.File{Spec: "S.F90", Lines: lines, Free: true}, nil, &messages)
	file := Parse(lex.Lex(program.Statements, &messages), &messages)

	if ms := messages.Messages(); len(ms) != 0 {
		t.Fatalf("messages: %v", ms)
	}

	want := strings.Join([]string{
		"== main program P",
		"1           PROGRAM P",
		"2           PICK: SELECT CASE (+ I 1)",
		"3           CASE (1, 3:5, :0, 10:) PICK",
		"4           CASE ('a':'z')",
		"5           CASE DEFAULT PICK",
		"6           CASE DEFAULT",
		"7           END SELECT PICK",
		"8           END SELECT",
		"9           END PROGRAM P",
		"",
	}, "\n")

	if got := ast.Dump(file); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	errors := []struct{ line, want string }{
		{"  case defaultx", "SYNTAX"},
		{"  case (:)", "SYNTAX"},
		{"  case ()", "SYNTAX"},
		{"  case (1) a, b", "EXTRATEXT"},
		{"  case default (1)", "EXTRATEXT"},
		{"  select case i", "SYNTAX"},
	}

	for _, e := range errors {
		var messages diag.List

		program := source.Read(source.Form{}, &source.File{Spec: "E.F90", Lines: []string{e.line, "end"}, Free: true}, nil, &messages)
		Parse(lex.Lex(program.Statements, &messages), &messages)

		if got := strings.Join(idents(messages.Messages()), " "); got != e.want {
			t.Errorf("%s: got %s, want %s", e.line, got, e.want)
		}
	}
}

// TestArrayForms checks Fortran 90's array forms (docs/planning/PHASE-102 -
// Fortran 90 array expressions.md, subtask 2): subscript triplets in every
// shape, in a list with subscripts and after a substring's list, a substring
// still read as a triplet after a name (sema tells it from a section), a
// second list after a section a substring, array constructors with nested
// implied-DO lists, and the WHERE statements.
func TestArrayForms(t *testing.T) {
	var messages diag.List

	lines := []string{
		"program p",
		"  a(1:n:2) = b(:, j) + c(n:1:-1)",
		"  a(:) = b(2:, :3)",
		"  s = c(1:3)",
		"  t(:, 1)(2:3) = 'ab'",
		"  x = (/ 1, 2, (i, i*i, i = 3, n), ((k, k = 1, i), i = 1, 2) /)",
		"  y = sum((/ a(1:2), 3.0 /))",
		"  where (a > 0) a = sqrt(a)",
		"  where (b(:, 1) /= 0)",
		"    b(:, 1) = 1 / b(:, 1)",
		"  elsewhere",
		"    b(:, 1) = 0",
		"  end where",
		"  where (a > 0)",
		"  elsewhere (a < 0)",
		"  endwhere",
		"end program p",
	}

	program := source.Read(source.Form{}, &source.File{Spec: "A.F90", Lines: lines, Free: true}, nil, &messages)
	file := Parse(lex.Lex(program.Statements, &messages), &messages)

	if ms := messages.Messages(); len(ms) != 0 {
		t.Fatalf("messages: %v", ms)
	}

	want := strings.Join([]string{
		"== main program P",
		"1           PROGRAM P",
		"2           A(1:N:2) = (+ B(:, J) C(N:1:(- 1)))",
		"3           A(:) = B(2:, :3)",
		"4           S = C(1:3)",
		"5           T(:, 1)(2:3) = 'ab'",
		"6           X = (/ 1, 2, (do I, (* I I); I = 3, N), (do (do K; K = 1, I); I = 1, 2) /)",
		"7           Y = SUM((/ A(1:2), 3.0 /))",
		"8           WHERE (.GT. A 0) A = SQRT(A)",
		"9           WHERE (.NE. B(:, 1) 0)",
		"10          B(:, 1) = (/ 1 B(:, 1))",
		"11          ELSEWHERE",
		"12          B(:, 1) = 0",
		"13          END WHERE",
		"14          WHERE (.GT. A 0)",
		"15          ELSEWHERE (.LT. A 0)",
		"16          END WHERE",
		"17          END PROGRAM P",
		"",
	}, "\n")

	if got := ast.Dump(file); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	errors := []struct{ line, want string }{
		{"A(1:2:) = 0", "SYNTAX"},
		{"X = (/ 1, 2 )", "SYNTAX"},
		{"X = (/ /)", "SYNTAX"},
		{"WHERE (A > 0) GO TO 10", "BADWHERESTMT"},
		{"WHERE (A > 0) CALL S", "BADWHERESTMT"},
		{"END WHERE X", "EXTRATEXT"},
	}

	for _, e := range errors {
		_, ms := parseLines(t, "E.FOR", fixed(e.line), "   10 END")
		if got := strings.Join(idents(ms), " "); got != e.want {
			t.Errorf("%s: got %v, want %s", e.line, ms, e.want)
		}
	}
}
