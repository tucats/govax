package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
)

// Phase 50, subtask 21: the other processes run between DCL commands
// (dcllend.go).

// lendConsole is a console with the scheduler on and its dispatcher, and
// a function that dispatches a line and returns the program's lines.
func lendConsole(t *testing.T) (*console.Console, func(string) []string) {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	return c, func(line string) []string {
		t.Helper()

		out.Reset()

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v\n%s", line, err, out.String())
		}

		return programLines(out.String())
	}
}

// writeProcedure writes lines as dir's file name, and returns its path
// quoted, for @.
func writeProcedure(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return `"` + path + `"`
}

// indexOf is the index of the first line that is text, or -1.
func indexOf(lines []string, text string) int {
	for i, line := range lines {
		if line == text {
			return i
		}
	}

	return -1
}

// TestLend_spawnNoWait: SPAWN/NOWAIT of one of DCL's commands at the
// terminal runs it before the next prompt, and the subprocess has logged
// out by then.
func TestLend_spawnNoWait(t *testing.T) {
	c, dispatch := lendConsole(t)

	got := strings.Join(dispatch(`SPAWN/NOWAIT/PROCESS=BACK WRITE SYS$OUTPUT "in the background"`), "|")
	if got != "%DCL-S-SPAWNED, process BACK spawned|in the background" {
		t.Errorf("SPAWN/NOWAIT: %q", got)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes after SPAWN/NOWAIT, want only process 1", n)
	}
}

// TestLend_interleave: a procedure that spawns another with /NOWAIT
// goes on with its commands, and the subprocess's procedure runs its
// commands between them, each process giving the CPU up after a
// command; both run to their ends.
func TestLend_interleave(t *testing.T) {
	c, dispatch := lendConsole(t)
	dir := t.TempDir()

	foo := writeProcedure(t, dir, "foo.com",
		`$ WRITE SYS$OUTPUT "foo 1"`,
		`$ WRITE SYS$OUTPUT "foo 2"`,
		`$ WRITE SYS$OUTPUT "foo 3"`,
		`$ WRITE SYS$OUTPUT "foo 4"`,
	)

	lines := []string{"$ SPAWN/NOWAIT/PROCESS=FOO @" + foo}
	for i := 1; i <= 8; i++ {
		lines = append(lines, `$ WRITE SYS$OUTPUT "bar `+string(rune('0'+i))+`"`)
	}

	bar := writeProcedure(t, dir, "bar.com", lines...)

	got := dispatch("@" + bar)

	// FOO's commands come between BAR's, not all after them: FOO, made
	// with a boost above process 1's, has two turns first, then the two
	// take turns, a command each, until FOO is done.
	want := []string{
		"%DCL-S-SPAWNED, process FOO spawned",
		"foo 1", "foo 2", "bar 1", "foo 3", "bar 2", "foo 4",
		"bar 3", "bar 4", "bar 5", "bar 6", "bar 7", "bar 8",
	}

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes at the end, want only process 1", n)
	}
}

// TestLend_wait: process 1's WAIT lets a subprocess run its whole
// procedure meanwhile; a subprocess's WAIT doesn't stop process 1, whose
// commands run while it waits.
func TestLend_wait(t *testing.T) {
	c, dispatch := lendConsole(t)
	dir := t.TempDir()

	foo := writeProcedure(t, dir, "foo.com",
		`$ WRITE SYS$OUTPUT "foo 1"`,
		`$ WRITE SYS$OUTPUT "foo 2"`,
		`$ WRITE SYS$OUTPUT "foo 3"`,
	)

	bar := writeProcedure(t, dir, "bar.com",
		"$ SPAWN/NOWAIT/PROCESS=FOO @"+foo,
		"$ WAIT 00:00:00.20",
		`$ WRITE SYS$OUTPUT "bar after the wait"`,
	)

	got := dispatch("@" + bar)
	if i := indexOf(got, "bar after the wait"); i < 0 || indexOf(got, "foo 3") < 0 || indexOf(got, "foo 3") > i {
		t.Errorf("process 1's WAIT: %q", got)
	}

	slow := writeProcedure(t, dir, "slow.com",
		`$ WRITE SYS$OUTPUT "slow 1"`,
		"$ WAIT 00:00:00.30",
		`$ WRITE SYS$OUTPUT "slow 2"`,
	)

	quick := writeProcedure(t, dir, "quick.com",
		"$ SPAWN/NOWAIT/PROCESS=SLOW @"+slow,
		`$ WRITE SYS$OUTPUT "quick 1"`,
		`$ WRITE SYS$OUTPUT "quick 2"`,
		`$ WRITE SYS$OUTPUT "quick 3"`,
	)

	got = dispatch("@" + quick)
	if indexOf(got, "quick 3") < 0 || indexOf(got, "slow 2") >= 0 {
		t.Errorf("a subprocess's WAIT: %q", got)
	}

	if n := len(c.RTL.Processes()); n != 2 {
		t.Errorf("%d processes while SLOW waits, want 2", n)
	}

	got = dispatch("WAIT 00:00:00.50")
	if indexOf(got, "slow 2") < 0 {
		t.Errorf("after SLOW's WAIT: %q", got)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes at the end, want only process 1", n)
	}
}
