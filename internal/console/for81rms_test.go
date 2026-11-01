package console_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// The Phase 81 probe of RMS (docs/PHASE-81 - FORTRAN files.md, subtask 3):
// testdata/for81's for81r.mar runs RMS's services case by case and prints what
// each returned and left behind. vax/for81r-vax.txt is its output on VMS.
// TestFor81RMSOracle holds govax to it for the cases on sequential files and
// FOP's bits; the relative file cases wait for subtask 9.

var for81Dir = filepath.Join("..", "..", "testdata", "for81")

// runFor81R assembles, links, and runs the probe on a fresh volume, and returns
// its output, a line an element.
func runFor81R(t *testing.T) []string {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	dispatch := func(line string) string {
		t.Helper()

		out.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			out.WriteString("%" + err.Error() + "\n")
		}

		return strings.Join(programLines(out.String()), "\n")
	}

	dsk := filepath.Join(t.TempDir(), "for81.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FOR81`)
	dispatch(`MOUNT/WRITE DUA2 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA2:[000000]")

	src, err := filepath.Abs(filepath.Join(for81Dir, "for81r.mar"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + src + `"/HOST FOR81R.MAR`)

	if out := dispatch("MACRO/NOLIST FOR81R"); out != "" {
		t.Fatalf("MACRO: %s", out)
	}

	if out := dispatch("LINK/NOMAP FOR81R"); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	return strings.Split(dispatch("RUN FOR81R"), "\n")
}

// for81RCompared says which cases are compared: not the relative files, #2 to #24.
func for81RCompared(number int) bool {
	return number < 2 || number > 24
}

// for81RMasks takes out what doesn't compare. VMS's $CREATE leaves a channel number
// in FAB$L_STV, where govax opens no channel. The stat lines' FAB gets its MRN from
// the first relative file it opens, which is one of the cases not compared.
var for81RMasks = regexp.MustCompile(`( stv=)0000(?:00A0|0000)| mrn=\d+`)

// for81RCases splits lines into cases by their "#n" lines, keeping those
// for81RCompared takes, and the blank lines out.
func for81RCases(lines []string) []string {
	var out []string

	keep := true

	for _, line := range lines {
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#####") {
			keep = true

			continue
		}

		if text, ok := strings.CutPrefix(line, "#"); ok {
			word, _, _ := strings.Cut(text, " ")

			if n, err := strconv.Atoi(word); err == nil {
				keep = for81RCompared(n)
			}
		}

		if keep {
			out = append(out, for81RMasks.ReplaceAllString(line, "$1"))
		}
	}

	return out
}

// TestFor81RMSOracle holds the probe's sequential file and FOP cases to VMS's run.
func TestFor81RMSOracle(t *testing.T) {
	raw := runFor81R(t)

	if path := os.Getenv("FOR81R_OUT"); path != "" {
		if err := os.WriteFile(path, []byte(strings.Join(raw, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(filepath.Join(for81Dir, "vax", "for81r-vax.txt"))
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("no VMS run (vax/for81r-vax.txt)")
	}

	if err != nil {
		t.Fatal(err)
	}

	got := for81RCases(raw)
	lines := for81RCases(strings.Split(strings.ReplaceAll(string(want), "\r", ""), "\n"))

	for i := range max(len(got), len(lines)) {
		var g, w string
		if i < len(got) {
			g = got[i]
		}

		if i < len(lines) {
			w = lines[i]
		}

		if g != w {
			t.Errorf("line %d:\n vms   %q\n govax %q", i+1, w, g)
		}
	}
}
