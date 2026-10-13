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

// Why VMS's output differs from govax's for the cases in
// expectedLib56Cases (docs/PHASE-56's progress log, the probe's second
// run).
const (
	// VMS V7.1's LIB$CONVERT_DATE_STRING returned SS$_ROPRAND (a reserved
	// operand, as a status) for nearly every string with a letter in it or
	// beginning with punctuation, the manual's own Table 3-2 examples
	// among them, on simh's VAX 8600. Suspected a fault inside the routine
	// or the emulator, not a rule to follow: govax converts them.
	ropRand = "VMS returned SS$_ROPRAND"

	// "19880113 091325.14" by "!Y4!MN0!D0 !H04!M0!S0.!C2": VMS returned
	// LIB$_BADBLOADR, a heap error, after the ROPRAND cases before it.
	badBlockAddress = "VMS returned LIB$_BADBLOADR"

	// The difference from TODAY's result, which VMS didn't make (ROPRAND).
	todayFailed = "TODAY failed on VMS"
)

// expectedLib56Cases are the cases whose output govax doesn't match, by
// name, each with why. Every line of such a case may differ; each must
// differ somewhere, or it is listed for nothing.
var expectedLib56Cases = map[string]string{
	"CONVERT -MAR":                 ropRand,
	"CONVERT -SEPTEMBER":           ropRand,
	"CONVERT -1988":                ropRand,
	"CONVERT :45:":                 ropRand,
	"CONVERT ::23":                 ropRand,
	"CONVERT .01":                  ropRand,
	"CONVERT whole":                ropRand,
	"CONVERT whole flags 0":        ropRand,
	"CONVERT date only":            ropRand,
	"CONVERT no seconds":           ropRand,
	"CONVERT no seconds colon":     ropRand,
	"CONVERT one-digit hour":       ropRand,
	"CONVERT one-digit fraction":   ropRand,
	"CONVERT three-digit fraction": ropRand,
	"CONVERT lower case":           ropRand,
	"CONVERT full month":           ropRand,
	"CONVERT month ja":             ropRand,
	"CONVERT month ju":             ropRand,
	"CONVERT month ma":             ropRand,
	"CONVERT month jun":            ropRand,
	"CONVERT blanks":               ropRand,
	"CONVERT tab":                  ropRand,
	"CONVERT two blanks":           ropRand,
	"CONVERT no blank":             ropRand,
	"CONVERT blank in date":        ropRand,
	"CONVERT 29 Feb 1989":          ropRand,
	"CONVERT hour 24":              ropRand,
	"CONVERT minute 60":            ropRand,
	"CONVERT year 1858":            ropRand,
	"CONVERT year 88":              ropRand,
	"CONVERT slashes":              ropRand,
	"CONVERT hello":                ropRand,
	"CONVERT trailing junk":        ropRand,
	"CONVERT -MAR default flags":   ropRand,
	"CONVERT no seconds flags 0":   ropRand,
	"CONVERT high flag bits":       ropRand,
	"CONVERT DEFV2 -MAR":           ropRand,
	"CONVERT DEFV2 :45":            ropRand,
	"CONVERT DEFV2 date only":      ropRand,
	"CONVERT 12-hour am":           ropRand,
	"CONVERT 12-hour pm":           ropRand,
	"CONVERT 12-hour 12 am":        ropRand,
	"CONVERT 12-hour 12 pm":        ropRand,
	"CONVERT 12-hour p":            ropRand,
	"CONVERT 12-hour short":        ropRand,
	"CONVERT 12-hour no meridiem":  ropRand,
	"CONVERT 12-hour pm only":      ropRand,
	"CONVERT 12-hour 13 pm":        ropRand,
	"CONVERT 12-hour 0 am":         ropRand,
	"CONVERT 12-hour no blank":     ropRand,
	"CONVERT packed":               badBlockAddress,
	"CONVERT ticks":                ropRand,
	"CONVERT ticks short":          ropRand,
	"CONVERT ticks 8 digits":       ropRand,
	"CONVERT names standard":       ropRand,
	"CONVERT static nothing":       ropRand,
	"CONVERT TODAY":                ropRand,
	"CONVERT YESTERDAY":            ropRand,
	"CONVERT TOMORROW":             ropRand,
	"CONVERT today lower":          ropRand,
	"CONVERT tom":                  ropRand,
	"CONVERT y":                    ropRand,
	"CONVERT T":                    ropRand,
	"CONVERT TO":                   ropRand,
	"CONVERT TODAY 10:00":          ropRand,
	"CONVERT TODAY DEFV2":          ropRand,
	"CONVERT TODAY flags 0":        ropRand,
	"CONVERT 10:00 no defaults":    todayFailed,
	"CONVERT -MAR no defaults":     ropRand,
}

// runLib56 assembles, links, and runs lib56.mar as lib56.com does on
// VMS, with SYS$LANGUAGE ENGLISH in the system table as the VAX had it
// (vax/lib56.log). govax's system table has LIB$DT_FORMAT and
// LIB$DT_INPUT_FORMAT as the VAX's did, built in.
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

		if why, ok := expectedLib56Cases[caseName]; ok {
			if !seen[caseName] {
				t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)
			}

			seen[caseName] = true

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	for name := range expectedLib56Cases {
		if !seen[name] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedLib56Cases", name)
		}
	}
}
