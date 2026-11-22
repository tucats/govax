package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Fortran 90 program's VMS round (docs/planning/PHASE-111 - Fortran 90 milestone.md,
// subtask 11; testdata/f90r): fcalc and the whole Fortran 90 corpus, compiled by govax,
// assembled and linked by VMS's MACRO and LINK against an FTNRTL built there from its
// sources. TestF90RProcedure rehearses the procedure on govax, so a mistake in it shows
// here before it reaches the VAX.

// f90rFileDifferences are the corpus programs that don't print their name.out when
// F90R.COM runs them, with why: a name.out is a terminal's output, and the procedure's
// runs write a file and read a file. Each also runs as its N and R variants.
var f90rFileDifferences = map[string]string{
	"ADVANCE": "a nonadvancing prompt before a read of a file stays in its record, which the next WRITE goes on with",
}

// f90rFcalcDifferences are fcalc's scripts whose output in the procedure isn't their
// name.out, with why.
var f90rFcalcDifferences = map[string]string{}

// f90rPiecesAfterPrompt is how many characters of a line fcalc's SAY fits in the record
// its prompt left open, "fcalc> ", when it read the line from a file: the record's 132
// less the prompt's 7. At a terminal the line starts a record of its own, of 132.
const f90rPiecesAfterPrompt = 125

// f90rFromFile is a script's name.out as fcalc prints it reading the script from a file:
// a typed line too long for the record its prompt left open is split after 125
// characters, not 132 (fcmessages.f90's SAY), the rest in pieces of 132.
func f90rFromFile(out, in string) string {
	pieces := func(line string, first int) string {
		var parts []string
		for at, room := 0, first; at < len(line); at, room = at+room, 132 {
			parts = append(parts, line[at:min(len(line), at+room)])
		}

		return "fcalc> " + strings.Join(parts, "\n") + "\n"
	}

	for line := range strings.SplitSeq(in, "\n") {
		if len(line) > f90rPiecesAfterPrompt {
			out = strings.ReplaceAll(out, pieces(line, 132), pieces(line, f90rPiecesAfterPrompt))
		}
	}

	return out
}

// runF90R builds the volume as exchange.cmd does and runs F90R.COM on it.
func runF90R(t *testing.T) (string, *Console, *Dispatcher) {
	t.Helper()

	return runVMSRound(t, "f90r", 200000, "F90R")
}

// fcalcVariantsF90R are fcalc's directories in F90R.COM: gen.go's fcalcVariants.
var fcalcVariantsF90R = []string{"FCALC", "FCALCN", "FCALCR", "FCALCC"}

// TestF90RProcedure rehearses the VMS round on govax. FTNRTL builds from its sources
// with no message; each corpus program prints what its name.out says, but for
// f90rFileDifferences, which it's held to as well; and fcalc, in each variant, builds
// with no message, prints each script's name.out but for f90rFcalcDifferences, writes
// the files testdata/fcalc/expected holds, and runs ONESHOT.COM as oneshot.out says.
func TestF90RProcedure(t *testing.T) {
	log, _, _ := runF90R(t)
	sections := ftn86Sections(log)

	if _, ok := sections["END"]; !ok {
		t.Fatalf("the procedure didn't reach its end:\n%.2000s", log)
	}

	for line := range strings.SplitSeq(strings.TrimRight(sections["FTNRTL"], "\n"), "\n") {
		if line != `  $STATUS == "%X10000001"` {
			t.Errorf("building FTNRTL said: %s", line)
		}
	}

	checkF90RFcalc(t, sections)

	var sources []string

	for _, pattern := range []string{"testdata/fortran90/*.f90", "testdata/fortran90/modules/*"} {
		found, err := filepath.Glob(filepath.FromSlash(pattern))
		if err != nil {
			t.Fatal(err)
		}

		sources = append(sources, found...)
	}

	checked := 0
	used := map[string]bool{}

	for _, source := range sources {
		base := strings.TrimSuffix(filepath.Base(source), ".f90")
		upper := strings.ToUpper(base)
		want := filepath.Join(filepath.Dir(source), base+".out")
		directory := upper

		if !strings.HasSuffix(source, ".f90") {
			want = filepath.Join(source, base+".out")
			directory = "M_" + upper
		}

		wanted, err := os.ReadFile(want)
		if err != nil {
			t.Fatal(err)
		}

		_, differs := f90rFileDifferences[upper]
		_, recursiveDiffers := recursiveDifferences[base]

		for _, variant := range []string{directory, directory + "N", directory + "R"} {
			got, ok := sections[variant]
			if !ok {
				t.Errorf("%s isn't in the log", variant)

				continue
			}

			checked++

			if recursiveDiffers && variant == directory+"R" {
				continue
			}

			switch same := maskCodeAddresses(got) == maskCodeAddresses(string(wanted)); {
			case differs && same:
				used[upper] = used[upper] || false
			case differs:
				used[upper] = true
			case !same:
				t.Errorf("%s printed:\n%s\nwant:\n%s", variant, got, wanted)
			}
		}
	}

	for name, why := range f90rFileDifferences {
		if !used[name] {
			t.Errorf("%s printed its name.out, though f90rFileDifferences says: %s", name, why)
		}
	}

	t.Logf("%d runs of the corpus", checked)
}

