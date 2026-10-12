package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Phase 53 oracle (docs/PHASE-53, subtasks 12 and 13): testdata/
// lib53's lib53.mar calls every LIBRTL routine of the phase, case by
// case, and prints what each returned, signaled, and stored.
// vax/lib53-vax.txt is its output on VMS (V7.1 on simh's VAX 8600,
// 2026-10-12), which govax's must match line for line.

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

func TestLib53Oracle(t *testing.T) {
	got := runLib53(t)

	want, err := os.ReadFile(filepath.Join(lib53Dir, "vax", "lib53-vax.txt"))
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
