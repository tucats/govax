package dclcmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fcalc, the Fortran 90 milestone program (docs/planning/PHASE-111 - Fortran
// 90 milestone.md), compiled here without a console, so a change to the
// compiler that breaks it shows quickly; internal/console's
// TestFcalcMilestone builds and runs it.

// fcalcSources is fcalc's sources' directory, internal/fcalc/src.
var fcalcSources = filepath.Join("..", "fcalc", "src")

// fcalcFiles are fcalc's sources by name, NAME.F90, read from internal/fcalc/src,
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

// fcalcToolMessages are what /STANDARD=F90 and /WARNINGS=ALL say of fcalc,
// each module's messages by identifier, in order, and why each stays
// (Phase 111's subtask 9; the rest were fixed).
var fcalcToolMessages = map[string]map[string][]string{
	// FCPLATFORM is where fcalc keeps what only VMS has: LIB$GETDVI (Phase
	// 111's subtask 11), LIB$GET_FOREIGN, and LBR$OUTPUT_HELP (subtask 12),
	// whose names have a $, each called with a null argument (its channel,
	// its prompt, its width and library: one message a statement).
	"F90": {"FCPLATFORM": {"EXTENSION", "EXTENSION", "EXTENSION", "EXTENSION", "EXTENSION", "EXTENSION"}},

	// HOSTHIDDEN: each internal procedure declares the locals it uses (K, E,
	// H, V, WIDTH, STATUS) so as not to change its host's; the warning is for
	// a reader who might take them for the host's. UNREAD: READ_LINE reads the
	// rest of an overlong line into REST only to drop it.
	"ALL": {
		"FCEVAL":    {"HOSTHIDDEN", "HOSTHIDDEN"},
		"FCNUMERIC": {"HOSTHIDDEN", "HOSTHIDDEN", "HOSTHIDDEN"},
		"FCPLOT":    {"HOSTHIDDEN", "HOSTHIDDEN", "HOSTHIDDEN", "HOSTHIDDEN", "HOSTHIDDEN", "HOSTHIDDEN", "HOSTHIDDEN"},
		"FCSTATE":   {"UNREAD"},
		"FCALC":     {"HOSTHIDDEN"},
	},
}

// fcalcMessage finds a message's identifier, %FORT-W-HOSTHIDDEN's
// HOSTHIDDEN.
var fcalcMessage = regexp.MustCompile(`%FORT-[A-Z]-([A-Z0-9]+),`)

// TestFcalcTools runs the compiler's tools on fcalc, a whole program: each
// module compiled /LIST/CROSS_REFERENCE says nothing and writes its listing,
// and /STANDARD=F90 and /WARNINGS=ALL say only fcalcToolMessages.
func TestFcalcTools(t *testing.T) {
	files, order := fcalcFiles(t)

	c, out := newFortranContextSized(t, files, 20000)

	for _, name := range order {
		err := c.Fortran(FortranOptions{Sources: FortranSources(name), GFloating: true, Optimize: true,
			List: true, CrossReference: true})
		if err != nil || out.Len() != 0 {
			t.Fatalf("FORTRAN/LIST/CROSS_REFERENCE %s: err %v, output:\n%s", name, err, out)
		}

		if listing, _ := volumeFile(t, c, name+".LIS"); len(listing) < 50 {
			t.Errorf("%s.LIS has %d lines", name, len(listing))
		}
	}

	for _, keyword := range []string{"F90", "ALL"} {
		c, out := newFortranContextSized(t, files, 20000)

		for _, name := range order {
			options := FortranOptions{Sources: FortranSources(name), GFloating: true, Optimize: true}
			if keyword == "F90" {
				options.Standard, options.StandardGiven = []string{"F90"}, true
			} else {
				options.Warnings, options.WarningsGiven = []string{"ALL"}, true
			}

			if err := c.Fortran(options); err != nil {
				t.Fatalf("%s %s: %v\n%s", keyword, name, err, out)
			}

			var got []string
			for _, m := range fcalcMessage.FindAllStringSubmatch(out.String(), -1) {
				got = append(got, m[1])
			}

			if want := fcalcToolMessages[keyword][name]; strings.Join(got, " ") != strings.Join(want, " ") {
				t.Errorf("/%s on %s said %v, want %v:\n%s", keyword, name, got, want, out)
			}

			out.Reset()
		}
	}
}