// checkF90RFcalc holds fcalc's sections of the procedure's log to testdata/fcalc.
func checkF90RFcalc(t *testing.T, sections map[string]string) {
	t.Helper()

	// runVMSRound has made the repository root the working directory.
	fcalcDirectory := filepath.Join("testdata", "fcalc")

	scripts, err := filepath.Glob(filepath.Join(fcalcDirectory, "*.in"))
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no scripts in %s (%v)", fcalcDirectory, err)
	}

	expected, err := filepath.Glob(filepath.Join(fcalcDirectory, "expected", "*"))
	if err != nil || len(expected) == 0 {
		t.Fatalf("no files in %s (%v)", filepath.Join(fcalcDirectory, "expected"), err)
	}

	oneshot, err := os.ReadFile(filepath.Join(fcalcDirectory, "oneshot.out"))
	if err != nil {
		t.Fatal(err)
	}

	used := map[string]bool{}

	for _, variant := range fcalcVariantsF90R {
		if got := sections[variant]; got != `  $STATUS == "%X10000001"`+"\n" {
			t.Errorf("building %s said:\n%s", variant, got)
		}

		for _, script := range scripts {
			name := strings.ToUpper(strings.TrimSuffix(filepath.Base(script), ".in"))

			want, err := os.ReadFile(strings.TrimSuffix(script, ".in") + ".out")
			if err != nil {
				t.Fatal(err)
			}

			typed, err := os.ReadFile(script)
			if err != nil {
				t.Fatal(err)
			}

			got := fcalcTime.ReplaceAllString(sections[variant+"."+name], fcalcTimeMask)
			same := got == f90rFromFile(string(want), string(typed))+`  $STATUS == "%X00000001"`+"\n"

			switch _, differs := f90rFcalcDifferences[name]; {
			case differs && same:
				used[name] = used[name] || false
			case differs:
				used[name] = true
			case !same:
				t.Errorf("%s on %s.IN printed:\n%s\nwant:\n%s", variant, name, got, want)
			}
		}

		for _, file := range expected {
			want, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}

			name := strings.ToUpper(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))

			got := fcalcTime.ReplaceAllString(sections[variant+"."+name], fcalcTimeMask)
			if got != string(want) {
				t.Errorf("%s's %s holds:\n%s\nwant:\n%s", variant, filepath.Base(file), got, want)
			}
		}

		if got := fcalcTime.ReplaceAllString(sections[variant+".ONESHOT"], fcalcTimeMask); got != string(oneshot) {
			t.Errorf("%s's @ONESHOT printed:\n%s\nwant:\n%s", variant, got, oneshot)
		}
	}

	for name, why := range f90rFcalcDifferences {
		if !used[name] {
			t.Errorf("%s printed its name.out, though f90rFcalcDifferences says: %s", name, why)
		}
	}
}
