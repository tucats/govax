package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// Phase 51, subtask 3: the terminal's Ctrl/Y goes to the process it is
// attached to, the subprocess a SPAWN without /NOWAIT waits for (and so
// on down a chain of SPAWNs), whose DCL takes it by the rules process 1's
// DCL follows in a procedure (dclcontroly.go).

// ssControlY is SS$_CONTROLY, the status of an image a Ctrl/Y ended at a
// subprocess's command level 0.
const ssControlY = 0x611

// subControlY runs SPAWN line in a console with the scheduler on, with
// typed as the terminal's input and a Ctrl/Y typed after delay (from
// another goroutine, as the terminal's keys arrive). The procedures in
// files are written to a directory first, and "DIR/" in line and in
// them names it. It returns the output's lines, joined with "|", the
// console, and how long the SPAWN took.
func subControlY(t *testing.T, typed, line string, files map[string][]string) (string, *console.Console, time.Duration) {
	t.Helper()

	return subControlYWith(t, typed, line, files, nil)
}

// subControlYWith is subControlY, calling started (if not nil) with the
// console just before the SPAWN.
func subControlYWith(t *testing.T, typed, line string, files map[string][]string, started func(*console.Console)) (string, *console.Console, time.Duration) {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	forever := buildImage(t, c, "forever", strings.Replace(spinSource, "COUNT", "0", 1))
	finite := buildImage(t, c, "finite", strings.Replace(spinSource, "COUNT", "40000000", 1))

	dir := t.TempDir()
	expand := func(s string) string {
		return strings.NewReplacer("DIR/", dir+"/", "FOREVER", forever, "FINITE", finite).Replace(s)
	}

	for name, lines := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(expand(strings.Join(lines, "\n"))+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	c.In = strings.NewReader(typed)
	out.Reset()

	engine := c.Engine

	go func() {
		time.Sleep(100 * time.Millisecond)
		engine.AttentionKey(cpu.AttentionCtrlY)
	}()

	if started != nil {
		started(c)
	}

	start := time.Now()

	if err := d.Dispatch(expand(line)); err != nil && !vmserrors.MessageInhibited(err) {
		out.WriteString("%" + err.Error() + "\n")
	}

	elapsed := time.Since(start)

	if n := len(c.RTL.Processes()); n != 1 || c.CommandLevel() != 0 {
		t.Errorf("%s: %d processes, level %d after SPAWN; want only process 1, level 0", line, n, c.CommandLevel())
	}

	if k := c.Engine.PendingAttention(); k != 0 || c.Engine.StoppedBy() != 0 {
		t.Errorf("%s: attention key %02X pending, %02X stopped the run; want neither", line, k, c.Engine.StoppedBy())
	}

	return strings.Join(programLines(out.String()), "|"), c, elapsed
}

// What SPAWN of SYSTEM_1 says, and what it says when it has waited.
const (
	spawnedOne  = "%DCL-S-SPAWNED, process SYSTEM_1 spawned|%DCL-S-ATTACHED, terminal now attached to process SYSTEM_1|"
	returnedOne = "%DCL-S-RETURNED, control returned to process SYSTEM"
)

// TestSubprocessControlY_procedure: in a spawned subprocess's procedure,
// Ctrl/Y interrupts the image it runs, and the subprocess's DCL reads the
// Ctrl/Y level from the terminal, with the subprocess's prompt: STOP ends
// the procedure, and CONTINUE lets the image finish and the procedure go
// on (13.11.1), a command typed there having run in the procedure's
// context (its symbols), and the image going on from where it was
// paused (it runs
// long enough that the key comes first, so the prompt comes before its
// output); with ON CONTROL_Y, the image is ended and the command runs
// (13.12.1). Process 1, waiting in SPAWN, doesn't see the key, and
// govax doesn't end.
func TestSubprocessControlY_procedure(t *testing.T) {
	for _, tc := range []struct {
		name, typed string
		lines       []string
		want        string
	}{
		{"STOP", "STOP\n", []string{`$ RUN "FOREVER"`, `$ WRITE SYS$OUTPUT "after"`}, "$ " + returnedOne},
		{"ON CONTROL_Y", "", []string{
			"$ ON CONTROL_Y THEN GOTO DONE",
			`$ RUN "FOREVER"`,
			`$ WRITE SYS$OUTPUT "after"`,
			`$ DONE: WRITE SYS$OUTPUT "done"`,
		}, "done|" + returnedOne},
		{"CONTINUE", "WRITE SYS$OUTPUT \"X is ''X'\"\nCONTINUE\n", []string{
			`$ X = "the subprocess's"`,
			`$ RUN "FINITE"`,
			`$ WRITE SYS$OUTPUT "after"`,
		}, "$ X is the subprocess's|$ spun|after|" + returnedOne},
	} {
		got, _, _ := subControlY(t, tc.typed, `SPAWN @"DIR/spin.com"`, map[string][]string{"spin.com": tc.lines})

		if got != spawnedOne+tc.want {
			t.Errorf("%s:\n%q\nwant\n%q", tc.name, got, spawnedOne+tc.want)
		}
	}
}

// TestSubprocessControlY_wait: a subprocess's WAIT ends at a Ctrl/Y, as
// process 1's does, and its procedure reads the Ctrl/Y level; with STOP
// typed there, it ends.
func TestSubprocessControlY_wait(t *testing.T) {
	got, _, elapsed := subControlY(t, "STOP\n", `SPAWN @"DIR/wait.com"`, map[string][]string{
		"wait.com": {"$ WAIT 00:01:00", `$ WRITE SYS$OUTPUT "after"`},
	})

	if want := spawnedOne + "$ " + returnedOne; got != want || elapsed > 30*time.Second {
		t.Errorf("output %q after %v; want %q, at once", got, elapsed, want)
	}
}

// TestSubprocessControlY_levelZero: outside a procedure, a subprocess's
// Ctrl/Y ends the image it runs, with SS$_CONTROLY, and its DCL goes on:
// SPAWN of one command logs out, and SPAWN's status is SS$_CONTROLY.
func TestSubprocessControlY_levelZero(t *testing.T) {
	got, c, _ := subControlY(t, "", `SPAWN RUN "FOREVER"`, nil)

	if got != spawnedOne+returnedOne || c.Status() != ssControlY {
		t.Errorf("output %q, $STATUS %08X; want %q, %08X", got, c.Status(), spawnedOne+returnedOne, ssControlY)
	}
}

// TestSubprocessControlY_nested: down a chain of SPAWNs, the key goes to
// the innermost subprocess, the one the terminal is attached to: STOP
// at its Ctrl/Y level ends its procedure, and its parent's procedure,
// which waited in SPAWN, goes on.
func TestSubprocessControlY_nested(t *testing.T) {
	got, _, _ := subControlY(t, "STOP\n", `SPAWN @"DIR/outer.com"`, map[string][]string{
		"outer.com": {`$ SPAWN/PROCESS=INNER @"DIR/inner.com"`, `$ WRITE SYS$OUTPUT "outer after"`},
		"inner.com": {`$ RUN "FOREVER"`, `$ WRITE SYS$OUTPUT "inner after"`},
	})

	want := spawnedOne +
		"%DCL-S-SPAWNED, process INNER spawned|%DCL-S-ATTACHED, terminal now attached to process INNER|" +
		"$ %DCL-S-RETURNED, control returned to process SYSTEM_1|outer after|" + returnedOne
	if got != want {
		t.Errorf("output\n%q\nwant\n%q", got, want)
	}
}

// TestSubprocessControlY_noControl (Phase 57, subtask 7): SET CONTROL's
// settings are each process's. A spawned procedure's SET NOCONTROL=Y
// turns Ctrl/Y off at the terminal while the subprocess has it
// (ControlYEnabled, as cmd/govax's key goroutine asks), so the key is
// ignored and its image finishes; once SPAWN has returned, the terminal
// is process 1's again, whose Ctrl/Y is on.
func TestSubprocessControlY_noControl(t *testing.T) {
	off := make(chan bool, 1)
	ready := make(chan *console.Console, 1)

	go func() {
		c := <-ready

		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if !c.ControlYEnabled() {
				off <- true

				return
			}

			time.Sleep(time.Millisecond)
		}

		off <- false
	}()

	got, c, _ := subControlYWith(t, "", `SPAWN @"DIR/spin.com"`, map[string][]string{
		"spin.com": {"$ SET NOCONTROL=Y", `$ RUN "FINITE"`, `$ WRITE SYS$OUTPUT "after"`},
	}, func(c *console.Console) { ready <- c })

	if want := spawnedOne + "spun|after|" + returnedOne; got != want {
		t.Errorf("output %q; want %q", got, want)
	}

	if !<-off {
		t.Error("Ctrl/Y stayed enabled while the subprocess that disabled it had the terminal")
	}

	if !c.ControlYEnabled() {
		t.Error("Ctrl/Y is disabled after SPAWN returned to process 1")
	}
}
