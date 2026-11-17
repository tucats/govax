package fortran_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran"
)

// TestNoF90Explicit checks that /NOF90 given is the default for a fixed-form
// source: every corpus source compiles to the same MACRO file and messages
// (docs/planning/PHASE-99 - Fortran 90.md, Decision 2).
func TestNoF90Explicit(t *testing.T) {
	for _, path := range noF90Sources(t) {
		opts := noF90Options("X")
		opts.Optimize = true

		given := opts
		given.F90Given = true

		if noF90Digest(t, path, opts) != noF90Digest(t, path, given) {
			t.Errorf("%s: /NOF90 compiles otherwise than no /F90", noF90Key(path))
		}
	}
}

// TestF90Default checks /F90's default: the first file's form, unless given.
// Nothing reads it yet but the source pass's " rule, which the lexer doesn't
// share until subtask 4, so what there is to see is that both forms compile.
func TestF90Default(t *testing.T) {
	fixed := fortran.File{Spec: "A.FOR", Name: "A", Lines: []string{"      PROGRAM A", "      END"}}
	free := fortran.File{Spec: "A.F90", Name: "A", Lines: []string{"program a; i = 1", "end"}, Free: true}

	ids := func(r fortran.Result) []string {
		var out []string
		for _, m := range r.Messages {
			out = append(out, m.Ident)
		}

		return out
	}

	opts := noF90Options("A")

	if r := fortran.Compile(opts, fixed); r.Errors != 0 || r.MACRO == nil {
		t.Errorf("fixed form: %v", ids(r))
	}

	if r := fortran.Compile(opts, free); r.Errors != 0 || r.MACRO == nil {
		t.Errorf("free form: %v", ids(r))
	}

	opts.F90, opts.F90Given = true, true
	if r := fortran.Compile(opts, fixed); r.Errors != 0 || r.MACRO == nil {
		t.Errorf("fixed form under /F90: %v", ids(r))
	}
}

// TestF90LexicalItems compiles Fortran 90's lexical items through every pass
// (docs/planning/PHASE-99 - Fortran 90.md, subtask 4): the relational operators'
// new spellings compile as .EQ. and the rest, a quoted character constant, and
// Z'1F' and B'101' in DATA; a kind parameter is NOTIMPL until Phase 100.
func TestF90LexicalItems(t *testing.T) {
	free := fortran.File{Spec: "A.F90", Name: "A", Free: true, Lines: []string{
		"program a",
		"  integer i, j",
		"  data i /z'1f'/, j /b'101'/",
		`  if (i == 31 .and. j /= 4) print *, "it's ", i, j`,
		"end",
	}}

	r := fortran.Compile(noF90Options("A"), free)
	if r.Errors != 0 || r.MACRO == nil {
		t.Fatalf("messages: %v", r.Messages)
	}

	text := strings.Join(r.MACRO, "\n")
	for _, want := range []string{"\t.LONG\t31\t", "\t.LONG\t5\t", "\tCMPL\tA.I,#31\n\tBEQL", "\tCMPL\tA.J,#4\n\tBNEQ", "\t.ASCII\t/it's /"} {
		if !strings.Contains(text, want) {
			t.Errorf("the MACRO file has no %q", want)
		}
	}

	kinded := fortran.File{Spec: "B.F90", Name: "B", Free: true, Lines: []string{"x = 1.5_8", "end"}}

	r = fortran.Compile(noF90Options("B"), kinded)
	if len(r.Messages) != 1 || r.Messages[0].Ident != "NOTIMPL" || !strings.Contains(r.Messages[0].Text, "a kind parameter on a constant") {
		t.Errorf("a kind parameter: %v", r.Messages)
	}
}
