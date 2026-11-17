package fortran_test

import (
	"slices"
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
// Nothing reads it yet but the source pass, so a free-form file's NOTIMPL is all
// there is to see of the form; /F90's effect on fixed form waits for subtask 4.
func TestF90Default(t *testing.T) {
	fixed := fortran.File{Spec: "A.FOR", Name: "A", Lines: []string{"      PROGRAM A", "      END"}}
	free := fortran.File{Spec: "A.F90", Name: "A", Lines: []string{"program a", "end program a"}, Free: true}

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

	if r := fortran.Compile(opts, free); !slices.Equal(ids(r), []string{"NOTIMPL"}) || r.MACRO != nil {
		t.Errorf("free form: %v, want only NOTIMPL", ids(r))
	}

	opts.F90, opts.F90Given = true, true
	if r := fortran.Compile(opts, fixed); r.Errors != 0 || r.MACRO == nil {
		t.Errorf("fixed form under /F90: %v", ids(r))
	}
}
