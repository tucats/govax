package console

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
)

// ftn86FileDifferences are the programs that don't print their name.out when FTN86.COM runs
// them, with why. A name.out is a terminal's output, and the procedure's runs write a file
// and read a file; each of these also runs with its N variant, compiled /NOOPTIMIZE.
var ftn86FileDifferences = map[string]string{
	"ASTS":       "writes a line by $QIO to SYS$OUTPUT, here a file and not a terminal",
	"DIALOG":     "a $ prompt's line isn't ended by a terminal read, so the next record starts its own",
	"FORMATS":    "a $ prompt's line isn't ended by a terminal read, so the next record starts its own",
	"INQUIRE":    "the directory is the program's own, not [000000]",
	"IOSTMT":     "writes unit 5, which only a terminal allows: RMS refuses it in a procedure",
	"SERVICES":   "no prompt for a read of a file",
	"WHET":       "a kernel with no name.out",
}

// runFTN86 builds the volume as exchange.cmd does and runs FTN86.COM on it, returning the
// log and the console, whose default directory is the volume's. It's the procedure VMS
// runs, here as a rehearsal, FTNRTL built from its sources by it and every program linked
// against that.
func runFTN86(t *testing.T) (string, *Console, *Dispatcher) {
	t.Helper()

	c, buf := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "ftn86.dsk")
	if err := c.cmd().InitializeContainer(path, 60000, "FTN86", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA2:", path, true); err != nil {
		t.Fatal(err)
	}

	t.Chdir(filepath.Join("..", ".."))

	script, err := os.ReadFile(filepath.Join("testdata", "ftn86", "exchange.cmd"))
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	for line := range strings.SplitSeq(string(script), "\n") {
		for _, skip := range []string{"!", "INITIALIZE/", "MOUNT/", "DISMOUNT", "QUIT"} {
			if strings.HasPrefix(line, skip) {
				line = ""
			}
		}

		if line == "" {
			continue
		}

		shareFindRun(t, d, line)

		if out := buf.String(); out != "" {
			t.Fatalf("%s said: %.300s", line, out)
		}
	}

	// As from an earlier run in the same session. This doesn't test what that did on
	// VMS: there LINK translates the name for FTNRTL/OPTIONS and looks for the image it
	// is to make, and govax's LINK doesn't translate it (the phase doc's log).
	shareFindRun(t, d, "DEFINE/NOLOG FTNRTL DUA2:[FTN86]FTNRTL.EXE")
	shareFindRun(t, d, "@FTN86/OUTPUT=FTN86.LOG")

	buf.Reset()

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA2:[000000]FTN86.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	var log strings.Builder
	for _, r := range records {
		log.Write(r)
		log.WriteByte('\n')
	}

	return log.String(), c, d
}

// ftn86Sections is a log's sections by the name each starts with.
func ftn86Sections(log string) map[string]string {
	sections := map[string]string{}
	parts := regexp.MustCompile(`(?m)^==== (\S+)\n`).Split(log, -1)
	names := regexp.MustCompile(`(?m)^==== (\S+)\n`).FindAllStringSubmatch(log, -1)

	for i, name := range names {
		sections[name[1]] = parts[i+1]
	}

	return sections
}

// TestFTN86Procedure rehearses the VMS round (docs/PHASE-86 - FORTRAN milestone.md, subtask
// 4) on govax. FTNRTL builds from its sources with no message, and each program prints what
// the corpus says it should, but for ftn86FileDifferences, which it's held to as well: an
// entry no longer needed is reported.
func TestFTN86Procedure(t *testing.T) {
	log, _, _ := runFTN86(t)
	sections := ftn86Sections(log)

	if _, ok := sections["END"]; !ok {
		t.Fatalf("the procedure didn't reach its end:\n%.2000s", log)
	}

	for line := range strings.SplitSeq(strings.TrimRight(sections["FTNRTL"], "\n"), "\n") {
		if line != `  $STATUS == "%X10000001"` {
			t.Errorf("building FTNRTL said: %s", line)
		}
	}

	var sources []string

	for _, pattern := range []string{"testdata/fortran/*.for", "testdata/opt84/kernels/*.for", "testdata/fortran/modules/*"} {
		found, err := filepath.Glob(filepath.FromSlash(pattern))
		if err != nil {
			t.Fatal(err)
		}

		sources = append(sources, found...)
	}

	checked := 0

	for _, source := range sources {
		base := strings.TrimSuffix(filepath.Base(source), ".for")
		upper := strings.ToUpper(base)
		want := filepath.Join(filepath.Dir(source), base+".out")
		directory := upper

		if !strings.HasSuffix(source, ".for") {
			want = filepath.Join(source, base+".out")
			directory = "M_" + upper
		}

		wanted, err := os.ReadFile(want)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}

		why, differs := ftn86FileDifferences[upper]

		for _, variant := range []string{directory, directory + "N"} {
			got, ok := sections[variant]
			if !ok {
				t.Errorf("%s isn't in the log", variant)

				continue
			}

			checked++

			// As in TestFortranPrograms: /OPTIMIZE moves the code, so what a program
			// prints that says where its code is can't be the same.
			switch same := maskCodeAddresses(got) == maskCodeAddresses(string(wanted)); {
			case differs && same:
				t.Errorf("%s printed its name.out, though ftn86FileDifferences says: %s", variant, why)
			case !differs && !same:
				t.Errorf("%s printed:\n%s\nwant:\n%s", variant, got, wanted)
			}
		}
	}

	t.Logf("%d runs", checked)
}
