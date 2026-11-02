package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Phase 83 oracle for STR$ (docs/PHASE-83, subtask 9): testdata/str83's
// str83.mar calls the 26 string routines and STR$UPCASE, case by case, and
// prints what each returned, signaled, and stored. vax/str83-vax.txt is its
// output on VMS (V7.1 on simh's VAX), which govax's must match line for line,
// but for expectedStr83Differences.

// expectedStr83Differences are the lines of VMS's output govax doesn't
// reproduce, by the case's name and the line's first word, each with why.
var expectedStr83Differences = map[string]string{
	// These JSB entries leave what VMS's own code had in R1 to R6: a buffer on
	// its stack for DUPL_CHAR, its heap's values for the other two.
	"DUPL_CHAR (JSB)/REGS:": "a stack address",
	"REPLACE (JSB)/REGS:":   "heap internals",
	"GET1_DX (JSB)/REGS:":   "heap internals",
}

func TestStr83Oracle(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "str83")

	src, err := os.ReadFile(filepath.Join(dir, "str83.mar"))
	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile(filepath.Join(dir, "vax", "str83-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	r0, out := runMacroProgram(t, string(src))
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
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
		if why, ok := expectedStr83Differences[key]; ok {
			seen[key] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	for key := range expectedStr83Differences {
		if !seen[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedStr83Differences", key)
		}
	}
}
