package fortran

import (
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/sema"
)

// The syntax of recursive routines (docs/PHASE-89 - reentrant FORTRAN.md, subtask
// 2): which units /RECURSIVE, OPTIONS, and RECURSIVE make recursive. Until the
// frame holds a recursive unit's storage (subtask 4), each such unit is analyzed
// and then reported NOTIMPL at its header, so these tests read the NOTIMPLs.

// recursiveUnits compiles lines and lists, as "line:message", what it reports.
func recursiveUnits(t *testing.T, recursive bool, lines ...string) string {
	t.Helper()

	options := testOptions
	options.Recursive = recursive

	r := Compile(options, File{Spec: "T.FOR", Name: "T", Lines: lines})

	var got []string
	for _, m := range r.Messages {
		got = append(got, strconv.Itoa(m.Pos.Line)+":"+m.Ident)
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
		{"/RECURSIVE: subprograms, not the main program or BLOCK DATA", true, program, "4:NOTIMPL 7:NOTIMPL"},
		{"RECURSIVE on the header", false, []string{
			"      RECURSIVE SUBROUTINE S(N)",
			"      IF (N .GT. 0) CALL S(N - 1)",
			"      END",
			"      SUBROUTINE U",
			"      END",
		}, "1:NOTIMPL"},
		{"OPTIONS /RECURSIVE for one unit", false, []string{
			"      OPTIONS /RECURSIVE",
			"      SUBROUTINE S(N)",
			"      CALL S(N)",
			"      END",
			"      SUBROUTINE U",
			"      CALL U",
			"      END",
		}, "6:RECURSIVE 2:NOTIMPL"}, // the NOTIMPLs come after analysis
		{"OPTIONS /NORECURSIVE against the command", true, []string{
			"      OPTIONS /NOREC",
			"      SUBROUTINE S",
			"      END",
			"      SUBROUTINE U",
			"      END",
		}, "4:NOTIMPL"},
		{"RECURSIVE wins over /NORECURSIVE", false, []string{
			"      OPTIONS /NORECURSIVE",
			"      INTEGER RECURSIVE FUNCTION K(N) RESULT(M)",
			"      M = 1",
			"      IF (N .GT. 1) M = N * K(N - 1)",
			"      END",
		}, "2:NOTIMPL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := recursiveUnits(t, tt.recursive, tt.lines...); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
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
