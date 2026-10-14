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

// The Phase 58 oracle (docs/PHASE-58, subtask 8): testdata/lib58's
// lib58.mar calls every LIBRTL routine of the phase, case by case, and
// prints what each returned, signaled, and stored, and what it passed
// the program's own routines. vax/lib58-vax.txt is its output on VMS,
// which govax's must match line for line, but for
// expectedLib58Differences. A line whose value changes from run to run
// starts "  ~": only what's before its colon is compared.

// lib58Dir is the probe's directory, from this package's.
var lib58Dir = filepath.Join("..", "..", "testdata", "lib58")

// expectedLib58Differences are the lines of VMS's output govax doesn't
// reproduce, each with why: the line VMS printed, by its case's name and
// the line's first word.
var expectedLib58Differences = map[string]string{}

// runLib58 assembles, links, and runs lib58.mar as lib58.com does on
// VMS: on a fresh RD51 volume labeled LIB58 mounted as DUA1, its
// [000000] the default directory, LIB58_DISK naming it, and NLA0:
// defined (as vax.init defines it).
func runLib58(t *testing.T) (uint32, string) {
	t.Helper()

	c := newBootableConsole(t)
	out := &bytes.Buffer{}
	c.Out = out
	dir := t.TempDir()
	disk := filepath.Join(dir, "lib58.dsk")

	if err := c.cmd().InitializeContainer(disk, 0, "LIB58", 0, "RD51"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA1", disk, true); err != nil {
		t.Fatal(err)
	}

	d := NewDispatcher(c, loadEvaxGrammar(t), nil)

	for _, line := range []string{
		"SET DEFAULT DUA1:[000000]",
		"DEFINE LIB58_DISK DUA1:",
		"DEFINE/DEVICE NLA0/DEVCLASS=MAILBOX/DEVTYPE=NULL/DEVCHAR=202702849/OWNUIC=65537/DEVBUFSIZ=512",
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	src, err := os.ReadFile(filepath.Join(lib58Dir, "lib58.mar"))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "lib58")
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

func TestLib58Oracle(t *testing.T) {
	r0, out := runLib58(t)
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	want, err := os.ReadFile(filepath.Join(lib58Dir, "vax", "lib58-vax.txt"))
	if errors.Is(err, os.ErrNotExist) {
		t.Logf("govax's output:\n%s", out)
		t.Skip("no VMS run yet (vax/lib58-vax.txt)")
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
		if why, ok := expectedLib58Differences[key]; ok {
			seen[key] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	for key := range expectedLib58Differences {
		if !seen[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedLib58Differences", key)
		}
	}
}
