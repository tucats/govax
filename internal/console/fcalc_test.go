package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// fcalc, the Fortran 90 milestone program (docs/planning/PHASE-111 - Fortran
// 90 milestone.md): a scientific calculator in modules, in testdata/fcalc.
// Each script there, name.in, is what a user types, and name.out what fcalc
// prints, worked by hand (and witnessed by gfortran's build of the same
// sources, with gfortran/fcplatform.f90). These tests build fcalc as its
// README says, /G_FLOATING, and run every script.

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

			got := shareRun(t, d, buf, "FCALC")
			if expected := string(want) + "  $STATUS == \"%X00000001\"\n"; got != expected {
				t.Errorf("FCALC on %s.in printed:\n%s\nwant:\n%s", name, got, expected)
			}
		})
	}
}

// TestFcalc builds fcalc the usual way, /OPTIMIZE, and runs every script.
func TestFcalc(t *testing.T) {
	d, c, buf := fcalcConsole(t, false)
	buildFcalc(t, d, c, buf, "")
	runFcalcScripts(t, d, c, buf)
}
