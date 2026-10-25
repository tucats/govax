package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Phase 72 OTS$ oracle (docs/PHASE-72 - legacy images.md, subtask 6):
// testdata/ots72's ots72.mar calls LIBRTL's OTS$ floating conversions,
// moves, and dynamic strings, case by case, and prints what each
// returned, signaled, and stored. vax/ots72-vax.txt is its output on VMS
// (V7.1 on simh's VAX 8600, 2026-10-24), which govax's must match line
// for line, but for expectedOTS72Differences. (ots72t.mar, the study of
// FORRTL's calls, needs VMS's FORRTL.EXE, which can't be committed:
// TestOTS72TOracle runs it when a local volume has one, and librtl's
// TestOTSCvtForDigits checks its answers without FORRTL.)

// ots72Dir is the probe's directory, from this package's.
var ots72Dir = filepath.Join("..", "..", "testdata", "ots72")

// expectedOTS72Differences are the lines of VMS's output govax doesn't
// reproduce, each with why, by the case's name and the line's first word.
var expectedOTS72Differences = map[string]string{
	// VMS's H extension bits are off in the last bits ("calculations are
	// only carried to 128 bits", the manual says); govax's are exact.
	"CVT_T_H 0.1 extension/E:": "H's imprecise extension bits",

	// The dynamic-string JSB entries leave VMS's heap code's registers
	// in R1 to R6, as LIB$'s do (testdata/lib54).
	"SGET1_DD_R6 7 (JSB)/REGS:": "heap internals",
	"SFREE1_DD6 (JSB)/REGS:":    "heap internals",
	"SGET1_DD_R6 3 (JSB)/REGS:": "heap internals",
	"SFREEN_DD6 1 (JSB)/REGS:":  "heap internals",
}

func TestOTS72Oracle(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(ots72Dir, "ots72.mar"))
	if err != nil {
		t.Fatal(err)
	}

	r0, out := runMacroProgram(t, string(src))
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	want, err := os.ReadFile(filepath.Join(ots72Dir, "vax", "ots72-vax.txt"))
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
		if why, ok := expectedOTS72Differences[key]; ok {
			seen[key] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	for key := range expectedOTS72Differences {
		if !seen[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedOTS72Differences", key)
		}
	}
}
