package console

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Phase 53 program (docs/PHASE-53, subtask 12): testdata/lib53's
// lib53.mar calls every LIBRTL routine of the phase, case by case, and
// prints what each returned, signaled, and stored. lib53-govax.txt is
// govax's output, checked by hand against the manual's examples and the
// phase doc's survey; run with -update to rewrite it after a change
// that's meant. The same program is the VMS probe (subtask 13).

var updateLib53 = flag.Bool("update", false, "rewrite testdata/lib53/lib53-govax.txt from govax's run of lib53.mar")

// lib53Dir is the probe's directory, from this package's.
var lib53Dir = filepath.Join("..", "..", "testdata", "lib53")

// runLib53 assembles, links, and runs lib53.mar, and returns its output
// with host line endings.
func runLib53(t *testing.T) string {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(lib53Dir, "lib53.mar"))
	if err != nil {
		t.Fatal(err)
	}

	r0, out := runMacroProgram(t, string(src))
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	return strings.ReplaceAll(out, "\r", "")
}

func TestLib53Program(t *testing.T) {
	got := runLib53(t)
	path := filepath.Join(lib53Dir, "lib53-govax.txt")

	if *updateLib53 {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")

	for i := range max(len(gotLines), len(wantLines)) {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}

		if i < len(wantLines) {
			w = wantLines[i]
		}

		if g != w {
			t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
		}
	}
}
