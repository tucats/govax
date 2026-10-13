package console

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Phase 54 program (docs/PHASE-54, subtask 11): testdata/lib54's
// lib54.mar calls every LIBRTL routine of the phase, case by case, and
// prints what each returned, signaled, and stored. lib54-govax.txt is
// govax's output, checked by hand against the manual's examples and the
// phase doc's survey; run with -update to rewrite it after a change
// that's meant. The same program is the probe for VMS, whose output,
// once run, replaces it as the oracle (as Phase 53's did).

var updateLib54 = flag.Bool("update", false, "rewrite testdata/lib54/lib54-govax.txt from govax's run of lib54.mar")

// lib54Dir is the probe's directory, from this package's.
var lib54Dir = filepath.Join("..", "..", "testdata", "lib54")

func TestLib54Program(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(lib54Dir, "lib54.mar"))
	if err != nil {
		t.Fatal(err)
	}

	r0, out := runMacroProgram(t, string(src))
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	got := strings.ReplaceAll(out, "\r", "")
	path := filepath.Join(lib54Dir, "lib54-govax.txt")

	if *updateLib54 {
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
