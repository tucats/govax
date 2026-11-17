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
// (docs/planning/PHASE-99 - Fortran 90.md, subtask 5): a module is one unit, its
// interface block, derived type, and CONTAINS each holding what is inside them,
// a module procedure's own CONTAINS inside that, and the unit's own END in its
// body; after the module, a main program with Fortran 90's forms of old
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
		"2           not yet: INTERFACE",
		"3             FUNCTION F(X)",
		"4             END FUNCTION F",
		"5             not yet: MODULE PROCEDURE",
		"6             not yet: END INTERFACE",
		"7           not yet: derived TYPE",
		"8             REAL :: X, Y",
		"9             not yet: END TYPE",
		"10          not yet: CONTAINS",
		"11            SUBROUTINE A",
		"12            not yet: CONTAINS",
		"13              SUBROUTINE B",
		"14              END SUBROUTINE B",
		"15            END SUBROUTINE A",
		"16          END MODULE M",
		"== main program P",
		"17          PROGRAM P",
		"18          not yet: REAL, a kind or length selector",
		"19          DIMENSION :: A(10)",
		"20          not yet: IMPLICIT, a kind or length selector",
		"21          OUTER: DO I = 1, 10",
		"22          IF (.GT. I 3) not yet: EXIT",
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
