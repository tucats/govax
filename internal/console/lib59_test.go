package console

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
)

// The Phase 59 oracle (docs/PHASE-59, subtask 10): testdata/lib59's
// lib59.mar calls every LIBRTL memory routine of the phase, case by
// case, and prints what each returned, signaled, and stored, and what it
// passed the program's own routines. vax/lib59-vax.txt is its output on
// VMS, which govax's must match line for line, but for
// expectedLib59Differences. A line whose value changes from run to run
// starts "  ~": only what's before its colon is compared. The zone
// reports' lines ("  |", and those a routine writes to SYS$OUTPUT
// itself, between "{{" and "}}") are compared with each 8-digit hex
// number masked, since they show addresses.

// lib59Dir is the probe's directory, from this package's.
var lib59Dir = filepath.Join("..", "..", "testdata", "lib59")

// expectedLib59Differences are the lines of VMS's output govax doesn't
// reproduce, each with why: the line VMS printed, by its case's name and
// the line's first word.
var expectedLib59Differences = map[string]string{}

// hexWord is an 8-digit hex number in a zone report's line.
var hexWord = regexp.MustCompile(`\b[0-9A-F]{8}\b`)

// runLib59 assembles, links, and runs lib59.mar as lib59.com does on
// VMS.
func runLib59(t *testing.T) (uint32, string) {
	t.Helper()

	c := newBootableConsole(t)
	out := &bytes.Buffer{}
	c.Out = out
	dir := t.TempDir()

	src, err := os.ReadFile(filepath.Join(lib59Dir, "lib59.mar"))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "lib59")
	writeHostFile(t, path+".mar", string(src))

	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: path + ".mar"}); err != nil {
		t.Fatalf("MACRO: %v\n%s", err, out.String())
	}

	if err := c.cmd().Link(dclcmd.LinkOptions{Objects: []string{path}}); err != nil {
		t.Fatalf("LINK: %v\n%s", err, out.String())
	}

	r0 := runImageBounded(t, c, path+".exe", 50_000_000)
	_, after, _ := strings.Cut(out.String(), "Kernel initialized\n\r")

	return r0, strings.ReplaceAll(after, "\r", "")
}

// maskLib59 masks the addresses in the zone reports' lines.
func maskLib59(lines []string) []string {
	out := make([]string, len(lines))
	raw := false

	for i, line := range lines {
		switch {
		case line == "{{":
			raw = true
		case line == "}}":
			raw = false
		case raw || strings.HasPrefix(line, "  |"):
			line = hexWord.ReplaceAllString(line, "XXXXXXXX")
		}

		out[i] = line
	}

	return out
}

func TestLib59Oracle(t *testing.T) {
	r0, out := runLib59(t)
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	want, err := os.ReadFile(filepath.Join(lib59Dir, "vax", "lib59-vax.txt"))
	if errors.Is(err, os.ErrNotExist) {
		t.Logf("govax's output:\n%s", out)
		t.Skip("no VMS run yet (vax/lib59-vax.txt)")
	}

	if err != nil {
		t.Fatal(err)
	}

	gotLines := maskLib59(strings.Split(out, "\n"))
	wantLines := maskLib59(strings.Split(strings.ReplaceAll(string(want), "\r", ""), "\n"))
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

		// A value that varies: compare its name only.
		if strings.HasPrefix(w, "  ~") {
			wn, _, _ := strings.Cut(w, ":")
			gn, _, _ := strings.Cut(g, ":")

			if wn == gn {
				continue
			}
		}

		if g == w {
			continue
		}

		key := caseName + "/" + strings.Fields(w + " ?")[0]
		if why, ok := expectedLib59Differences[key]; ok {
			seen[key] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	for key := range expectedLib59Differences {
		if !seen[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedLib59Differences", key)
		}
	}
}
