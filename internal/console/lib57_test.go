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

// lib57Compared is a log as the comparison sees it: SHOW LOGICAL/FULL's
// header for the job and group tables, the table's name and the line of
// its attributes after it, written as "(LNM$JOB)" or "(LNM$GROUP)" and
// "[attributes]". The job table's name is the job's own, and govax shows
// none of a shareable table's attributes, quota, protection, and owner
// ("[no protection information]"), where VMS V7.1 did (its quota in use
// varying from run to run).
func lib57Compared(text string) string {
	lines := strings.Split(text, "\n")

	for i := 0; i < len(lines); i++ {
		for _, table := range []string{"(LNM$JOB_", "(LNM$GROUP_"} {
			if strings.HasPrefix(lines[i], table) && i+1 < len(lines) {
				lines[i] = strings.TrimSuffix(table, "_") + ")"
				lines[i+1] = "[attributes]"
			}
		}
	}

	return strings.Join(lines, "\n")
}

// lib57RunHistory is VMS's log as it would be in a fresh process: the
// author ran the probe twice on the VAX, in one login, and the second
// run's log is the one kept, so two things in it are the first run's
// doing. The common area held the first run's last text when COM1 first
// read it ("after an exit handler", from RUNEXH); govax's process starts
// with it empty, as VMS's did. And LIB57N.OUT, the no-CLI child's
// SYS$OUTPUT, has the child's lines twice: the second run's were
// appended to the first's file (unconfirmed: LIB$PUT_OUTPUT seems to
// open an existing SYS$OUTPUT file to append), where govax's run makes
// it afresh. Only the first copy is compared.
func lib57RunHistory(t *testing.T, text string) string {
	t.Helper()

	const firstGet = "GET first R0=00000001\n  D: 21 \"after an exit handler\"\n  L: 00000015\n"
	if !strings.Contains(text, firstGet) {
		t.Fatal("VMS's log has no first-run common area: lib57RunHistory is out of date")
	}

	text = strings.Replace(text, firstGet, "GET first R0=00000001\n  D: 0 \"\"\n  L: 00000000\n", 1)

	start := strings.Index(text, "$ TYPE LIB57N.OUT\n")
	end := strings.Index(text, "$ IF F$SEARCH(\"LIB57N.ERR\")")

	if start < 0 || end < start {
		t.Fatal("VMS's log has no LIB57N.OUT: lib57RunHistory is out of date")
	}

	child := text[start:end]
	first := strings.Index(child, "GET_FOREIGN R0=")
	second := strings.LastIndex(child, "GET_FOREIGN R0=")

	if first < 0 || second <= first || child[first:second] != child[second:] {
		t.Fatal("VMS's LIB57N.OUT isn't two copies: lib57RunHistory is out of date")
	}

	return text[:start] + child[:second] + text[end:]
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

	vax := lib57RunHistory(t, strings.TrimPrefix(strings.ReplaceAll(string(logText), "\r", ""), "\n"))
	want := console.ProbeSections(lib57Compared(vax), "S")
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
