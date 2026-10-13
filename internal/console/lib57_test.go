package console_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// The Phase 57 probe (docs/PHASE-57, subtask 10): testdata/lib57's
// lib57.com runs lib57.mar's groups of LIB$ CLI cases, each as a foreign
// command, and shows DCL's state after each. vax/lib57.log is its log on
// VMS, which govax's must match section by section (S1 to S9).

// lib57Dir is the probe's directory, from this package's.
var lib57Dir = filepath.Join("..", "..", "testdata", "lib57")

// lib57Files are the files lib57.com needs on its volume.
var lib57Files = []string{
	"lib57.mar", "lib57b.mar", "def_libcli.mar",
	"lib57.com", "lib57s.com", "lib57c.com", "lib57p.com",
}

// runLib57 runs lib57.com as the VAX run did: on a booted machine with
// the scheduler on, the volume mounted on DUA1 and the default directory
// its [000000], the node called SIMVAX, and @LIB57/OUTPUT=LIB57.LOG. It
// returns the log.
func runLib57(t *testing.T) string {
	t.Helper()

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	d := c.Dispatcher

	disk := filepath.Join(t.TempDir(), "lib57.dsk")
	if err := c.Commands().InitializeContainer(disk, 0, "LIB57", 0, "RD51"); err != nil {
		t.Fatal(err)
	}

	if err := c.Commands().Mount("DUA1", disk, true); err != nil {
		t.Fatal(err)
	}

	c.RTL.NodeName = "SIMVAX"

	for _, name := range lib57Files {
		path, err := filepath.Abs(filepath.Join(lib57Dir, name))
		if err != nil {
			t.Fatal(err)
		}

		if err := d.Dispatch(`COPY "` + path + `"/HOST DUA1:[000000]` + strings.ToUpper(name)); err != nil {
			t.Fatal(err)
		}
	}

	if err := d.Dispatch("SET DEFAULT DUA1:[000000]"); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch("@LIB57/OUTPUT=LIB57.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@LIB57: %v", err)
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]LIB57.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	return string(console.JoinRecords(records))
}

// lib57Compared is a log as the comparison sees it: the job table's
// name, which is the job's own, written as LNM$JOB.
func lib57Compared(text string) string {
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if strings.HasPrefix(line, "(LNM$JOB_") {
			_, rest, _ := strings.Cut(line, ")")
			lines[i] = "(LNM$JOB)" + rest
		}
	}

	return strings.Join(lines, "\n")
}

// TestLib57Oracle compares govax's log of lib57.com with VMS's, section
// by section; before the VMS run, it shows govax's. Either way, the
// programs must assemble and link, and every section run.
func TestLib57Oracle(t *testing.T) {
	got := runLib57(t)

	for _, bad := range []string{"[000000]LIB57.MAR", "[000000]LIB57B.MAR", "activate DUA1:[000000]LIB57"} {
		if strings.Contains(got, bad) {
			t.Errorf("the log has an error for %s:\n%s", bad, got)
		}
	}

	sections := console.ProbeSections(got, "S")
	for i := 1; i <= 9; i++ {
		if name := "S" + strconv.Itoa(i); len(sections[name]) < 3 {
			t.Errorf("section %s didn't run: %q", name, sections[name])
		}
	}

	logText, err := os.ReadFile(filepath.Join(lib57Dir, "vax", "lib57.log"))
	if errors.Is(err, os.ErrNotExist) {
		t.Logf("govax's log:\n%s", got)
		t.Skip("no VMS run yet (vax/lib57.log)")
	}

	if err != nil {
		t.Fatal(err)
	}

	want := console.ProbeSections(lib57Compared(strings.TrimPrefix(strings.ReplaceAll(string(logText), "\r", ""), "\n")), "S")
	gotSections := console.ProbeSections(lib57Compared(got), "S")

	if len(want) < 10 {
		t.Fatalf("the log has %d sections, want 10", len(want))
	}

	for name, wantLines := range want {
		// Before S1: the system's version, MACRO, LINK, and ANALYZE.
		if name == "" {
			continue
		}

		gotLines := gotSections[name]

		for i := range max(len(gotLines), len(wantLines)) {
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
