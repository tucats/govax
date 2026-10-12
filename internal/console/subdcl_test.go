package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// Phase 50, subtask 19: a subprocess runs the console's whole DCL
// (subcli.go), with its own symbols, command levels, and $STATUS.

// TestSpawnDCL: SPAWN of one of DCL's commands runs it in the subprocess
// and logs out; SPAWN of a command procedure runs it there, with labels,
// IF, a GOTO loop, an image that reads the procedure's data lines (the
// subprocess's SYS$INPUT while the line runs), lexical functions, and
// DCL's file I/O; the
// subprocess has a copy of process 1's symbols, and process 1's own
// symbols and command level are as they were. A command that works the
// machine itself is process 1's.
func TestSpawnDCL(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	reader := buildImage(t, c, "reader", strings.Replace(readerSource, "ONCE", "", 1))

	dir := t.TempDir()

	text := filepath.Join(dir, "text.txt")
	if err := os.WriteFile(text, []byte("typed by the subprocess\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	proc := filepath.Join(dir, "sub.com")
	if err := os.WriteFile(proc, []byte(strings.Join([]string{
		"$ X = 2",
		`$ IF X .GT. 1 THEN WRITE SYS$OUTPUT "big ''X' ''G'"`,
		"$ COUNT = 0",
		"$ LOOP:",
		"$ COUNT = COUNT + 1",
		`$ RUN "` + reader + `"`,
		"line ''COUNT'",
		"$ IF COUNT .LT. 2 THEN GOTO LOOP",
		`$ OWN = F$GETJPI("","PID") .EQS. F$GETJPI("","OWNER")`,
		`$ WRITE SYS$OUTPUT "depth ''F$ENVIRONMENT("DEPTH")' own ''OWN'"`,
		`$ OPEN/READ IN "` + text + `"`,
		"$ READ IN RECORD",
		"$ CLOSE IN",
		"$ WRITE SYS$OUTPUT RECORD",
		"$ EXIT 3",
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dispatch := func(line string) string {
		t.Helper()

		out.Reset()

		// A SPAWN whose subprocess failed fails too, its message shown.
		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			out.WriteString("%" + err.Error() + "\n")
		}

		return strings.Join(programLines(out.String()), "|")
	}

	dispatch(`G == "global"`)
	dispatch(`L = "local"`)

	const (
		attached = "%DCL-S-SPAWNED, process SYSTEM_1 spawned|%DCL-S-ATTACHED, terminal now attached to process SYSTEM_1|"
		returned = "%DCL-S-RETURNED, control returned to process SYSTEM"
	)

	if got := dispatch(`SPAWN WRITE SYS$OUTPUT "hello ''G'"`); got != attached+"hello global|"+returned {
		t.Errorf("SPAWN WRITE: %q", got)
	}

	want := attached + "big 2 global|line ''COUNT'|end of input|line ''COUNT'|end of input|depth 1 own 0|typed by the subprocess|" + returned
	if got := dispatch(`SPAWN @"` + proc + `"`); got != want {
		t.Errorf("SPAWN @:\n%q\nwant\n%q", got, want)
	}

	if got := dispatch("SPAWN DEBUG"); !strings.Contains(got, "%VAX-E-NOTSUBPROC, DEBUG is not available in a subprocess") {
		t.Errorf("SPAWN DEBUG: %q", got)
	}

	// Process 1's DCL is as it was.
	if got := dispatch(`WRITE SYS$OUTPUT "''L' ''F$TYPE(X)'/''F$TYPE(COUNT)' ''F$ENVIRONMENT("DEPTH")'"`); got != "local / 0" {
		t.Errorf("process 1's symbols afterward: %q", got)
	}

	if n := len(c.RTL.Processes()); n != 1 || c.CommandLevel() != 0 {
		t.Errorf("%d processes, level %d at the end; want only process 1, level 0", n, c.CommandLevel())
	}
}

// TestSpawnDCL_stopWhileRunning: STOP of a subprocess whose procedure
// is running an image deletes it, and its interpreter, waiting for the
// image, is unwound; process 1 goes on as before.
func TestSpawnDCL_stopWhileRunning(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	forever := buildImage(t, c, "forever", strings.Replace(spinSource, "COUNT", "0", 1))
	child := buildChildImage(t, c)

	proc := filepath.Join(t.TempDir(), "spin.com")
	if err := os.WriteFile(proc, []byte(`$ RUN "`+forever+`"`+"\n$ WRITE SYS$OUTPUT \"after\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{`SPAWN/NOWAIT/PROCESS=SPIN @"` + proc + `"`} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	// The machine runs: SPIN gets to its image while the child runs.
	if err := c.Run(child, console.RunOptions{}); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	if err := d.Dispatch("STOP SPIN"); err != nil {
		t.Fatalf("STOP SPIN: %v", err)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes after STOP, want only process 1", n)
	}

	if err := d.Dispatch(`WRITE SYS$OUTPUT "still ''F$ENVIRONMENT("DEPTH")'"`); err != nil || strings.Contains(out.String(), "after") ||
		!strings.Contains(out.String(), "still 0") {
		t.Errorf("after STOP: %v, %q", err, out.String())
	}
}
