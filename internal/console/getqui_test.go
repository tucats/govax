package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/testproc"
)

// $GETQUI (docs/planning/PHASE-98 - batch queues.md, subtask 9), held to
// round 2 of the probe: BAT98Q, the probe's own program, assembled, linked,
// and run on govax, its output compared with VMS 7.1's
// (testdata/bat98/vax/bat98b.log).

// bat98bLog is round 2's log.
var bat98bLog = filepath.Join("..", "..", "testdata", "bat98", "vax", "bat98b.log")

// bat98qTimes are the items whose values are times of the run, masked.
var bat98qTimes = []string{"QUI$_AFTER_TIME", "QUI$_SUBMISSION_TIME"}

// bat98qOutput is BAT98Q's cases in a log: the lines after "$ RUN
// BAT98Q" up to its JBC MESSAGES section, each time item's bytes masked.
// The section, $GETMSG's dump of the JBC facility's message numbers, is
// left out: govax's system messages are VMS 7.3's file's, which has
// messages VMS 7.1's hasn't and words others differently, so the dump
// differs (vmsdef's TestJBCSharedMessages checks a few of its lines).
func bat98qOutput(lines []string) []string {
	var out []string

	running, mask := false, false

	for _, line := range lines {
		switch {
		case line == "$ RUN BAT98Q":
			running = true

			continue
		case !running:
			continue
		case line == "$ SHOW SYMBOL $STATUS" || line == "######## JBC MESSAGES":
			return out
		case mask:
			line, mask = "    (time)", false
		}

		for _, item := range bat98qTimes {
			if strings.HasPrefix(line, "  "+item+" len=8") {
				mask = true
			}
		}

		out = append(out, line)
	}

	return out
}

// TestGetquiOracle runs BAT98Q as round 2 did: on a volume with
// BAT98OK.COM, the queue manager started, a queue SYS$BATCH (case 41
// translates its name), and entry numbers from 419, as the VAX's were. Its
// 59 cases, $SNDJBC's and $GETQUI's, are held to VMS's line for line, the
// times masked.
func TestGetquiOracle(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	if testing.Short() {
		t.Skip("runs a program on a booted machine")
	}

	c, dispatch := batchConsole(t)

	src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "bat98", "bat98q.mar"))
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{
		`COPY "` + src + `"/HOST DUA1:[000000]BAT98Q.MAR/QUIET`,
		"INITIALIZE/QUEUE/BATCH SYS$BATCH",
		"SET DEFAULT DUA1:[000000]",
	} {
		if out := dispatch(line); strings.Contains(out, "%") {
			t.Fatalf("%s: %s", line, out)
		}
	}

	err = jobqueue.Update(c.QueueFile, func(d *jobqueue.Database) error {
		d.NextEntry = 419

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{"MACRO BAT98Q", "LINK BAT98Q"} {
		if out := dispatch(line); strings.Contains(out, "%") {
			t.Fatalf("%s: %s", line, out)
		}
	}

	got := bat98qOutput(append(append([]string{"$ RUN BAT98Q"},
		strings.Split(strings.TrimRight(dispatch("RUN BAT98Q"), "\n"), "\n")...), "$ SHOW SYMBOL $STATUS"))

	data, err := os.ReadFile(bat98bLog)
	if err != nil {
		t.Fatal(err)
	}

	want := bat98qOutput(strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n"))
	if len(want) < 300 || !strings.HasPrefix(want[len(want)-3], "== 59 ") {
		t.Fatalf("BAT98Q's cases not found in %s (%d lines)", bat98bLog, len(want))
	}

	failures := 0

	for i := 0; i < max(len(got), len(want)); i++ {
		g, w := "(none)", "(none)"
		if i < len(got) {
			g = got[i]
		}

		if i < len(want) {
			w = want[i]
		}

		if g == w {
			continue
		}

		t.Errorf("line %d:\n got %q\nwant %q", i+1, g, w)

		// Report the first few only: one difference usually moves every
		// line after it.
		if failures++; failures > 10 {
			t.FailNow()
		}
	}
}
