package dclcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fcalc, the Fortran 90 milestone program (docs/planning/PHASE-111 - Fortran
// 90 milestone.md), compiled here without a console, so a change to the
// compiler that breaks it shows quickly; internal/console's TestFcalc builds
// and runs it.

// fcalcSources is fcalc's directory.
var fcalcSources = filepath.Join("..", "..", "testdata", "fcalc")

// fcalcFiles are fcalc's sources by name, NAME.F90, read from testdata/fcalc,
// and the order compiles.txt compiles them in, a FORTRAN command each.
func fcalcFiles(t *testing.T) (map[string][]string, []string) {
	t.Helper()

	order, err := os.ReadFile(filepath.Join(fcalcSources, "compiles.txt"))
	if err != nil {
		t.Fatal(err)
	}

	files := map[string][]string{}
	names := strings.Fields(string(order))

	for _, name := range names {
		text, err := os.ReadFile(filepath.Join(fcalcSources, strings.ToLower(name)+".f90"))
		if err != nil {
			t.Fatal(err)
		}

		files[name+".F90"] = strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	}

	return files, names
}

// TestFcalcCompiles compiles every fcalc module in order, /G_FLOATING as
// fcalc must be, with and without /OPTIMIZE, and /CHECK=ALL (whose checks
// once crashed the compiler on FCSYMBOLS, Phase 111's subtask 3a): the
// compiler must say nothing.
func TestFcalcCompiles(t *testing.T) {
	files, order := fcalcFiles(t)

	for _, mode := range []struct {
		optimize bool
		check    []string
	}{{false, nil}, {true, nil}, {true, []string{"ALL"}}} {
		c, out := newFortranContextSized(t, files, 20000)

		for _, name := range order {
			err := c.Fortran(FortranOptions{Sources: FortranSources(name), GFloating: true, Optimize: mode.optimize,
				Check: mode.check, CheckGiven: mode.check != nil})
			if err != nil || out.Len() != 0 {
				t.Fatalf("/OPTIMIZE %v /CHECK=%v, FORTRAN %s: err %v, output:\n%s", mode.optimize, mode.check, name, err, out)
			}
		}

		if macro, _ := volumeFile(t, c, "FCALC.MAR"); len(macro) < 1000 {
			t.Errorf("/OPTIMIZE %v /CHECK=%v: FCALC.MAR has %d lines", mode.optimize, mode.check, len(macro))
		}
	}
}
