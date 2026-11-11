package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 88's VMS round (docs/planning/PHASE-88 - keyed index files.md, subtask 15;
// testdata/vms88's README). TestVMS88Procedure rehearses it on govax: the volume as
// exchange.cmd makes it, and VMS88.COM on it, so a mistake of ours can't cost the author a
// run on the VAX.

// runVMS88 makes the exchange volume and runs the procedure, and returns its log and the
// dispatcher, the volume on DUA1.
func runVMS88(t *testing.T) (string, *Dispatcher) {
	t.Helper()

	d, c, buf := shareFindConsole(t, false)

	path := filepath.Join(t.TempDir(), "vms88.dsk")
	if err := c.cmd().InitializeContainer(path, 100000, "VMS88", 3, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA5:", path, true); err != nil {
		t.Fatal(err)
	}

	t.Chdir(filepath.Join("..", ".."))

	script, err := os.ReadFile(filepath.Join("testdata", "vms88", "exchange.cmd"))
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	for line := range strings.SplitSeq(string(script), "\n") {
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "INITIALIZE/") ||
			strings.HasPrefix(line, "MOUNT/") || strings.HasPrefix(line, "DISMOUNT") {
			continue
		}

		err := d.Dispatch(line)
		if out := buf.String(); err != nil || out != "" {
			t.Fatalf("%s returned %v and said:\n%.600s", line, err, out)
		}
	}

	// On the VAX it's DUA1.
	shareFindRun(t, d, "SET DEFAULT DUA2:[000000]")
	shareFindRun(t, d, "DISMOUNT DUA5")
	shareFindRun(t, d, "DISMOUNT DUA1")

	if err := c.cmd().Mount("DUA1:", path, true); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	shareFindRun(t, d, "@VMS88/OUTPUT=VMS88.LOG")
	buf.Reset()

	return ftn86Log(t, d, "VMS88.LOG"), d
}

// TestVMS88Procedure is the rehearsal: the procedure reaches its end, each file's walks
// agree before and after the changes, and the programs print testdata/rms88's output.
func TestVMS88Procedure(t *testing.T) {
	log, d := runVMS88(t)
	sections := ftn86Sections(log)

	for _, name := range []string{"TOOLS", "G88", "F88", "R88", "OPEN", "FTNRTL", "PHONES", "PHONESL", "PHONESF", "PHONESD", "LOAD", "CHECKS", "ODD", "SAVE", "END"} {
		if _, ok := sections[name]; !ok {
			t.Errorf("no section %s in the log", name)
		}
	}

	// Each file's two walks agree on what doesn't depend on the order of records, and
	// the changes all went in.
	files := 0

	for _, dir := range []string{"G88", "F88", "R88"} {
		for _, file := range strings.Split("\n"+sections[dir], "\n== ")[1:] {
			files++

			name, _, _ := strings.Cut(file, "\n")

			// A case's file that says so: VMS's ANALYZE gives its own E85 the same
			// one error (testdata/idx88/vax/anl/e85.chk).
			if name == "G_E85" {
				continue
			}

			if strings.Contains(file, "first failure") || strings.Contains(file, "reading") || strings.Contains(file, "OPEN") {
				t.Errorf("%s:\n%s", name, file)
			}

			var sums []string

			for line := range strings.SplitSeq(file, "\n") {
				if f := strings.Fields(line); len(f) == 8 && f[0] == "key" {
					sums = append(sums, f[1]+" "+f[3]+" "+f[5])
				}

				if strings.Contains(line, "$STATUS") && !strings.Contains(line, `"%X103B8103"`) && !strings.Contains(line, `"%X00000001"`) {
					t.Errorf("%s: %s", name, line)
				}
			}

			if n := len(sums) / 2; len(sums)%2 != 0 || strings.Join(sums[:n], "|") != strings.Join(sums[n:], "|") {
				t.Errorf("%s: its walks differ:\n%s", name, file)
			}
		}
	}

	t.Logf("%d files", files)

	// The programs print what they print on any RMS, the same as testdata/rms88's.
	for section, file := range map[string]string{"PHONES": "phones.out", "PHONESL": "phonesl.out", "PHONESF": "phonesf.out", "PHONESD": "phonesd.out", "LOAD": "load.out"} {
		want, err := os.ReadFile(filepath.Join("testdata", "rms88", file))
		if err != nil {
			t.Fatal(err)
		}

		if sections[section] != string(want) {
			t.Errorf("%s printed:\n%s\nwant:\n%s", section, sections[section], want)
		}
	}

	// A file that's meant to be damaged or empty shows in FILES's status; see the log.
	t.Logf("OPEN:\n%s\nODD:\n%s\nTOOLS:\n%s\nFTNRTL:\n%s\nCHECKS:\n%s\nSAVE:\n%s", sections["OPEN"], sections["ODD"], sections["TOOLS"], sections["FTNRTL"], sections["CHECKS"], sections["SAVE"])

	vms88Oracle(t, d, sections)
}
