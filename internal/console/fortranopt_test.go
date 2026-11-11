package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// readMacro is a MACRO file's lines on DUA1, but for the two header comments that
// differ from one compilation to the next: when it was made, and by what command.
func readMacro(t *testing.T, c *Console, name string) []string {
	t.Helper()

	return macroLines(t, c, rms.FileLocation{Name: "DUA1:[000000]" + name + ".MAR"})
}

// macroLines is readMacro for a MACRO file anywhere.
func macroLines(t *testing.T, c *Console, loc rms.FileLocation) []string {
	t.Helper()

	records, _, err := c.ContainerSession.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	var lines []string

	for _, r := range records {
		line := string(r)
		if !strings.HasPrefix(line, "; Compiled by") && !strings.HasPrefix(line, "; by the command") {
			lines = append(lines, line)
		}
	}

	return lines
}

// TestFortranOptimizeQualifier checks /[NO]OPTIMIZE: the default is /OPTIMIZE,
// whose header says it ran, and /NOOPTIMIZE's file doesn't mention the optimizer.
func TestFortranOptimizeQualifier(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]OPTQ.FOR"},
		rms.TextRecords, splitSource("      I = 1\n      END\n")); err != nil {
		t.Fatal(err)
	}

	compile := func(qualifier string) []string {
		shareFindRun(t, d, "FORTRAN"+qualifier+" OPTQ")

		if out := buf.String(); out != "" {
			t.Fatalf("FORTRAN%s said:\n%s", qualifier, out)
		}

		return readMacro(t, c, "OPTQ")
	}

	plain := strings.Join(compile(""), "\n")

	if !strings.Contains(plain, "; Optimized by") {
		t.Errorf("the default's file doesn't say it was optimized:\n%s", plain)
	}

	if got := strings.Join(compile("/OPTIMIZE"), "\n"); got != plain {
		t.Errorf("/OPTIMIZE's file differs from the default's:\n%s\nand:\n%s", got, plain)
	}

	if got := strings.Join(compile("/NOOPTIMIZE"), "\n"); strings.Contains(got, "OPTIMIZE'S") || strings.Contains(got, "Optimized by") {
		t.Errorf("/NOOPTIMIZE's file mentions the optimizer:\n%s", got)
	}
}

// dungeonSource is where the author keeps DUNGEON's source and data, which stay
// out of the repository (docs/planning/PHASE-86 - FORTRAN milestone.md).
func dungeonSource() string {
	home, _ := os.UserHomeDir()

	return filepath.Join(home, "Documents", "Technical Doc", "VMS", "dungeon-3.2A")
}

// dungeonModules are its source files, in LINK's order.
var dungeonModules = []string{"dungeon", "game", "gdt", "objects", "parser", "rooms", "subr", "timefnc", "verbs"}

// TestFortranDungeonBothWays plays a scripted game of DUNGEON compiled each way,
// which must print the same (Decision 10 of docs/planning/PHASE-84 - FORTRAN optimizer.md):
// the one large program to hand, and the optimizer's default depends on it. It
// skips without DUNGEON's source. ITIME is fixed (testdata/opt84/measure), so the
// game's random numbers are the same each run. What it prints is only compared
// between the builds, never kept; Phase 86's test checks the game itself.
func TestFortranDungeonBothWays(t *testing.T) {
	dir := dungeonSource()
	if _, err := os.Stat(filepath.Join(dir, "dungeon.f")); err != nil {
		t.Skipf("no DUNGEON source: %v", err)
	}

	kit := filepath.Join("..", "..", "testdata", "opt84", "measure")

	input, err := os.ReadFile(filepath.Join(kit, "game.in"))
	if err != nil {
		t.Fatal(err)
	}

	var (
		printed string
		before  runCounts
	)

	for _, mode := range optimizerModes {
		t.Run(mode.name, func(t *testing.T) {
			got, cost := playDungeon(t, dir, kit, mode.qualifier, string(input))

			if printed == "" {
				printed, before = got, cost

				return
			}

			if got != printed {
				t.Errorf("DUNGEON's game printed something different with /OPTIMIZE (%d bytes, %d without)", len(got), len(printed))
			}

			logGain(t, "DUNGEON", before, cost)
		})
	}
}

// dungeonConsole is a console whose default directory is on a volume large enough
// for DUNGEON, DUA3:[000000].
func dungeonConsole(t *testing.T, paging bool) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	d, c, buf := shareFindConsole(t, paging)

	path := filepath.Join(t.TempDir(), "dungeon.dsk")
	if err := c.cmd().InitializeContainer(path, 60000, "DUNGEON", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA3:", path, true); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "SET DEFAULT DUA3:[000000]")

	return d, c, buf
}

// playDungeon builds DUNGEON with FORTRAN's qualifier on a large volume of its
// own, then plays the game script input, and returns what it printed and cost.
func playDungeon(t *testing.T, dir, kit, qualifier, input string) (string, runCounts) {
	t.Helper()

	d, c, buf := dungeonConsole(t, false)

	create := func(name string, kind rms.RecordKind, records [][]byte) {
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA3:[000000]" + name}, kind, records); err != nil {
			t.Fatal(name, err)
		}
	}

	text := func(host, name string) {
		data, err := os.ReadFile(host)
		if err != nil {
			t.Fatal(err)
		}

		create(name, rms.TextRecords, splitSource(string(data)))
	}

	run := func(command string) {
		shareFindRun(t, d, command)

		if out := buf.String(); out != "" {
			t.Fatalf("%s said %d bytes, first: %.200s", command, len(out), out)
		}
	}

	text(filepath.Join(dir, "dparam.for"), "DPARAM.FOR")
	text(filepath.Join(dir, "dindx"), "DINDX.DAT")
	text(filepath.Join(kit, "mkdtx.for"), "MKDTX.FOR")
	text(filepath.Join(kit, "itime.for"), "ITIME.FOR")

	// DTEXT.DAT is fixed-length records, which MKDTX makes from its variable-length
	// copy here.
	raw, err := os.ReadFile(filepath.Join(dir, "dtext"))
	if err != nil {
		t.Fatal(err)
	}

	var records [][]byte

	for ; len(raw) >= 80; raw = raw[80:] {
		records = append(records, raw[:80])
	}

	create("DTEXTV.DAT", rms.VariableRecords, records)

	buf.Reset()

	for _, command := range []string{"FORTRAN MKDTX", "MACRO MKDTX", "LINK MKDTX", "RUN MKDTX", "FORTRAN ITIME", "MACRO ITIME"} {
		run(command)
	}

	var objects []string

	for _, module := range dungeonModules {
		name := strings.ToUpper(module)
		text(filepath.Join(dir, module+".f"), name+".FOR")
		run("FORTRAN" + qualifier + "/CONTINUATIONS=40 " + name)
		run("MACRO " + name)

		objects = append(objects, name)
	}

	run("LINK/EXECUTABLE=DUNGEON " + strings.Join(objects, ",") + ",ITIME")

	c.In = bytes.NewReader([]byte(input))

	return countedRun(t, d, c, buf, "DUNGEON")
}
