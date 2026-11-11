package fortran

import (
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/sema"
)

// The syntax of recursive routines (docs/planning/PHASE-89 - reentrant FORTRAN.md, subtask
// 2): which units /RECURSIVE, OPTIONS, and RECURSIVE make recursive. Since subtask 4
// a recursive unit is compiled with its storage in the frame, and its storage map's
// heading says it is RECURSIVE, so these tests read the headings.

// recursiveUnits compiles lines and lists, as "line:message", what it reports, then
// as "R:name" each routine compiled as a RECURSIVE one.
func recursiveUnits(t *testing.T, recursive bool, lines ...string) string {
	t.Helper()

	options := testOptions
	options.Recursive = recursive

	r := Compile(options, File{Spec: "T.FOR", Name: "T", Lines: lines})

	var got []string
	for _, m := range r.Messages {
		got = append(got, strconv.Itoa(m.Pos.Line)+":"+m.Ident)
	}

	for _, line := range r.MACRO {
		if rest, ok := strings.CutPrefix(line, "; Storage map for "); ok && strings.Contains(rest, "RECURSIVE") {
			name, _, _ := strings.Cut(rest, ",")
			got = append(got, "R:"+name)
		}
	}

	return strings.Join(got, " ")
}

func TestRecursiveUnits(t *testing.T) {
	program := []string{
		"      PROGRAM T",       // 1
		"      CALL S(3)",       // 2
		"      END",             // 3
		"      SUBROUTINE S(N)", // 4
		"      IF (N .GT. 0) CALL S(N - 1)",
		"      END",
		"      FUNCTION F(X)", // 7
		"      F = X",
		"      END",
		"      BLOCK DATA",
		"      END",
	}

	tests := []struct {
		name      string
		recursive bool
		lines     []string
		want      string
	}{
		{"not recursive", false, program, "5:RECURSIVE"},
		{"/RECURSIVE: subprograms, not the main program or BLOCK DATA", true, program, "R:S R:F"},
		{"RECURSIVE on the header", false, []string{
			"      RECURSIVE SUBROUTINE S(N)",
			"      IF (N .GT. 0) CALL S(N - 1)",
			"      END",
			"      SUBROUTINE U",
			"      END",
		}, "R:S"},
		{"OPTIONS /RECURSIVE for one unit", false, []string{
			"      OPTIONS /RECURSIVE",
			"      SUBROUTINE S(N)",
			"      CALL S(N)",
			"      END",
			"      SUBROUTINE U",
			"      CALL U",
			"      END",
		}, "6:RECURSIVE"}, // an error: no MACRO
		{"OPTIONS /NORECURSIVE against the command", true, []string{
			"      OPTIONS /NOREC",
			"      SUBROUTINE S",
			"      END",
			"      SUBROUTINE U",
			"      END",
		}, "R:U"},
		{"RECURSIVE wins over /NORECURSIVE", false, []string{
			"      OPTIONS /NORECURSIVE",
			"      INTEGER RECURSIVE FUNCTION K(N) RESULT(M)",
			"      M = 1",
			"      IF (N .GT. 1) M = N * K(N - 1)",
			"      END",
		}, "R:K"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := recursiveUnits(t, tt.recursive, tt.lines...); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestVFEsReentrant checks that a recursive unit's variable format expressions and
// NAMELIST groups compile with no message: nothing is left that it can't have.
func TestVFEsReentrant(t *testing.T) {
	got := recursiveUnits(t, false,
		"      RECURSIVE SUBROUTINE S(N)",
		"      INTEGER K, L",
		"      SAVE L",
		"      NAMELIST /G/ K",
		"      NAMELIST /H/ L",
		"      WRITE (6, 10) N",
		"   10 FORMAT (I<N>)",
		"      WRITE (6, H)",
		"      END",
		"      SUBROUTINE U(N)",
		"      NAMELIST /G/ K",
		"      WRITE (6, 10) N",
		"   10 FORMAT (I<N>)",
		"      END",
	)

	if want := "R:S"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestRecursiveStandard checks /STANDARD's flags for RECURSIVE and RESULT, and
// that RESULT's name is a name like any other.
func TestRecursiveStandard(t *testing.T) {
	standard, err := sema.Conformance{}.With(nil)
	if err != nil {
		t.Fatal(err)
	}

	options := testOptions
	options.Standard = standard

	r := Compile(options, File{Spec: "T.FOR", Name: "T", Lines: []string{
		"      RECURSIVE FUNCTION F(N) RESULT(ANSWERS)",
		"      ANSWERS = N",
		"      ENTRY G(N) RESULT(M)",
		"      M = N",
		"      END",
		"      RECURSIVE SUBROUTINE S",
		"      END",
	}})

	var got []string

	for _, m := range r.Messages {
		if m.Ident == "EXTENSION" {
			got = append(got, strconv.Itoa(m.Pos.Line)+" "+strings.TrimPrefix(m.Text, "extension to FORTRAN-77: "))
		}
	}

	want := []string{
		"1 a RECURSIVE subprogram",
		"1 RESULT",
		"1 the name ANSWERS, longer than six characters",
		"3 RESULT",
		"6 a RECURSIVE subprogram",
	}

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q\nwant %q", got, want)
	}
}
