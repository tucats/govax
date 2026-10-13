package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Phase 54 oracle (docs/PHASE-54, subtask 11): testdata/lib54's
// lib54.mar calls every LIBRTL routine of the phase, case by case, and
// prints what each returned, signaled, and stored. vax/lib54-vax.txt is
// its output on VMS (V7.1 on simh's VAX 8600, 2026-10-13), which govax's
// must match line for line, but for expectedLib54Differences.

// lib54Dir is the probe's directory, from this package's.
var lib54Dir = filepath.Join("..", "..", "testdata", "lib54")

// expectedLib54Differences are the lines of VMS's output govax doesn't
// reproduce, each with why: the line VMS printed, by its case's name and
// the line's first word.
var expectedLib54Differences = map[string]string{
	// The dynamic-string routines' JSB entries leave whatever VMS's heap
	// code left in R1 to R6 (the suffix's "may change R0 to R6"): values
	// of its own allocator's, which govax's heap doesn't have. govax
	// changes only R0.
	"SGET1_DD_R6 7 (JSB)/REGS:": "heap internals",
	"SGET1_DD_R6 3 (JSB)/REGS:": "heap internals",
	"SFREE1_DD6 (JSB)/REGS:":    "heap internals",
	"SFREEN_DD6 1 (JSB)/REGS:":  "heap internals",
}

func TestLib54Oracle(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(lib54Dir, "lib54.mar"))
	if err != nil {
		t.Fatal(err)
	}

	r0, out := runMacroProgram(t, string(src))
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	want, err := os.ReadFile(filepath.Join(lib54Dir, "vax", "lib54-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	gotLines := strings.Split(strings.ReplaceAll(out, "\r", ""), "\n")
	wantLines := strings.Split(string(want), "\n")
	caseName := ""
	seen := map[string]bool{}

	for i := range max(len(gotLines), len(wantLines)) {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}

		if i < len(wantLines) {
			w = wantLines[i]
		}

		// A case's line starts in column 1, its name before " R0=".
		if name, _, ok := strings.Cut(w, " R0="); ok && !strings.HasPrefix(w, " ") {
			caseName = name
		}

		if g == w {
			continue
		}

		key := caseName + "/" + strings.Fields(w + " ?")[0]
		if why, ok := expectedLib54Differences[key]; ok {
			seen[key] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	for key := range expectedLib54Differences {
		if !seen[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedLib54Differences", key)
		}
	}
}
