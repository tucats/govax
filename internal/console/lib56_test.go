package console

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
)

// The Phase 56 oracle (docs/PHASE-56, subtask 8): testdata/lib56's
// lib56.mar calls every LIBRTL date and time routine, case by case, and
// prints what each returned, signaled, and stored. vax/lib56-vax.txt is
// its output on VMS, which govax's must match line for line, but for
// expectedLib56Differences. A line whose value changes from run to run
// starts "  ~": only what's before its colon is compared.

// lib56Dir is the probe's directory, from this package's.
var lib56Dir = filepath.Join("..", "..", "testdata", "lib56")

// expectedLib56Differences are the lines of VMS's output govax doesn't
// reproduce, each with why: the line VMS printed, by its case's name and
// the line's first word.
var expectedLib56Differences = map[string]string{}

// runLib56 assembles, links, and runs lib56.mar as lib56.com does on
// VMS, with SYS$LANGUAGE ENGLISH in the system table (as on the VAX the
// Phase 55 probe ran on).
func runLib56(t *testing.T) (uint32, string) {
	t.Helper()

	t.Setenv("LC_ALL", "C")

	c := newBootableConsole(t)
	out := &bytes.Buffer{}
	c.Out = out
	dir := t.TempDir()

	d := NewDispatcher(c, loadEvaxGrammar(t), nil)

	for _, line := range []string{
		"DEFINE/SYSTEM SYS$LANGUAGE ENGLISH",
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	src, err := os.ReadFile(filepath.Join(lib56Dir, "lib56.mar"))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "lib56")
	writeHostFile(t, path+".mar", string(src))

	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: path + ".mar"}); err != nil {
		t.Fatalf("MACRO: %v\n%s", err, out.String())
	}

	if err := c.cmd().Link(dclcmd.LinkOptions{Objects: []string{path}}); err != nil {
		t.Fatalf("LINK: %v\n%s", err, out.String())
	}

	r0 := runImageBounded(t, c, path+".exe", 20_000_000)
	_, after, _ := strings.Cut(out.String(), "Kernel initialized\n\r")

	return r0, strings.ReplaceAll(after, "\r", "")
}

func TestLib56Oracle(t *testing.T) {
	r0, out := runLib56(t)
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	want, err := os.ReadFile(filepath.Join(lib56Dir, "vax", "lib56-vax.txt"))
	if errors.Is(err, os.ErrNotExist) {
		t.Logf("govax's output:\n%s", out)
		t.Skip("no VMS run yet (vax/lib56-vax.txt)")
	}

	if err != nil {
		t.Fatal(err)
	}

	gotLines := strings.Split(out, "\n")
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
		if why, ok := expectedLib56Differences[key]; ok {
			seen[key] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	for key := range expectedLib56Differences {
		if !seen[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedLib56Differences", key)
		}
	}
}
