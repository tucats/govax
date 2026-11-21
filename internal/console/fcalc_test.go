package console

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// fcalc, the Fortran 90 milestone program (docs/planning/PHASE-111 - Fortran
// 90 milestone.md): a scientific calculator in modules, in testdata/fcalc.
// Each script there, name.in, is what a user types, and name.out what fcalc
// prints, worked by hand (and witnessed by gfortran's build of the same
// sources, with gfortran/fcplatform.f90). These tests build fcalc as its
// README says, /G_FLOATING, and run every script, with the files of fcalc's
// commands the scripts run (testdata/fcalc/files) beside it; then the files
// the scripts wrote are held to testdata/fcalc/expected, and oneshot.com runs
// fcalc as a foreign command.

// fcalcDirectory is fcalc's directory.
var fcalcDirectory = filepath.Join("..", "..", "testdata", "fcalc")

// fcalcConsole is a console with a volume large enough for fcalc's files on
// DUA3:, the default directory.
func fcalcConsole(t *testing.T, paging bool) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	d, c, buf := shareFindConsole(t, paging)
	mountFreshContainerSized(t, c, "DUA3:", 20000)
	shareFindRun(t, d, "SET DEFAULT DUA3:[000000]")
	buf.Reset()

	return d, c, buf
}

// buildFcalc copies fcalc's sources to the volume and compiles each (with
// FORTRAN's qualifiers, after /G_FLOATING) in compiles.txt's order, assembles
// it, and links FCALC.EXE. Each step must say nothing.
func buildFcalc(t *testing.T, d *Dispatcher, c *Console, buf *bytes.Buffer, qualifiers string) {
	t.Helper()

	order, err := os.ReadFile(filepath.Join(fcalcDirectory, "compiles.txt"))
	if err != nil {
		t.Fatal(err)
	}

	modules := strings.Fields(string(order))

	for _, name := range modules {
		text, err := os.ReadFile(filepath.Join(fcalcDirectory, strings.ToLower(name)+".f90"))
		if err != nil {
			t.Fatal(err)
		}

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA3:[000000]" + name + ".F90"},
			rms.TextRecords, splitSource(string(text))); err != nil {
			t.Fatal(err)
		}
	}

	copyFcalcFiles(t, c)

	var commands []string

	for _, name := range modules {
		commands = append(commands, "FORTRAN/G_FLOATING"+qualifiers+" "+name)
	}

	for _, name := range modules {
		commands = append(commands, "MACRO "+name)
	}

	// The main program first, so the image is named for it.
	commands = append(commands, "LINK "+modules[len(modules)-1]+","+strings.Join(modules[:len(modules)-1], ","))

	buf.Reset()

	for _, command := range commands {
		shareFindRun(t, d, command)

		if out := buf.String(); out != "" {
			t.Fatalf("%s said:\n%s", command, out)
		}
	}
}

// copyFcalcFiles copies the files of fcalc's commands the scripts run,
// testdata/fcalc/files, to the default directory.
func copyFcalcFiles(t *testing.T, c *Console) {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(fcalcDirectory, "files", "*"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no files in %s (%v)", filepath.Join(fcalcDirectory, "files"), err)
	}

	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		name := "DUA3:[000000]" + strings.ToUpper(filepath.Base(file))
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: name},
			rms.TextRecords, splitSource(string(text))); err != nil {
			t.Fatal(err)
		}
	}
}

// fcalcTime is a date and time as fcalc writes one in a file's heading,
// which the expected files have as fcalcTimeMask.
var fcalcTime = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}`)

const fcalcTimeMask = "YYYY-MM-DD hh:mm:ss"

// runFcalcScripts runs FCALC on each script, its input from the script, and
// holds what it prints to the script's .out file, followed by $STATUS.
func runFcalcScripts(t *testing.T, d *Dispatcher, c *Console, buf *bytes.Buffer) {
	t.Helper()

	scripts, err := filepath.Glob(filepath.Join(fcalcDirectory, "*.in"))
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no scripts in %s (%v)", fcalcDirectory, err)
	}

	for _, script := range scripts {
		name := strings.TrimSuffix(filepath.Base(script), ".in")

		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(script)
			if err != nil {
				t.Fatal(err)
			}

			want, err := os.ReadFile(strings.TrimSuffix(script, ".in") + ".out")
			if err != nil {
				t.Fatal(err)
			}

			c.In = strings.NewReader(string(input))

			got := fcalcTime.ReplaceAllString(shareRun(t, d, buf, "FCALC"), fcalcTimeMask)
			if expected := string(want) + "  $STATUS == \"%X00000001\"\n"; got != expected {
				t.Errorf("FCALC on %s.in printed:\n%s\nwant:\n%s", name, got, expected)
			}
		})
	}
}

// checkFcalcFiles holds each file the scripts wrote to its copy in
// testdata/fcalc/expected, the times in their headings masked.
func checkFcalcFiles(t *testing.T, c *Console) {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(fcalcDirectory, "expected", "*"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no files in %s (%v)", filepath.Join(fcalcDirectory, "expected"), err)
	}

	for _, file := range files {
		want, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		name := "DUA3:[000000]" + strings.ToUpper(filepath.Base(file))

		records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: name}, rms.TextRecords)
		if err != nil {
			t.Errorf("%s: %v", name, err)

			continue
		}

		var got strings.Builder

		for _, record := range records {
			got.WriteString(fcalcTime.ReplaceAllString(string(record), fcalcTimeMask) + "\n")
		}

		if got.String() != string(want) {
			t.Errorf("%s holds:\n%s\nwant:\n%s", name, got.String(), want)
		}
	}
}

// runFcalcOneShot runs oneshot.com, which runs fcalc as a foreign command,
// and holds what it prints, and $STATUS after it, to oneshot.out.
func runFcalcOneShot(t *testing.T, d *Dispatcher, c *Console, buf *bytes.Buffer) {
	t.Helper()

	text, err := os.ReadFile(filepath.Join(fcalcDirectory, "oneshot.com"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA3:[000000]ONESHOT.COM"},
		rms.TextRecords, splitSource(string(text))); err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile(filepath.Join(fcalcDirectory, "oneshot.out"))
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	if err := d.Dispatch("@ONESHOT"); err != nil && !vmserrors.MessageInhibited(err) {
		fmt.Fprintf(buf, "%%%s\n", err) // as cmd/govax shows it
	}

	shareFindRun(t, d, "SHOW SYMBOL $STATUS")

	got := fcalcTime.ReplaceAllString(buf.String(), fcalcTimeMask)

	if got != string(want) {
		t.Errorf("@ONESHOT printed:\n%s\nwant:\n%s", got, want)
	}
}

// TestFcalc builds fcalc the usual way, /OPTIMIZE, and runs every script,
// then checks the files they wrote and runs oneshot.com.
func TestFcalc(t *testing.T) {
	d, c, buf := fcalcConsole(t, false)
	buildFcalc(t, d, c, buf, "")
	runFcalcScripts(t, d, c, buf)
	checkFcalcFiles(t, c)
	runFcalcOneShot(t, d, c, buf)
}
