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

// TestProcedureFormsPending checks that Phase 103's forms, which the parser
// builds since subtask 2, are each NOTIMPL in semantic analysis until the
// subtask that compiles them: CONTAINS, an interface block, a keyword argument
// in a CALL, and a defined operator.
func TestProcedureFormsPending(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"CONTAINS", []string{"program p", "call s", "contains", "subroutine s", "end subroutine", "end program"},
			"NOTIMPL: the CONTAINS statement is not implemented yet"},
		{"an interface block", []string{"program p", "interface", "subroutine s(x)", "end subroutine", "end interface", "end program"},
			"NOTIMPL: the INTERFACE statement is not implemented yet"},
		{"a keyword argument", []string{"program p", "call s(x, tol=1.0)", "end program"},
			"NOTIMPL: a keyword argument to a procedure that isn't intrinsic is not implemented yet"},
		{"a binary defined operator", []string{"program p", "y = a .cross. b", "end program"},
			"NOTIMPL: a defined operator is not implemented yet"},
		{"a unary defined operator", []string{"program p", "y = .inverse. a", "end program"},
			"NOTIMPL: a defined operator is not implemented yet"},
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
