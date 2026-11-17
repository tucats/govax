package parse

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/ast"
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
		"8             not yet: REAL, the :: form of a type declaration",
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
		"19          not yet: DIMENSION, the :: form of DIMENSION",
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
			"ENDNAME: the END statement names G, but the FUNCTION F is named F"},
		{"a name for a main program with none", []string{"X = 1", "END PROGRAM X"},
			"ENDNONAME: the END statement names X, but the main program has no name"},
		{"a name for an unnamed block data", []string{"BLOCK DATA", "END BLOCK DATA B"},
			"ENDNONAME: the END statement names B, but the BLOCK DATA has no name"},
		{"an inner procedure's plain END", []string{"SUBROUTINE S", "CONTAINS", "SUBROUTINE T", "END", "END"},
			"ENDNOKIND: the END of the SUBROUTINE T, which is inside another unit, must be END SUBROUTINE"},
		{"an inner procedure's wrong name", []string{"SUBROUTINE S", "CONTAINS", "SUBROUTINE T", "END SUBROUTINE U", "END"},
			"ENDNAME: the END statement names U, but the SUBROUTINE T is named T"},
		{"a module's", []string{"MODULE M", "END MODULE N"},
			"ENDNAME: the END statement names N, but the MODULE M is named M"},
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
