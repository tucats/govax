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

// The Phase 55 oracle (docs/PHASE-55, subtask 8): testdata/lib55's
// lib55.mar calls every LIBRTL routine of the phase, case by case, and
// prints what each returned, signaled, and stored. vax/lib55-vax.txt is
// its output on VMS, which govax's must match line for line, but for
// expectedLib55Differences. A line whose value changes from run to run
// starts "  ~": only what's before its colon is compared.

// lib55Dir is the probe's directory, from this package's.
var lib55Dir = filepath.Join("..", "..", "testdata", "lib55")

// expectedLib55Differences are the lines of VMS's output govax doesn't
// reproduce, each with why: the line VMS printed, by its case's name and
// the line's first word.
var expectedLib55Differences = map[string]string{
	// LIB$SYS_FAO's 300 bytes into a 400-byte varying string: VMS set
	// CURLEN 256 but left zeros at the start of the text, where govax
	// copies the 256 X's; unexplained (the case shows only the first
	// longword).
	"SYS_FAO 300 into VS 400/D_V:": "VMS's text unexplained",

	// LIB$SYS_ASCTIM of a delta of 10,001 days: VMS returned SS$_IVTIME
	// but wrote 42 characters, unseen, to the string and its length;
	// govax writes nothing on a failure.
	"SYS_ASCTIM 10001 days/D:": "VMS's text unseen",
	"SYS_ASCTIM 10001 days/L:": "VMS's text unseen",

	// A mounted volume's owner is the process that mounted it, whose PID
	// differs between the systems (VMS's 00000115, govax's process 1).
	"GETDVI PID/V:": "the mounter's PID",
	"GETDVI PID/D:": "the mounter's PID",

	// VMS's output went to a file (direct I/O), govax's to the terminal
	// (buffered I/O), so the buffered I/O count's width differs.
	"SHOW_TIMER/TIMDSC":        "output device",
	"SHOW_TIMER 3/TIMDSC":      "output device",
	"SHOW_TIMER code 0/TIMDSC": "output device",

	// A wait rounded to nothing: VMS's clock moved a tick (10ms) meanwhile,
	// govax's quantum clock didn't.
	"WAIT 0.004/Q:": "clock granularity",
}

// runLib55 assembles, links, and runs lib55.mar as lib55.com does on
// VMS: with LIB55_DISK naming a mounted RD51 volume labeled LIB55 (the
// exchange volume's twin), NLA0: defined (as vax.init defines it),
// SYS$LANGUAGE ENGLISH in the system table (as on the VAX: lib55.log),
// and no host locale.
func runLib55(t *testing.T) (uint32, string) {
	t.Helper()

	t.Setenv("LC_ALL", "C")

	c := newBootableConsole(t)
	out := &bytes.Buffer{}
	c.Out = out
	dir := t.TempDir()
	disk := filepath.Join(dir, "lib55.dsk")

	if err := c.cmd().InitializeContainer(disk, 0, "LIB55", 0, "RD51"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA1", disk, true); err != nil {
		t.Fatal(err)
	}

	// LIB55_DISK, and the null device as vax.init defines it.
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)

	for _, line := range []string{
		"DEFINE LIB55_DISK DUA1:",
		"DEFINE/SYSTEM SYS$LANGUAGE ENGLISH", // as on the VAX the probe ran on
		"DEFINE/DEVICE NLA0/DEVCLASS=MAILBOX/DEVTYPE=NULL/DEVCHAR=202702849/OWNUIC=65537/DEVBUFSIZ=512",
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	src, err := os.ReadFile(filepath.Join(lib55Dir, "lib55.mar"))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "lib55")
	writeHostFile(t, path+".mar", string(src))

	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: path + ".mar"}); err != nil {
		t.Fatalf("MACRO: %v\n%s", err, out.String())
	}

	if err := c.cmd().Link(dclcmd.LinkOptions{Objects: []string{path}}); err != nil {
		t.Fatalf("LINK: %v\n%s", err, out.String())
	}

	r0 := runImageBounded(t, c, path+".exe", 5_000_000)
	_, after, _ := strings.Cut(out.String(), "Kernel initialized\n\r")

	return r0, strings.ReplaceAll(after, "\r", "")
}

func TestLib55Oracle(t *testing.T) {
	r0, out := runLib55(t)
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	want, err := os.ReadFile(filepath.Join(lib55Dir, "vax", "lib55-vax.txt"))
	if errors.Is(err, os.ErrNotExist) {
		t.Logf("govax's output:\n%s", out)
		t.Skip("no VMS run yet (vax/lib55-vax.txt)")
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
		if why, ok := expectedLib55Differences[key]; ok {
			seen[key] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	for key := range expectedLib55Differences {
		if !seen[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedLib55Differences", key)
		}
	}
}
