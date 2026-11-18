package sema

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/parse"
	"github.com/tucats/govax/internal/fortran/source"
)

// analyzeFree runs the passes through semantic analysis over lines as a
// free-form file compiled /F90.
func analyzeFree(t *testing.T, lines ...string) []diag.Message {
	t.Helper()

	var messages diag.List

	program := source.Read(source.Form{F90: true}, &source.File{Spec: "T.F90", Lines: lines, Free: true}, nil, &messages)
	file := parse.Parse(lex.Lex(program.Statements, &messages), &messages)
	Analyze(file, "T$MAIN", Options{F90: true}, &messages)

	return messages.Messages()
}

// TestProcedureFormsPending checks that what is left of Phase 103's forms, which
// the parser builds since subtask 2, is NOTIMPL in semantic analysis: MODULE
// PROCEDURE, Phase 106's. Every other form is compiled since subtask 8.
func TestProcedureFormsPending(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"MODULE PROCEDURE", []string{"program p", "interface g", "module procedure f", "end interface", "end program"},
			"NOTIMPL: the MODULE PROCEDURE statement is not implemented yet"},
		{"a record's field", []string{"program p", "structure /s/", "real b", "end structure",
			"record /s/ a", "y = a.b", "end program"}, ""},
	}

	for _, c := range cases {
		var got []string
		for _, m := range analyzeFree(t, c.lines...) {
			got = append(got, m.Ident+": "+m.Text)
		}

		if strings.Join(got, "; ") != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestInternalProcedures checks what semantic analysis says about internal
// procedures (internal.go): nothing for a program that is right, and each
// message for a mistake.
func TestInternalProcedures(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"host association", []string{"program p", "implicit integer (a-z)", "real x(3)", "namelist /g/ x",
			"structure /pt/", "real u", "end structure", "do i = 1, 3", "call s", "end do", "y = f(2.0)",
			"contains", "subroutine s", "record /pt/ q", "x(i) = q.u", "write (*, nml=g)", "end subroutine",
			"real function f(v)", "real v", "f = v + i", "end function", "end program"}, ""},
		{"an internal procedure hides a unit of the file", []string{"program p", "call s", "contains",
			"subroutine s", "end subroutine", "end program", "subroutine s(a, b)", "end subroutine"}, ""},
		{"two of one name", []string{"program p", "contains", "subroutine s", "end subroutine",
			"subroutine s", "end subroutine", "end program"},
			"DUPNAME: S is already an internal procedure in this unit, at line 3"},
		{"the host's name", []string{"subroutine h", "contains", "subroutine h", "end subroutine", "end subroutine"},
			"DUPNAME: H is already the name of this subroutine in this unit, at line 1"},
		{"a type statement", []string{"program p", "real f", "contains", "real function f()", "f = 1",
			"end function", "end program"},
			"DUPINTERFACE: F is an internal procedure (line 4), whose interface its own statements give; it can't be declared here"},
		{"EXTERNAL", []string{"program p", "external s", "contains", "subroutine s", "end subroutine", "end program"},
			"DUPINTERFACE: S is an internal procedure (line 4), whose interface its own statements give; it can't be declared here"},
		{"an actual argument", []string{"program p", "call x(s)", "contains", "subroutine s", "end subroutine", "end program"},
			"INTERNALARG: the internal procedure S can't be an actual argument"},
		{"the argument count", []string{"program p", "call s(1)", "contains", "subroutine s(a, b)",
			"end subroutine", "end program"}, "ARGCOUNT: S takes 2 dummy arguments (line 4), but is given 1 here"},
		{"a type", []string{"program p", "call s(1)", "contains", "subroutine s(a)", "real a",
			"end subroutine", "end program"},
			"ARGTYPE: argument 1 of S is an INTEGER, but its dummy argument A is a REAL (line 4)"},
		{"a kind", []string{"program p", "double precision d", "call s(d)", "contains", "subroutine s(a)", "real a",
			"end subroutine", "end program"},
			"ARGTYPE: argument 1 of S is a REAL*8, but its dummy argument A is a REAL (line 5)"},
		{"an array to a scalar", []string{"program p", "real v(3)", "call s(v)", "contains", "subroutine s(a)",
			"end subroutine", "end program"},
			"ARGRANK: argument 1 of S is an array of rank 1, but its dummy argument A is a scalar (line 5)"},
		{"a scalar to an array", []string{"program p", "call s(2.0)", "contains", "subroutine s(a)", "real a(2)",
			"end subroutine", "end program"},
			"ARGRANK: argument 1 of S is a scalar, but its dummy argument A is an array of rank 1 (line 4)"},
		{"an element to an array", []string{"program p", "real v(3)", "call s(v(2))", "contains", "subroutine s(a)",
			"real a(2)", "end subroutine", "end program"}, ""},
		{"a length", []string{"program p", "character*2 c", "call s(c)", "contains", "subroutine s(a)",
			"character*4 a", "end subroutine", "end program"},
			"ARGLENGTH: argument 1 of S is 2 characters long, but its dummy argument A is 4 (line 5)"},
		{"INTENT(OUT)", []string{"program p", "call s(1.0 + x)", "contains", "subroutine s(a)",
			"real, intent(out) :: a", "a = 1", "end subroutine", "end program"},
			"NOTDEFINABLE: argument 1 of S must be a variable, since its dummy argument A is INTENT(OUT) (line 4)"},
		{"a recursive one's call of itself", []string{"program p", "call s(1)", "contains", "recursive subroutine s(n)",
			"if (n < 3) call s(n + 1, 2)", "end subroutine", "end program"},
			"ARGCOUNT: S takes 1 dummy argument (line 4), but is given 2 here"},
		{"a function's type is its own", []string{"program p", "character*3 c", "c = f()", "contains",
			"character*3 function f()", "f = 'abc'", "end function", "end program"}, ""},
		{"the host's IMPLICIT NONE", []string{"program p", "implicit none", "contains", "subroutine s",
			"k = 1", "end subroutine", "end program"}, "NOTYPE: K has no type: with IMPLICIT NONE, every name must be declared"},
		{"letters the host gave", []string{"program p", "implicit real (a-z)", "contains", "subroutine s",
			"implicit integer (k)", "k = 1", "end subroutine", "end program"}, ""},
		{"letters after the host's IMPLICIT NONE", []string{"program p", "implicit none", "contains", "subroutine s",
			"implicit integer (k)", "k = 1", "end subroutine", "end program"}, ""},
		{"a RECURSIVE host", []string{"recursive subroutine h", "call s", "contains", "subroutine s",
			"end subroutine", "end subroutine"}, ""},
		{"an AUTOMATIC host", []string{"subroutine h", "real, automatic :: a", "a = 1", "call s", "contains",
			"subroutine s", "a = 2", "end subroutine", "end subroutine"}, ""},
		{"a bound the host uses later", []string{"subroutine h(v)", "real v(5)", "n = 5", "call s(v)", "contains",
			"subroutine s(a)", "real a(n)", "end subroutine", "end subroutine"},
			"NOTIMPL: a name its host uses only in executable statements, in an internal procedure's declarations is not implemented yet"},
	}

	for _, c := range cases {
		var got []string
		for _, m := range analyzeFree(t, c.lines...) {
			got = append(got, m.Ident+": "+m.Text)
		}

		if strings.Join(got, "; ") != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestHostAssociationShares checks the four steps' meaning: an internal
// procedure's undeclared I, which its host first uses in its executable
// statements, is the host's I; a name the procedure declares is its own though
// its host has one; one neither has is the procedure's own.
func TestHostAssociationShares(t *testing.T) {
	var messages diag.List

	lines := []string{"program p", "do i = 1, 3", "call s", "end do", "k = 0", "contains", "subroutine s",
		"integer k", "do i = 1, 2", "end do", "k = 1", "m = 2", "end subroutine", "end program"}
	program := source.Read(source.Form{F90: true}, &source.File{Spec: "T.F90", Lines: lines, Free: true}, nil, &messages)
	file := parse.Parse(lex.Lex(program.Statements, &messages), &messages)
	prog := Analyze(file, "T$MAIN", Options{F90: true}, &messages)

	if messages.Failed() || len(prog.Units) != 2 {
		t.Fatalf("%d units, messages %v", len(prog.Units), messages.Messages())
	}

	host, inner := prog.Units[0], prog.Units[1]

	if inner.Host != host || len(host.Contains) != 1 || host.Contains[0] != inner || inner.Path() != "P.S" {
		t.Fatalf("S isn't P's internal procedure: host %v, path %s", inner.Host, inner.Path())
	}

	if inner.Lookup("I") != host.Lookup("I") {
		t.Error("S's I isn't its host's")
	}

	if inner.Lookup("K") == host.Lookup("K") {
		t.Error("S's declared K is its host's")
	}

	if m := inner.Lookup("M"); m == nil || host.Lookup("M") != nil {
		t.Error("S's M isn't its own")
	}

	if s := host.Lookup("S"); s == nil || s.Internal != inner || s.Class != Subroutine {
		t.Errorf("the host's S doesn't stand for its internal procedure: %+v", s)
	}
}
