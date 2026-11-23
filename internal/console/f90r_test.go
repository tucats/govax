package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
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

	// gen.go's headers and directories.
	return runVMSRound(t, "f90r", 200000, "F90R", rms.ContainerSpace{Headers: 6000, Directories: 800})
}

// fcalcVariantsF90R are fcalc's directories in F90R.COM: gen.go's fcalcVariants.
var fcalcVariantsF90R = []string{"FCALC", "FCALCN", "FCALCR", "FCALCC"}

// TestF90RProcedure rehearses the VMS round on govax. FTNRTL builds from its sources
// with no message; each corpus program prints what its name.out says, but for
// f90rFileDifferences, which it's held to as well; and fcalc, in each variant, builds
// with no message, prints each script's name.out but for f90rFcalcDifferences, writes
// the files testdata/fcalc/expected holds, and runs ONESHOT.COM as oneshot.out says.
func TestF90RProcedure(t *testing.T) {
	log := sharedRoundLog(t, "f90r", runF90R)
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

// f90rVMSDifferences are the sections of VMS 7.1's log (testdata/f90r/vax/f90r.log) that
// aren't what the same procedure prints on govax, with why. A name without a variant's
// letter stands for the program's three runs.
var f90rVMSDifferences = map[string]string{}

// f90rAwaitingVMS are the sections VMS's last run didn't make as govax's run does, which
// the next round is to, by prefix. (Round 1's, testdata/f90r/vax/round1, had the five
// programs and fcalc whose bare .ASCII VMS's MACRO refused; round 2 ran them all.)
// ZEROSIZE, NULLS, and DEFAULTS are Phase 112's (subtasks 3 to 5), added to the corpus after
// round 2; Phase 113's VMS round is to run them.
var f90rAwaitingVMS = []string{"ZEROSIZE", "NULLS", "DEFAULTS", "PURES", "ELEMENTALS", "FORALLS", "FORALLC"}

// TestF90ROracle holds the procedure's run on govax to VMS 7.1's: Fortran 90 programs
// govax's compiler made, assembled and linked by VMS's MACRO and LINK against an FTNRTL
// built there from its sources, on a real VAX's instruction set and VMS's own LIBRTL,
// MTHRTL, and RMS.
func TestF90ROracle(t *testing.T) {
	vms, err := os.ReadFile(filepath.Join("..", "..", "testdata", "f90r", "vax", "f90r.log"))
	if err != nil {
		t.Fatal(err)
	}

	// fcalc's saved files are headed by the time they were written.
	log := fcalcTime.ReplaceAllString(sharedRoundLog(t, "f90r", runF90R), fcalcTimeMask)
	got := ftn86Sections(log)
	want := ftn86Sections(ftn86Lines(fcalcTime.ReplaceAllString(string(vms), fcalcTimeMask)))

	awaiting := func(name string) bool {
		for _, prefix := range f90rAwaitingVMS {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}

		return false
	}

	if a, b := ftn86Statuses(got["FTNRTL"], 0), ftn86Statuses(want["FTNRTL"], 0); a != b {
		t.Errorf("building FTNRTL ended with:\n%s\nVMS's:\n%s", a, b)
	}

	seen := map[string]bool{}

	for name, text := range got {
		if name == "FTNRTL" || awaiting(name) {
			continue
		}

		vmsText, ok := want[name]
		if !ok {
			t.Errorf("%s isn't in VMS's log", name)

			continue
		}

		base := name
		if _, ok := f90rVMSDifferences[name]; !ok {
			base = strings.TrimSuffix(strings.TrimSuffix(name, "N"), "R")
		}

		_, expected := f90rVMSDifferences[base]
		same := ftn86Lines(withoutTraceLines(text)) == vmsText

		switch {
		case expected:
			seen[base] = seen[base] || !same
		case !same:
			t.Errorf("%s printed:\n%s\nVMS's:\n%s", name, ftn86Lines(withoutTraceLines(text)), vmsText)
		}
	}

	for name := range want {
		if _, ok := got[name]; !ok && !awaiting(name) {
			t.Errorf("VMS's log has %s, govax's doesn't", name)
		}
	}

	for name := range f90rVMSDifferences {
		if !seen[name] {
			t.Errorf("%s now matches VMS: take it out of f90rVMSDifferences", name)
		}
	}
}

// f90rObjectDifferences are fcalc's objects govax's MACRO doesn't make as VMS's did, with
// why: testdata/tir111 is the probe to settle it.
var f90rObjectDifferences = map[string]string{
	"FCFUNCS":  "VMS stores one byte of a long run of immediate data by STA_UB and STO_B, at $PDATA+^X216 and ^X666",
	"FCMATRIX": "the same, at $PDATA+^X551",
}

// TestF90RObjects holds govax's MACRO to VMS 7.1's on the FORTRAN compiler's own output:
// round 2 copied back fcalc's .MAR files, which govax's compiler wrote, and the objects
// VMS's MACRO made of them (testdata/f90r/vax/fcalc). govax's MACRO must make each
// object's GSD, TIR, and end records as VMS's did. The header records (time, tool,
// command) differ by nature, and the traceback and debugger records by design: govax's
// MACRO gives a compiled FORTRAN program's traceback records its FORTRAN lines
// (DEVIATIONS.md's "A FORTRAN program's traceback lines through MACRO").
//
// f90rObjectDifferences are the objects that still differ, with why.
func TestF90RObjects(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "f90r", "vax", "fcalc")

	sources, err := filepath.Glob(filepath.Join(dir, "*.mar"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no sources: %v", err)
	}

	// The object language's record types (obj's OBJ$C_ codes) compared.
	compared := map[byte]string{1: "GSD", 2: "TIR", 3: "EOM", 7: "EOMW"}

	kept := func(records [][]byte) [][]byte {
		var out [][]byte

		for _, r := range records {
			if len(r) > 0 {
				if _, ok := compared[r[0]]; ok {
					out = append(out, r)
				}
			}
		}

		return out
	}

	d, c, buf := dungeonConsole(t, false)

	for _, source := range sources {
		abs, err := filepath.Abs(source)
		if err != nil {
			t.Fatal(err)
		}

		name := strings.ToUpper(strings.TrimSuffix(filepath.Base(source), ".mar"))
		shareFindRun(t, d, `COPY/QUIET "`+abs+`"/HOST `+name+`.MAR`)
		shareFindRun(t, d, "MACRO/NOLIST "+name)

		if out := buf.String(); out != "" {
			t.Fatalf("MACRO %s said: %s", name, out)
		}

		records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: name + ".OBJ"}, rms.VariableRecords)
		if err != nil {
			t.Fatal(err)
		}

		vmsObj, err := filepath.Abs(filepath.Join(dir, strings.ToLower(name)+".obj"))
		if err != nil {
			t.Fatal(err)
		}

		vmsRecords, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Host: true, Name: vmsObj}, rms.VariableRecords)
		if err != nil {
			t.Fatal(err)
		}

		got, want := kept(records), kept(vmsRecords)
		_, differs := f90rObjectDifferences[name]
		same := len(got) == len(want)

		for i := 0; same && i < len(got); i++ {
			same = bytes.Equal(got[i], want[i])
		}

		switch {
		case differs && same:
			t.Errorf("%s is now VMS's: take it out of f90rObjectDifferences", name)

			continue
		case differs:
			continue
		}

		if len(got) != len(want) {
			t.Errorf("%s has %d GSD, TIR, and end records, VMS's %d", name, len(got), len(want))

			continue
		}

		for i := range got {
			if !bytes.Equal(got[i], want[i]) {
				at := 0
				for at < min(len(got[i]), len(want[i])) && got[i][at] == want[i][at] {
					at++
				}

				t.Errorf("%s's %s record %d differs from VMS's at byte %d of %d (VMS's %d):\n govax % x\n VMS   % x",
					name, compared[got[i][0]], i, at, len(got[i]), len(want[i]),
					got[i][max(0, at-16):min(len(got[i]), at+24)], want[i][max(0, at-16):min(len(want[i]), at+24)])

				break
			}
		}
	}
}
