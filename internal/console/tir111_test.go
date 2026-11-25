package console

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/testproc"
)

// tir111Differences are the probe's modules whose objects govax's MACRO doesn't make as
// VMS's did, with why (testdata/tir111's README).
var tir111Differences = map[string]string{
	"TIRA": "VMS stores one byte of the table's immediate run by STA_UB and STO_B, as in FCFUNCS",
	"TIRE": "the same, with the table's values 1",
	"TIRF": "the same, as in FCMATRIX",
}

// TestTIR111Objects holds govax's MACRO to VMS 7.1's on testdata/tir111's probe modules:
// each object's GSD, TIR, and end records, as TestF90RObjects compares fcalc's, but for
// tir111Differences. TIRX and TIRR, whose errors are on purpose, are
// TestTIR111ErrorObjects's.
func TestTIR111Objects(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	dir := filepath.Join("..", "..", "testdata", "tir111")

	sources, err := filepath.Glob(filepath.Join(dir, "tir*.mar"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no sources: %v", err)
	}

	d, c, buf := dungeonConsole(t, false)

	for _, source := range sources {
		name := strings.ToUpper(strings.TrimSuffix(filepath.Base(source), ".mar"))
		if name == "TIRX" || name == "TIRR" {
			continue // TestTIR111ErrorObjects's
		}

		t.Run(name, func(t *testing.T) {
			got, want := assembledAndVMS(t, d, c, buf, source, filepath.Join(dir, "vax"))

			diff := firstRecordDifference(got, want)
			why, differs := tir111Differences[name]

			switch {
			case differs && diff == "":
				t.Errorf("%s is now VMS's, though tir111Differences says: %s", name, why)
			case !differs && diff != "":
				t.Error(diff)
			}
		})
	}
}

// objectRecordTypes are the object language's record types (OBJ$C_) compared with VMS's:
// the header records hold the time and tool, and the traceback records govax's FORTRAN
// lines.
var objectRecordTypes = map[byte]string{1: "GSD", 2: "TIR", 3: "EOM", 7: "EOMW"}

// comparedRecords are an object's records of objectRecordTypes.
func comparedRecords(records [][]byte) [][]byte {
	var out [][]byte

	for _, r := range records {
		if len(r) > 0 {
			if _, ok := objectRecordTypes[r[0]]; ok {
				out = append(out, r)
			}
		}
	}

	return out
}

// assembledAndVMS assembles source with govax's MACRO (which must say nothing) and
// returns its object's compared records, and those of VMS's object of the same name in
// vmsDir.
func assembledAndVMS(t *testing.T, d *Dispatcher, c *Console, buf *bytes.Buffer, source, vmsDir string) ([][]byte, [][]byte) {
	t.Helper()

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

	vmsObj, err := filepath.Abs(filepath.Join(vmsDir, strings.ToLower(name)+".obj"))
	if err != nil {
		t.Fatal(err)
	}

	vmsRecords, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Host: true, Name: vmsObj}, rms.VariableRecords)
	if err != nil {
		t.Fatal(err)
	}

	return comparedRecords(records), comparedRecords(vmsRecords)
}

// firstRecordDifference describes where got first differs from want, or is "".
func firstRecordDifference(got, want [][]byte) string {
	for i := 0; i < min(len(got), len(want)); i++ {
		if bytes.Equal(got[i], want[i]) {
			continue
		}

		at := 0
		for at < min(len(got[i]), len(want[i])) && got[i][at] == want[i][at] {
			at++
		}

		return fmt.Sprintf("%s record %d differs from VMS's at byte %d of %d (VMS's %d):\n govax % x\n VMS   % x",
			objectRecordTypes[got[i][0]], i, at, len(got[i]), len(want[i]),
			got[i][max(0, at-16):min(len(got[i]), at+24)], want[i][max(0, at-16):min(len(want[i]), at+24)])
	}

	if len(got) != len(want) {
		return fmt.Sprintf("%d records, VMS's %d", len(got), len(want))
	}

	return ""
}

// TestTIR111Errors holds govax's MACRO's errors to VMS's on the probe's modules that have
// them: DIRSYNX on each string directive with nothing after it (TIRX), and on TIRR's
// .LONG R1[2]. (VMS's MACRO writes an object for such a module, as govax's doesn't.)
func TestTIR111Errors(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	dir := filepath.Join("..", "..", "testdata", "tir111")
	d, _, buf := dungeonConsole(t, false)

	for name, want := range map[string][]int{"TIRX": {3, 5, 7}, "TIRR": {27}} {
		abs, err := filepath.Abs(filepath.Join(dir, strings.ToLower(name)+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, `COPY/QUIET "`+abs+`"/HOST `+name+`.MAR`)
		buf.Reset()

		_ = d.Dispatch("MACRO/NOLIST " + name)

		var lines []string
		for _, n := range want {
			lines = append(lines, fmt.Sprintf("line %d: VAX-E-DIRSYNX", n))
		}

		for _, line := range lines {
			if !strings.Contains(buf.String(), line) {
				t.Errorf("MACRO %s said:\n%s\nwant %q", name, buf.String(), line)
			}
		}

		if n := strings.Count(buf.String(), "DIRSYNX"); n != len(want) {
			t.Errorf("MACRO %s gave %d DIRSYNX, VMS's %d:\n%s", name, n, len(want), buf.String())
		}
	}
}

// TestTIR111ErrorObjects holds govax's objects of the modules with errors, TIRX and
// TIRR, to VMS's (Phase 113, subtask 5): VMS's MACRO writes an object for one, its
// end record's severity 2 (an error), a line with an error assembled as it could be:
// a string directive with nothing after it as an empty string, and .LONG R1[2] one
// longword of 0, a register name being no data item (subtask 13c).
func TestTIR111ErrorObjects(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	dir := filepath.Join("..", "..", "testdata", "tir111")
	d, c, buf := dungeonConsole(t, false)

	for _, name := range []string{"TIRX", "TIRR"} {
		abs, err := filepath.Abs(filepath.Join(dir, strings.ToLower(name)+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, `COPY/QUIET "`+abs+`"/HOST `+name+`.MAR`)
		buf.Reset()

		_ = d.Dispatch("MACRO/NOLIST " + name)

		records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: name + ".OBJ"}, rms.VariableRecords)
		if err != nil {
			t.Fatalf("%s: no object: %v", name, err)
		}

		vmsObj, err := filepath.Abs(filepath.Join(dir, "vax", strings.ToLower(name)+".obj"))
		if err != nil {
			t.Fatal(err)
		}

		vmsRecords, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Host: true, Name: vmsObj}, rms.VariableRecords)
		if err != nil {
			t.Fatal(err)
		}

		got, want := comparedRecords(records), comparedRecords(vmsRecords)

		if diff := firstRecordDifference(got, want); diff != "" {
			t.Errorf("%s: %s", name, diff)
		}
	}
}
