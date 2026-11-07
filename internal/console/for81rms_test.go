package console_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// The Phase 81 probe of RMS (docs/PHASE-81 - FORTRAN files.md, subtasks 3 and
// 9): testdata/for81's for81r.mar and for81d.mar run RMS's services case by
// case and print what each returned and left behind. vax/for81r-vax.txt and
// vax/for81d-vax.txt are their output on VMS, and vax/for81-blocks.txt and
// vax/for81c-blocks.txt the blocks of the files they made there.
// TestFor81RMSOracle and TestFor81RelativeOracle hold govax to them.

var for81Dir = filepath.Join("..", "..", "testdata", "for81")

// for81Run is a probe program's run on a fresh volume: its output, a line an
// element, and a way to give the console more commands after it.
type for81Run struct {
	lines    []string
	dispatch func(string) string
}

// runFor81RMS assembles, links, and runs the probe program name (FOR81R or
// FOR81D) on a fresh volume as DUA2.
func runFor81RMS(t *testing.T, name string) for81Run {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
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

	src, err := filepath.Abs(filepath.Join(for81Dir, strings.ToLower(name)+".mar"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + src + `"/HOST ` + name + `.MAR`)

	if out := dispatch("MACRO/NOLIST " + name); out != "" {
		t.Fatalf("MACRO: %s", out)
	}

	if out := dispatch("LINK/NOMAP " + name); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	return for81Run{lines: strings.Split(dispatch("RUN "+name), "\n"), dispatch: dispatch}
}

// for81RMasks takes out what doesn't compare: VMS's $CREATE leaves a channel
// number in FAB$L_STV, where govax opens no channel.
var for81RMasks = regexp.MustCompile(`( stv=)0000(?:00A0|0000)`)

// for81RCases takes out the blank lines and the section lines, and masks the
// rest.
func for81RCases(lines []string) []string {
	var out []string

	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#####") {
			continue
		}

		out = append(out, for81RMasks.ReplaceAllString(line, "$1"))
	}

	return out
}

// compareFor81 holds got to want line for line.
func compareFor81(t *testing.T, got, want []string) {
	t.Helper()

	for i := range max(len(got), len(want)) {
		var g, w string
		if i < len(got) {
			g = got[i]
		}

		if i < len(want) {
			w = want[i]
		}

		if g != w {
			t.Errorf("line %d:\n vms   %q\n govax %q", i+1, w, g)
		}
	}
}

// vmsLines reads a file of VMS's output, a line an element, or skips the test
// if it isn't there.
func vmsLines(t *testing.T, name string) []string {
	t.Helper()

	want, err := os.ReadFile(filepath.Join(for81Dir, "vax", name))
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("no VMS run (vax/" + name + ")")
	}

	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.ReplaceAll(string(want), "\r", ""), "\n")
}

// for81Oracle runs the probe program name and holds its output to VMS's,
// returning the run.
func for81Oracle(t *testing.T, name, vms, env string) for81Run {
	t.Helper()

	run := runFor81RMS(t, name)

	if path := os.Getenv(env); path != "" {
		if err := os.WriteFile(path, []byte(strings.Join(run.lines, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	compareFor81(t, for81RCases(run.lines), for81RCases(vmsLines(t, vms)))

	return run
}

// TestFor81RMSOracle holds round 1's RMS program, sequential and relative
// files and FOP's bits, to VMS's run, and the relative files it made to VMS's
// blocks.
func TestFor81RMSOracle(t *testing.T) {
	run := for81Oracle(t, "FOR81R", "for81r-vax.txt", "FOR81R_OUT")
	compareFor81Blocks(t, run, "for81-blocks.txt", regexp.MustCompile(`^R\d+\.DAT$`))
}

// TestFor81RelativeOracle holds round 2's RMS program, on deleted cells, how a
// relative file grows, its bucket size, and locks, to VMS's run, and its files
// to VMS's blocks.
func TestFor81RelativeOracle(t *testing.T) {
	run := for81Oracle(t, "FOR81D", "for81d-vax.txt", "FOR81D_OUT")
	compareFor81Blocks(t, run, "for81c-blocks.txt", regexp.MustCompile(`^[DEB]\d+\.DAT$`))
}

// dumpHeader is a DUMP's first line for a file.
var dumpHeader = regexp.MustCompile(`^Dump of file DUA2:\[000000\]([A-Z0-9]+\.DAT);1(?: on .*)?$`)

// fileIDLine is the line after it, whose file ID is the volume's, not the
// file's.
var fileIDLine = regexp.MustCompile(`^File ID \(\d+,\d+,\d+\)`)

// for81Dumps splits a file of DUMP/BLOCK output into its files' dumps, by
// name, leaving out the lines of zeros, as the VMS files' dumps were kept.
func for81Dumps(lines []string) map[string][]string {
	dumps := map[string][]string{}

	var name string

	for _, line := range lines {
		if m := dumpHeader.FindStringSubmatch(line); m != nil {
			name = m[1]

			continue
		}

		if name == "" || strings.TrimSpace(line) == "" || allZeroDumpLine(line) {
			continue
		}

		if strings.HasPrefix(line, "%") {
			name = ""

			continue
		}

		dumps[name] = append(dumps[name], fileIDLine.ReplaceAllString(line, "File ID"))
	}

	return dumps
}

// allZeroDumpLine reports whether line is a line of a block's bytes that are
// all zeros.
func allZeroDumpLine(line string) bool {
	fields := strings.Fields(line)
	if len(fields) != 10 {
		return false
	}

	for _, f := range fields[:8] {
		if f != "00000000" {
			return false
		}
	}

	return true
}

// compareFor81Blocks dumps each file of VMS's dumps in vms whose name matches
// on govax's volume and holds its blocks to VMS's.
func compareFor81Blocks(t *testing.T, run for81Run, vms string, names *regexp.Regexp) {
	t.Helper()

	want := for81Dumps(vmsLines(t, vms))
	compared := 0

	for name, blocks := range want {
		if !names.MatchString(name) {
			continue
		}

		compared++

		got := for81Dumps(strings.Split(run.dispatch("DUMP/BLOCK "+name), "\n"))[name]

		if strings.Join(got, "\n") != strings.Join(blocks, "\n") {
			t.Errorf("%s's blocks:\n--- vms\n%s\n--- govax\n%s", name, strings.Join(blocks, "\n"), strings.Join(got, "\n"))
		}
	}

	t.Logf("%d files compared", compared)

	if compared == 0 {
		t.Errorf("no files of %s to compare", vms)
	}
}
