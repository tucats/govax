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
	"github.com/tucats/govax/internal/registers"
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
// the line's first word ("|" for a zone report's line). A line a routine
// wrote to SYS$OUTPUT itself, between "{{" and "}}", is its case's,
// whose line follows.
var expectedLib59Differences = map[string]string{
	// A call with too few arguments reads its next argument's address
	// past the argument list: on VMS a longword of the stack's, whose
	// fault shows that address, in govax 0.
	"CZ no arguments/CZ":         "the stack's contents",
	"CUZ no arguments/CUZ":       "the stack's contents",
	"STAT_VM 1 argument/STAT_VM": "the stack's contents",
	"GV 1 argument/GV":           "the stack's contents",

	// The user argument is the real zone's ID, an address, which the
	// report shows in decimal (so the mask doesn't cover it).
	"SVZ monitor level 3/|": "the real zone's address, in decimal",

	// A damaged block's dump shows its bytes as text too: its links,
	// addresses, are characters the mask doesn't cover.
	"SVZ X5 level 3/|": "a list link's address as text",
	"SVZ X6 level 3/|": "a list link's address as text",
	"SVZ X8 level 3/|": "a free-list link's address as text",

	// Where a C block that has to move goes, and the C heap's place in
	// P0, are VMS's own (docs/PHASE-59, subtask 10).
	"VM_REALLOC of 0/B_M8-B_M1:":                     "the C heap's layout",
	"the C blocks and the default zone's/B_M1-B_D1:": "the C heap's place in P0",
	"GV default after the C blocks/B_D4-B_M3:":       "the C heap's place in P0",

	// govax refuses to free pages a zone's area holds (the zone's
	// blocks are in them); VMS frees them.
	"FP X9's area page/FP": "a zone's pages refused",

	// The heap's totals: every count each case shows matches, but VMS
	// counts some of the RTL's own work that the run doesn't show one by
	// one (docs/PHASE-59, subtask 10): its totals differ from govax's by
	// a call or two and a few bytes and pages.
	"STAT_VM 1/V:":                    "the heap's totals",
	"SHOW_VM to SYS$OUTPUT/197":       "the heap's totals",
	"SHOW_VM 4 to SYS$OUTPUT/66":      "the page totals",
	"SHOW_VM omitted/|":               "the heap's totals",
	"SHOW_VM 0/|":                     "the heap's totals",
	"SHOW_VM 1/|":                     "the heap's totals",
	"SHOW_VM 2/|":                     "the heap's totals",
	"SHOW_VM 3/|":                     "the heap's totals",
	"SHOW_VM 4/|":                     "the page totals",
	"SHOW_VM 5/|":                     "the page totals",
	"SHOW_VM 7/|":                     "the page totals",
	"SHOW_VM 0 action fails/|":        "the heap's totals",
	"SHOW_VM 4 action fails/|":        "the page totals",
	"SHOW_VM user argument omitted/|": "the heap's totals",
	"SHOW_VM 4 arguments/|":           "the heap's totals",
}

// lib59Owners returns each line's case: the case whose line it follows,
// or, for a line between "{{" and "}}", the case whose line follows the
// "}}" (a routine's own output comes before its case's line).
func lib59Owners(lines []string) []string {
	owners := make([]string, len(lines))
	caseName := ""

	for i, line := range lines {
		if name, _, ok := strings.Cut(line, " R0="); ok && !strings.HasPrefix(line, " ") {
			caseName = name
		}

		owners[i] = caseName
	}

	for i := 0; i < len(lines); i++ {
		if lines[i] != "{{" {
			continue
		}

		end := i
		for end < len(lines) && lines[end] != "}}" {
			end++
		}

		if end+1 < len(lines) {
			for k := i; k <= end; k++ {
				owners[k] = owners[end+1]
			}
		}

		i = end
	}

	return owners
}

// hexWord is an 8-digit hex number in a zone report's line.
var hexWord = regexp.MustCompile(`\b[0-9A-F]{8}\b`)

// runLib59 assembles, links, and runs lib59.mar as lib59.com does on
// VMS. A HALT in user mode is a privileged instruction fault, as on a
// VAX, not govax's debugging stop: a case calls data as a routine.
func runLib59(t *testing.T) (uint32, string) {
	t.Helper()

	c := newBootableConsole(t)
	c.CPU.SetDebug(c.CPU.Debug() &^ registers.DebugUserHalt)
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

	// LIB59_OUT names a file for govax's output, for study.
	if path := os.Getenv("LIB59_OUT"); path != "" {
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
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
	owners := lib59Owners(wantLines)
	seen := map[string]bool{}

	for i := range max(len(gotLines), len(wantLines)) {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}

		caseName := ""

		if i < len(wantLines) {
			w, caseName = wantLines[i], owners[i]
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
