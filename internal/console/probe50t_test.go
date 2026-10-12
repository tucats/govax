package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// TestProbe50TOracle runs round 5 of testdata/dcl50, probe50t.com (and
// probe50u.com, which it runs, and probe50e.com, which it types), on a
// booted machine with a volume mounted on DUA1 and the node called
// SIMVAX, as the VMS run's was, with Ctrl/T enabled as SYSTEM's login
// enabled it there, and compares what it writes with VMS 7.3's log line
// for line, section by section: statuses and ON, flow, the lexical
// functions, and a subprocess's DCL.
func TestProbe50TOracle(t *testing.T) {
	logText, err := os.ReadFile("../../testdata/dcl50/vax/probe50t.log")
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	d := c.Dispatcher

	disk := filepath.Join(t.TempDir(), "dcl50.dsk")
	if err := c.Commands().InitializeContainer(disk, 2000, "DCL50", 0, "RD53"); err != nil {
		t.Fatal(err)
	}

	if err := c.Commands().Mount("DUA1", disk, true); err != nil {
		t.Fatal(err)
	}

	c.RTL.NodeName = "SIMVAX"

	dir := t.TempDir()

	for _, name := range []string{"probe50t.com", "probe50u.com", "probe50e.com"} {
		text, err := os.ReadFile(filepath.Join("../../testdata/dcl50", name))
		if err != nil {
			t.Fatal(err)
		}

		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, text, 0o644); err != nil {
			t.Fatal(err)
		}

		if err := d.Dispatch(`COPY "` + path + `"/HOST DUA1:[000000]` + strings.ToUpper(name)); err != nil {
			t.Fatal(err)
		}
	}

	for _, line := range []string{"SET DEFAULT DUA1:[000000]", "SET CONTROL=T"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	if err := d.Dispatch("@PROBE50T/OUTPUT=PROBE50T.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@PROBE50T: %v", err)
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]PROBE50T.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	want := console.ProbeSections(console.ProbeCompared(strings.TrimPrefix(strings.ReplaceAll(string(logText), "\r", ""), "\n")), "T")
	got := console.ProbeSections(console.ProbeCompared(string(console.JoinRecords(records))), "T")

	if len(want) < 5 {
		t.Fatalf("the log has %d sections, want 5", len(want))
	}

	for name, wantLines := range want {
		if name == "" {
			continue
		}

		gotLines := got[name]

		for i := 0; i < max(len(gotLines), len(wantLines)); i++ {
			var g, w string
			if i < len(gotLines) {
				g = gotLines[i]
			}

			if i < len(wantLines) {
				w = wantLines[i]
			}

			if g != w {
				t.Errorf("section %q, line %d:\n got %q\nwant %q", name, i+1, g, w)

				break
			}
		}
	}
}
