package sema

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/parse"
	"github.com/tucats/govax/internal/fortran/source"
)

// analyzeFreeWith is analyzeFree with options of the test's own (F90 is set).
func analyzeFreeWith(t *testing.T, opts Options, lines ...string) (*Program, []diag.Message) {
	t.Helper()

	var messages diag.List

	opts.F90 = true
	program := source.Read(source.Form{F90: true}, &source.File{Spec: "T.F90", Lines: lines, Free: true}, nil, &messages)
	file := parse.Parse(lex.Lex(program.Statements, &messages), &messages)
	prog := Analyze(file, "T$MAIN", opts, &messages)

	return prog, messages.Messages()
}

// messageText is the messages as "IDENT: text", joined by "; ".
func messageText(messages []diag.Message) string {
	got := make([]string, 0, len(messages))
	for _, m := range messages {
		got = append(got, m.Ident+": "+m.Text)
	}

	return strings.Join(got, "; ")
}

// TestInterfaceBlocks checks what semantic analysis says about interface
// blocks with no generic specification (interface.go): nothing for a program
// that is right, and each message for a mistake.
func TestInterfaceBlocks(t *testing.T) {
	// iface is an interface block describing S(X), X of the type given.
	iface := func(xType string) []string {
		return []string{"interface", "subroutine s(x)", xType + " x", "end subroutine", "end interface"}
	}

	lines := func(parts ...[]string) []string {
		var all []string
		for _, p := range parts {
			all = append(all, p...)
		}

		return all
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"a right call", lines([]string{"program p"}, iface("real"), []string{"call s(1.0)", "end program"}), ""},
		{"the argument count", lines([]string{"program p"}, iface("real"), []string{"call s(1.0, 2.0)", "end program"}),
			"ARGCOUNT: S takes 1 dummy argument (line 3), but is given 2 here"},
		{"a type", lines([]string{"program p"}, iface("real"), []string{"call s(1)", "end program"}),
			"ARGTYPE: argument 1 of S is an INTEGER, but its dummy argument X is a REAL (line 3)"},
		{"a kind", lines([]string{"program p"}, iface("real(8)"), []string{"call s(1.0)", "end program"}),
			"ARGTYPE: argument 1 of S is a REAL, but its dummy argument X is a REAL*8 (line 3)"},
		{"a function's type is the body's", []string{"program p", "interface", "character*3 function name(i)", "integer i",
			"end function", "end interface", "character*3 c", "c = name(2)", "x = name(2)", "end program"},
			"ASSIGNTYPE: a CHARACTER*3 value can't be assigned to X, which is REAL"},
		{"a dummy procedure", []string{"subroutine h(g)", "interface", "subroutine g(a)", "real a(3)", "end subroutine",
			"end interface", "call g(1.0)", "end subroutine"},
			"ARGRANK: argument 1 of G is a scalar, but its dummy argument A is an array of rank 1 (line 3)"},
		{"a dummy procedure's interface in a body", []string{"program p", "interface", "subroutine solve(f)", "interface",
			"real function f(x)", "end function", "end interface", "end subroutine", "end interface", "call solve(g)",
			"end program"}, ""},
		{"passed with no EXTERNAL", lines([]string{"program p"}, iface("real"), []string{"call t(s)", "end program"}), ""},
		{"no host in a body", []string{"program p", "integer, parameter :: k = 8", "interface", "subroutine s(x)",
			"real(k) x", "end subroutine", "end interface", "end program"},
			"KINDNOTCONST: a kind must be a constant integer expression, 0 or more"},
		{"the body's own IMPLICIT NONE", []string{"program p", "interface", "subroutine s(x)", "implicit none",
			"end subroutine", "end interface", "end program"},
			"NOTYPE: X has no type: with IMPLICIT NONE, every name must be declared"},
		{"a name declared already", lines([]string{"program p", "real s"}, iface("real"), []string{"end program"}),
			"INTERFACEDECL: S is already a name in this unit (line 2), so an interface body can't declare it"},
		{"EXTERNAL after the block", lines([]string{"program p"}, iface("real"), []string{"external s", "end program"}),
			"INTERFACEDECL: S is already an external procedure in this unit (line 7), so an interface body can't declare it"},
		{"two bodies of one name", lines([]string{"program p"}, iface("real"), iface("integer"), []string{"end program"}),
			"INTERFACEDECL: S is already a procedure with an interface body in this unit (line 3), so an interface body can't declare it"},
		{"a typed dummy", []string{"subroutine h(g)", "real g", "interface", "real function g(x)", "end function",
			"end interface", "end subroutine"},
			"INTERFACEDECL: G is already a name in this unit (line 1), so an interface body can't declare it"},
		{"the subprogram itself", []string{"subroutine h", "interface", "subroutine h", "end subroutine",
			"end interface", "end subroutine"},
			"INTERFACESELF: H is this subprogram's own procedure, so an interface body here can't describe it"},
		{"an internal procedure", lines([]string{"program p"}, iface("real"), []string{"contains", "subroutine s(x)",
			"end subroutine", "end program"}),
			"DUPINTERFACE: S is an internal procedure (line 8), whose interface its own statements give; it can't be declared here"},
		{"a definition that agrees", lines([]string{"program p"}, iface("real"), []string{"call s(1.0)", "end program",
			"subroutine s(y)", "real y", "end subroutine"}), ""},
		{"a definition of another type", lines([]string{"program p"}, iface("real"), []string{"end program",
			"subroutine s(y)", "integer y", "end subroutine"}),
			"INTERFACEDIFF: this interface body for S doesn't agree with its definition (line 8): its dummy argument 1 (X) is REAL here, INTEGER there"},
		{"a definition of another kind of unit", lines([]string{"program p"}, iface("real"), []string{"end program",
			"real function s(y)", "end function"}),
			"INTERFACEDIFF: this interface body for S doesn't agree with its definition (line 8): it is a SUBROUTINE here, a FUNCTION there"},
		{"a definition of other rank", lines([]string{"program p"}, iface("real"), []string{"end program",
			"subroutine s(y)", "real y(2)", "end subroutine"}),
			"INTERFACEDIFF: this interface body for S doesn't agree with its definition (line 8): its dummy argument 1 (X) is a scalar here, an array of rank 1 there"},
		{"a definition of another INTENT", lines([]string{"program p"}, iface("real, intent(in) ::"), []string{"end program",
			"subroutine s(y)", "real y", "end subroutine"}),
			"INTERFACEDIFF: this interface body for S doesn't agree with its definition (line 8): its dummy argument 1 (X) is INTENT(IN) here, of no INTENT there"},
		{"a definition with more dummies", lines([]string{"program p"}, iface("real"), []string{"end program",
			"subroutine s(y, z)", "end subroutine"}),
			"INTERFACEDIFF: this interface body for S doesn't agree with its definition (line 8): it has 1 dummy argument here, 2 dummy arguments there"},
		{"a function's value of another type", []string{"program p", "interface", "real function f()", "end function",
			"end interface", "end program", "integer function f()", "f = 1", "end function"},
			"INTERFACEDIFF: this interface body for F doesn't agree with its definition (line 7): its value is REAL here, INTEGER there"},
		{"an OPTIONAL dummy through an implicit interface", []string{"program p", "call s(1.0)", "end program",
			"subroutine s(y)", "real, optional :: y", "end subroutine"},
			"EXPLICITREQ: S has the OPTIONAL dummy argument Y (line 4), so a reference to it needs an explicit interface"},
		{"an OPTIONAL dummy through an explicit interface", []string{"program p", "interface", "subroutine s(y)",
			"real, optional :: y", "end subroutine", "end interface", "call s(1.0)", "end program",
			"subroutine s(y)", "real, optional :: y", "end subroutine"}, ""},
		{"in a BLOCK DATA", []string{"block data b", "interface", "subroutine s", "end subroutine", "end interface", "end block data"},
			"BLOCKSTMT: an INTERFACE statement can't be in a block data subprogram"},
		{"after an executable statement", []string{"program p", "x = 1"}, ""},
	}

	// The last case is completed here: an interface block among the
	// executable statements is a declaration out of place.
	cases[len(cases)-1].lines = lines(cases[len(cases)-1].lines, iface("real"), []string{"end program"})
	cases[len(cases)-1].want = "LATEDECL: a declaration must come before the executable statements, the first of which is at line 2"

	for _, c := range cases {
		_, messages := analyzeFreeWith(t, Options{}, c.lines...)
		if got := messageText(messages); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// TestInterfaceSymbols checks what an interface body makes of its name: an
// external procedure of the body's type, or a dummy procedure.
func TestInterfaceSymbols(t *testing.T) {
	prog, messages := analyzeFreeWith(t, Options{}, "subroutine h(g)", "interface", "integer function g(k)", "integer k",
		"end function", "character*4 function name()", "end function", "end interface", "character*4 c", "c = name()", "i = g(1)",
		"end subroutine")
	if len(messages) > 0 {
		t.Fatal(messageText(messages))
	}

	u := prog.Units[0]
	if len(u.Interfaces) != 2 {
		t.Fatalf("%d interfaces, want 2", len(u.Interfaces))
	}

	g, name := u.Lookup("G"), u.Lookup("NAME")
	if g.Interface != u.Interfaces[0] || !g.DummyProcedure() || g.Class != Function || g.Type != IntegerType {
		t.Errorf("G: interface %v, dummy procedure %v, %v %v", g.Interface != nil, g.DummyProcedure(), g.Class, g.Type)
	}

	if name.Interface != u.Interfaces[1] || !name.External || name.Class != Function || name.Type != (Type{Kind: Character, Size: 4}) {
		t.Errorf("NAME: interface %v, external %v, %v %v", name.Interface != nil, name.External, name.Class, name.Type)
	}

	// An external procedure with an interface body is named in the object only
	// if it is passed: a call names it itself.
	if name.Linked() {
		t.Error("NAME is linked, though never passed")
	}
}

// TestArgumentChecking checks /WARNINGS=ARGUMENT_CHECKING (the author's answer
// 1): references through an implicit interface to a routine of the file warn of
// a type, kind, or rank that isn't the dummy's, and only when asked for.
func TestArgumentChecking(t *testing.T) {
	lines := []string{"program p", "real v(3)", "call s(1, v, v(2))", "call s(1.0, 2.0, v)", "end program",
		"subroutine s(a, b, c)", "real a, b, c(3)", "end subroutine"}

	_, messages := analyzeFreeWith(t, Options{}, lines...)
	if got := messageText(messages); got != "" {
		t.Errorf("by default: got %q, want none", got)
	}

	_, messages = analyzeFreeWith(t, Options{Warnings: diag.Warnings{ArgumentChecking: true}}, lines...)

	want := "ARGTYPEW: argument 1 of S is an INTEGER, but its dummy argument A is a REAL (line 6); " +
		"ARGRANKW: argument 2 of S is an array of rank 1, but its dummy argument B is a scalar (line 6)"
	if got := messageText(messages); got != want {
		t.Errorf("ARGUMENT_CHECKING:\n got %q\nwant %q", got, want)
	}

	for _, m := range messages {
		if m.Severity != diag.Warning {
			t.Errorf("%s is %v, want a warning", m.Ident, m.Severity)
		}
	}
}
