package console_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
)

// Phase 51, subtask 4: the other processes run while the prompt waits
// for a line (dcllend.go's LendWhileReading).

// promptConsole is a console with the scheduler on, its output, and a
// function that dispatches a line at command level 0, as the prompt
// loop does.
func promptConsole(t *testing.T) (*console.Console, *bytes.Buffer, func(string)) {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	return c, out, func(line string) {
		t.Helper()

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v\n%s", line, err, out.String())
		}
	}
}

// awaitLine runs c's LendWhileReading for a line that takes d to type,
// failing if it hasn't returned soon after the line came. It returns
// how long it took.
func awaitLine(t *testing.T, c *console.Console, d time.Duration) time.Duration {
	t.Helper()

	line := make(chan struct{})
	time.AfterFunc(d, func() { close(line) })

	start := time.Now()
	done := make(chan struct{})

	go func() {
		c.LendWhileReading(line)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(d + 5*time.Second):
		t.Fatal("LendWhileReading didn't return when the line came")
	}

	return time.Since(start)
}

// TestLendWhileReading_wait: a subprocess whose procedure WAITs runs
// on while the prompt waits, finishing its procedure and logging out
// before the line is typed.
func TestLendWhileReading_wait(t *testing.T) {
	c, out, dispatch := promptConsole(t)
	dir := t.TempDir()

	slow := writeProcedure(t, dir, "slow.com",
		`$ WRITE SYS$OUTPUT "slow 1"`,
		"$ WAIT 00:00:00.30",
		`$ WRITE SYS$OUTPUT "slow 2"`,
	)

	out.Reset()
	dispatch("SPAWN/NOWAIT/PROCESS=SLOW @" + slow)

	got := programLines(out.String())
	if indexOf(got, "slow 1") < 0 || indexOf(got, "slow 2") >= 0 {
		t.Fatalf("by the prompt: %q", got)
	}

	out.Reset()

	if took := awaitLine(t, c, 600*time.Millisecond); took < 600*time.Millisecond {
		t.Errorf("LendWhileReading returned after %v, before the line", took)
	}

	if got := programLines(out.String()); indexOf(got, "slow 2") < 0 {
		t.Errorf("while the prompt waited: %q", got)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes after the line, want only process 1", n)
	}
}

// TestLendWhileReading_busy: a subprocess that never waits runs while
// the prompt waits, and the line typed ends the lending at once; the
// subprocess goes on with the next lending.
func TestLendWhileReading_busy(t *testing.T) {
	c, out, dispatch := promptConsole(t)
	dir := t.TempDir()

	busy := writeProcedure(t, dir, "busy.com",
		"$ N = 0",
		"$ LOOP:",
		"$ N = N + 1",
		`$ IF (N / 100) * 100 .EQ. N THEN WRITE SYS$OUTPUT "busy"`,
		"$ GOTO LOOP",
	)

	dispatch("SPAWN/NOWAIT/PROCESS=BUSY @" + busy)
	out.Reset()

	if took := awaitLine(t, c, 200*time.Millisecond); took > 2*time.Second {
		t.Errorf("LendWhileReading took %v to see the line", took)
	}

	if got := programLines(out.String()); indexOf(got, "busy") < 0 {
		t.Errorf("the subprocess didn't run while the prompt waited: %q", got)
	}

	if n := len(c.RTL.Processes()); n != 2 {
		t.Errorf("%d processes, want process 1 and BUSY", n)
	}

	out.Reset()
	dispatch("STOP BUSY")

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes after STOP BUSY, want only process 1\n%s", n, out.String())
	}
}

// TestLendWhileReading_alone: with no other process, the prompt just
// waits for the line; a line already typed ends it at once.
func TestLendWhileReading_alone(t *testing.T) {
	c, _, _ := promptConsole(t)

	if took := awaitLine(t, c, 50*time.Millisecond); took < 50*time.Millisecond {
		t.Errorf("LendWhileReading returned after %v, before the line", took)
	}

	line := make(chan struct{})
	close(line)
	c.LendWhileReading(line)

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes, want only process 1", n)
	}

}
