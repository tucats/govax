package fortran_test

import (
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
