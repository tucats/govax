package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Phase 113 probe of STR$'s arithmetic (docs/planning/PHASE-113, subtask
// 7): testdata/str113's str113.mar calls STR$ADD, STR$MUL, STR$DIVIDE,
// STR$RECIP, STR$ROUND, and STR$ELEMENT_R8 case by case. Until subtask 12 runs
// it on VMS, govax's run is held to the manuals' examples, the lines the
// manuals print; with vax/str113-vax.txt in place, it is held to VMS's output
// line for line, but for expectedStr113Differences.

// str113Compared turns on the comparison with VMS's run. VMS's output came back
// in Phase 113's subtask 12d; subtask 13 makes govax's routines VMS's and then
// sets it. Until then the run is held to the manuals' examples, and the lines
// that differ from VMS's are only counted.
const str113Compared = false

// expectedStr113Differences are the lines of VMS's output govax doesn't
// reproduce, by the case's name and the line's first word, each with why.
var expectedStr113Differences = map[string]string{}

// str113Manual is each manual example's case and the lines it prints: R0, the
// result's sign and exponent, and its digits.
var str113Manual = map[string][]string{
	"ADD manual":         {"ADD manual R0=00000001", "  CS: 00000001 FFFFFFFC", `  D: 7 "9999998"`},
	"MUL manual":         {"MUL manual R0=00000001", "  CS: 00000001 FFFFFFFF", `  D: 1 "2"`},
	"DIVIDE manual":      {"DIVIDE manual R0=00000001", "  CS: 00000001 FFFFFFFD", `  D: 10 "5000000000"`},
	"RECIP manual":       {"RECIP manual R0=00000001", "  CS: 00000001 FFFFFFFD", `  D: 1 "1"`},
	"ROUND manual":       {"ROUND manual R0=00000001", "  CS: 00000000 00000001", `  D: 3 "100"`},
	"ROUND manual trunc": {"ROUND manual trunc R0=00000001", "  CS: 00000000 00000000", `  D: 3 "999"`},
}

func TestStr113Probe(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "str113")

	src, err := os.ReadFile(filepath.Join(dir, "str113.mar"))
	if err != nil {
		t.Fatal(err)
	}

	r0, out := runMacroProgram(t, string(src))
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	gotLines := strings.Split(strings.ReplaceAll(out, "\r", ""), "\n")
	if path := os.Getenv("STR113_WRITE"); path != "" {
		if err := os.WriteFile(path, []byte(strings.Join(gotLines, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(filepath.Join(dir, "vax", "str113-vax.txt"))
	if os.IsNotExist(err) {
		holdToManual(t, gotLines)

		return
	} else if err != nil {
		t.Fatal(err)
	}

	if !str113Compared {
		holdToManual(t, gotLines)
		t.Logf("%d lines differ from VMS's run (str113Compared is off)",
			countDifferentLines(gotLines, strings.Split(string(want), "\n")))

		return
	}

	compareStr113(t, gotLines, strings.Split(string(want), "\n"))
}

// holdToManual checks the manuals' examples among govax's lines.
func holdToManual(t *testing.T, lines []string) {
	t.Helper()

	found := 0

	for i, line := range lines {
		name, _, ok := strings.Cut(line, " R0=")
		if !ok || strings.HasPrefix(line, " ") {
			continue
		}

		want, ok := str113Manual[name]
		if !ok {
			continue
		}

		found++

		for k, w := range want {
			if i+k >= len(lines) || lines[i+k] != w {
				t.Errorf("%s: line %d is %q, want %q", name, k+1, at(lines, i+k), w)
			}
		}
	}

	if found != len(str113Manual) {
		t.Errorf("found %d of the manuals' %d examples", found, len(str113Manual))
	}
}

// at is lines[i], or "" past the end.
func at(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}

	return ""
}

// countDifferentLines is how many lines of got and want differ, position by
// position.
func countDifferentLines(got, want []string) int {
	n := 0

	for i := range max(len(got), len(want)) {
		if at(got, i) != at(want, i) {
			n++
		}
	}

	return n
}

// compareStr113 holds govax's lines to VMS's, as TestStr83Oracle does.
func compareStr113(t *testing.T, gotLines, wantLines []string) {
	t.Helper()

	caseName := ""
	seen := map[string]bool{}

	for i := range max(len(gotLines), len(wantLines)) {
		g, w := at(gotLines, i), at(wantLines, i)

		if name, _, ok := strings.Cut(w, " R0="); ok && !strings.HasPrefix(w, " ") {
			caseName = name
		}

		if g == w {
			continue
		}

		key := caseName + "/" + strings.Fields(w + " ?")[0]
		if why, ok := expectedStr113Differences[key]; ok {
			seen[key] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	for key := range expectedStr113Differences {
		if !seen[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedStr113Differences", key)
		}
	}
}
