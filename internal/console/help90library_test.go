package console

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// help90Dir is the Phase 90 probe (docs/planning/PHASE-90 - VMS help.md), whose part A
// is LIBRARY/HELP on its own help files.
const help90Dir = "../../testdata/help90/"

// help90Inputs are the files part A reads.
var help90Inputs = []string{
	"h90a1.hlp", "h90a2.hlp", "alpha2.hlp", "h90b.hlp", "h90c.hlp", "h90g.hlp", "h90s.hlp", "h90d.hlp",
	"badtext.hlp", "skip.hlp", "noblank.hlp", "toolong.hlp", "lower.hlp", "digits.hlp", "zero.hlp",
	"t1.txt", "t2.txt", "h90m.mar",
}

// help90Left are part A's commands govax doesn't run as VMS 7.1 did, each
// with its listing:
//
//   - H90X8: VMS 7.1 failed to insert GONLY after SKIP's bad key line, a
//     librarian bug govax doesn't copy; TestHelp90LibraryGoesOn checks
//     govax's way.
var help90Left = []string{"H90X8"}

// help90Directory takes out of DIRECTORY/SIZE=ALL's listing what isn't
// govax's to match: H90X8, which help90Left leaves out, and the totals that
// count it; and H90R.HLB;2's blocks used, which VMS 7.1's DCX data
// reduction made fewer (15) where govax's map leaves its records as they
// are (lbr.Rebuild).
func help90Directory(lines []string) []string {
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "H90X8."):
			continue
		case strings.HasPrefix(line, "Total of "):
			line = "Total of (files and blocks)"
		case strings.HasPrefix(line, "H90R.HLB;2 "):
			line = "H90R.HLB;2 (data reduced)"
		}

		out = append(out, line)
	}

	return out
}

// help90PartA is part A of HELP90.COM, verified, without help90Left's
// commands.
func help90PartA(t *testing.T) string {
	t.Helper()

	text, err := os.ReadFile(help90Dir + "help90.com")
	if err != nil {
		t.Fatal(err)
	}

	_, part, ok := strings.Cut(string(text), "$ ! Part A: LIBRARY/HELP.\n")
	if !ok {
		t.Fatal("no part A")
	}

	part, _, ok = strings.Cut(part, "$ !\n$ ! Part B")
	if !ok {
		t.Fatal("part A doesn't end as it did")
	}

	var lines []string

	for line := range strings.Lines(part) {
		if !help90Leaves(line) {
			lines = append(lines, strings.TrimRight(line, "\n"))
		}
	}

	return "$ SET NOON\n$ SET VERIFY\n$ ! Part A: LIBRARY/HELP.\n" + strings.Join(lines, "\n") + "\n$ SET NOVERIFY\n"
}

func help90Leaves(command string) bool {
	for _, s := range help90Left {
		if strings.Contains(command, s) {
			return true
		}
	}

	return false
}

// help90PartALog is VMS's log of part A, without help90Left's commands and
// their output.
func help90PartALog(t *testing.T) []string {
	t.Helper()

	text, err := os.ReadFile(help90Dir + "vax/round1b/help90.log")
	if err != nil {
		t.Fatal(err)
	}

	_, part, ok := strings.Cut(strings.ReplaceAll(string(text), "\r", ""), "$ ! Part A: LIBRARY/HELP.\n")
	if !ok {
		t.Fatal("the log has no part A")
	}

	part, _, ok = strings.Cut(part, "$ !\n$ ! Part B")
	if !ok {
		t.Fatal("the log's part A doesn't end as it did")
	}

	kept := []string{"$ ! Part A: LIBRARY/HELP."}
	skipping := false

	for line := range strings.SplitSeq(strings.TrimSuffix(part, "\n"), "\n") {
		if strings.HasPrefix(line, "$ ") {
			skipping = help90Leaves(line)
		}

		if !skipping {
			kept = append(kept, line)
		}
	}

	return kept
}

// help90Volume makes a volume like the VAX's exchange volume, DUA1, with
// part A's input files, and makes [000000] the default.
func help90Volume(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	d, c, buf := newCommandDispatcher(t)
	// Part A makes some 50 files, more headers than mountClusteredContainer's
	// small volume has.
	path := filepath.Join(t.TempDir(), "help90.dsk")
	if err := c.cmd().InitializeContainer(path, 4000, "HELP90", 3, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA1", path, true); err != nil {
		t.Fatal(err)
	}

	commands := []string{"SET DEFAULT DUA1:[000000]"}
	for _, name := range help90Inputs {
		abs, err := filepath.Abs(help90Dir + name)
		if err != nil {
			t.Fatal(err)
		}

		commands = append(commands, `COPY "`+abs+`"/HOST `+strings.ToUpper(name))
	}

	for _, line := range commands {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	buf.Reset()

	return d, c, buf
}

// TestHelp90LibraryOracle runs HELP90.COM's part A, LIBRARY/HELP on the
// probe's files, and holds what it writes to VMS 7.1's log: the listings,
// LIBRARIAN's messages, and what /EXTRACT wrote. Times, file IDs, the
// librarian's name, and the count of history records (govax keeps none) are
// masked, as in TestFor83LibraryOracle.
func TestHelp90LibraryOracle(t *testing.T) {
	d, _, buf := help90Volume(t)

	host := filepath.Join(t.TempDir(), "parta.com")
	writeFile(t, host, help90PartA(t))

	if err := d.Dispatch(`COPY "` + host + `"/HOST PARTA.COM`); err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	if err := d.Dispatch("@PARTA"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@PARTA: %v\n%s", err, buf)
	}

	var run []string

	for line := range strings.SplitSeq(strings.TrimRight(strings.ReplaceAll(buf.String(), "\r", ""), "\n"), "\n") {
		if line != "$ SET NOVERIFY" {
			run = append(run, line)
		}
	}

	got := for83Mask(run)
	want := help90Directory(for83Mask(help90PartALog(t)))
	got = help90Directory(got)

	for i := 0; i < len(got) || i < len(want); i++ {
		g, w := "<end>", "<end>"

		if i < len(got) {
			g = got[i]
		}

		if i < len(want) {
			w = want[i]
		}

		if g != w {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, g, w)
		}
	}
}

// TestHelp90LibraryGoesOn checks what VMS 7.1 got wrong in H90X8: after
// SKIP.HLP's bad key line, LIBRARY reads the next file and inserts its
// modules, and the library lists and extracts them.
func TestHelp90LibraryGoesOn(t *testing.T) {
	d, _, buf := help90Volume(t)

	err := d.Dispatch("LIBRARY/CREATE/HELP H90X8 H90C,SKIP,H90G")

	var cond vmserrors.ConditionError
	if !errors.As(err, &cond) || cond.Status&7 != 2 || !vmserrors.MessageInhibited(err) {
		t.Errorf("status %v, want an error whose message is shown", err)
	}

	if got, want := buf.String(), "%LIBRAR-E-ILLKEYLVL, illegal key level 3; key THREE in DUA1:[000000]SKIP.HLP;1; expected 1\n"; got != want {
		t.Errorf("messages %q, want %q", got, want)
	}

	buf.Reset()

	for _, line := range []string{"LIBRARY/LIST/HELP H90X8", "LIBRARY/EXTRACT=GONLY/OUTPUT=SYS$OUTPUT/HELP H90X8"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	_, listing, _ := strings.Cut(buf.String(), "\n\n")
	if want := "CONLY\nGONLY\n1 GONLY\n   Text of GONLY, in library H90G (the group table's).\n"; listing != want {
		t.Errorf("listing and extract:\n%s\nwant:\n%s", listing, want)
	}
}
