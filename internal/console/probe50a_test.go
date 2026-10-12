package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// probe50ASkipped are the commands of round 6 whose output is outside
// the probe's question, and left out of the comparison:
// SHOW LOGICAL/PROCESS lists the process's own names, which are the
// VAX's (its terminal, SYS$DISK in two modes).
var probe50ASkipped = []string{"$ SHOW LOGICAL/PROCESS"}

// probe50AWait is round 6's WAIT, and probe50AShortWait the one the test
// runs instead, so the run takes hundredths of a second, not two seconds;
// the test's log is given back the original's text before it's compared.
const (
	probe50AWait      = "WAIT 00:00:01"
	probe50AShortWait = "WAIT 00:00:00.01"
)

// probe50ACompared is a round 6 log as the comparison sees it: the output
// of the probe50ASkipped commands left out; the job table's name, which
// is the job's own, written as LNM$JOB; a NUL byte VMS 7.3 wrote at the
// end of WAIT NOTATIME's segment, its buffer's garbage, taken out; and
// one comment written as VMS 7.3 showed it. VMS showed R3's comment "@'s /OUTPUT: ..." without
// the blank after the apostrophe's "s" ("@'s/OUTPUT"), where it kept such
// a blank before a letter or a quote in every other comment of the five
// rounds (unexplained; testdata/dcl50's README, "What VMS answered in
// round 6").
func probe50ACompared(text string) string {
	var (
		out      []string
		skipping bool
	)

	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "$ ") {
			skipping = false

			for _, prefix := range probe50ASkipped {
				if strings.HasPrefix(line, prefix) {
					skipping = true
				}
			}
		} else if skipping {
			continue
		}

		if strings.HasPrefix(line, "(LNM$JOB_") {
			line = "(LNM$JOB)"
		}

		line = maskVolumeDetails(strings.ReplaceAll(line, "\x00", ""))

		// VMS's RMS ends a block's variable-length records with a length
		// of FFFF; ods2's writer leaves the bytes after them zero
		// (DEVIATIONS.md, "[Phase 51] DUMP"), so R4's DUMP/BLOCK differs
		// in that one word.
		line = strings.Replace(line, " FFFF0068 67666564 ", " 00000068 67666564 ", 1)
		out = append(out, strings.Replace(line, "@'s /OUTPUT", "@'s/OUTPUT", 1))
	}

	return strings.Join(out, "\n")
}

// TestProbe50AOracle runs round 6 of testdata/dcl50, probe50a.com (and
// probe50b.com and probe50c.com, which it runs), on a volume mounted on
// DUA1, the node called SIMVAX, as the VMS run's was, and compares what
// it writes with VMS 7.3's log line for line, section by section: WAIT,
// flow, @'s /OUTPUT, OPEN/WRITE's file, SHOW LOGICAL, and odds and ends.
func TestProbe50AOracle(t *testing.T) {
	logText, err := os.ReadFile("../../testdata/dcl50/vax/probe50a.log")
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	d, c, _ := newCommandDispatcher(t)
	mountClusteredContainer(t, c, "DUA1", 3)
	c.RTL.NodeName = "SIMVAX"

	dir := t.TempDir()

	for _, name := range []string{"probe50a.com", "probe50b.com", "probe50c.com"} {
		path := filepath.Join(dir, name)
		text := strings.ReplaceAll(readTestdata(t, filepath.Join("../../testdata/dcl50", name)), probe50AWait, probe50AShortWait)
		writeFile(t, path, text)

		if err := d.Dispatch(`COPY "` + path + `"/HOST DUA1:[000000]` + strings.ToUpper(name)); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.cmd().SetDefault("DUA1:[000000]"); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch("@PROBE50A/OUTPUT=PROBE50A.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@PROBE50A: %v", err)
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]PROBE50A.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	gotText := strings.ReplaceAll(string(joinRecords(records)), probe50AShortWait, probe50AWait)

	want := probeSections(probe50ACompared(strings.TrimPrefix(strings.ReplaceAll(string(logText), "\r", ""), "\n")), "R")
	got := probeSections(probe50ACompared(gotText), "R")

	if len(want) < 7 {
		t.Fatalf("the log has %d sections, want 7", len(want))
	}

	for name, wantLines := range want {
		// Before its first section, VMS's log has the probe's opening
		// comments, verification having been on when it ran.
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

// TestProbe50ATerminal replays round 6's lines typed at the terminal
// (testdata/dcl50/vax/probe50a-term.log: RETURN, SUBROUTINE,
// ENDSUBROUTINE, ELSE, and WAIT at command level 0), each command's
// output and messages compared with VMS 7.3's, as cmd/govax shows them.
func TestProbe50ATerminal(t *testing.T) {
	logText, err := os.ReadFile("../../testdata/dcl50/vax/probe50a-term.log")
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	d, _, buf := newCommandDispatcher(t)

	type exchange struct {
		command string
		output  []string
	}

	var session []exchange

	for _, line := range strings.Split(strings.ReplaceAll(string(logText), "\r", ""), "\n") {
		if rest, ok := strings.CutPrefix(line, "$ "); ok {
			if command := strings.TrimSpace(rest); command != "" && !strings.HasPrefix(command, "!") {
				session = append(session, exchange{command: command})
			}

			continue
		}

		if len(session) > 0 && line != "" {
			last := &session[len(session)-1]
			last.output = append(last.output, line)
		}
	}

	if len(session) != 16 {
		t.Fatalf("the log has %d commands, want 16", len(session))
	}

	for _, x := range session {
		command := strings.Replace(x.command, "WAIT 00:00:01", probe50AShortWait, 1)

		buf.Reset()

		if err := d.Dispatch(command); err != nil && !vmserrors.MessageInhibited(err) {
			fmt.Fprintln(buf, "%"+err.Error())
		}

		got := strings.TrimSuffix(buf.String(), "\n")
		want := strings.Join(x.output, "\n")

		if got != want {
			t.Errorf("%s:\n got %q\nwant %q", x.command, got, want)
		}
	}
}
