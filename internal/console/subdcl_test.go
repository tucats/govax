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

// TestSpawnDCL_nested: a subprocess's DCL can SPAWN (Phase 51, subtask
// 2). Its subprocess is a subprocess of the subprocess; without /NOWAIT
// the spawning DCL waits for it (the machine runs it meanwhile), and
// SPAWN's status is its final status; with /NOWAIT the spawning DCL goes
// on at once, and the new process runs beside it.
func TestSpawnDCL_nested(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	dir := t.TempDir()

	inner := filepath.Join(dir, "inner.com")
	if err := os.WriteFile(inner, []byte(strings.Join([]string{
		`$ OWNER = F$GETJPI("","OWNER")`,
		`$ WRITE SYS$OUTPUT "inner, depth ''F$ENVIRONMENT("DEPTH")', owner ''F$GETJPI(OWNER,"PRCNAM")'"`,
		"$ EXIT 3",
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outer := filepath.Join(dir, "outer.com")
	if err := os.WriteFile(outer, []byte(strings.Join([]string{
		`$ WRITE SYS$OUTPUT "outer"`,
		`$ SPAWN @"` + inner + `"`,
		`$ WRITE SYS$OUTPUT "outer again, ''$STATUS'"`,
		`$ SPAWN/NOWAIT/PROCESS=SIDE WRITE SYS$OUTPUT "beside"`,
		"$ WAIT 00:00:00.20",
		`$ WRITE SYS$OUTPUT "outer done"`,
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dispatch := func(line string) string {
		t.Helper()

		out.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			out.WriteString("%" + err.Error() + "\n")
		}

		return strings.Join(programLines(out.String()), "|")
	}

	want := strings.Join([]string{
		"%DCL-S-SPAWNED, process SYSTEM_1 spawned",
		"%DCL-S-ATTACHED, terminal now attached to process SYSTEM_1",
		"%DCL-S-SPAWNED, process SYSTEM_2 spawned",
		"%DCL-S-ATTACHED, terminal now attached to process SYSTEM_2",
		"inner, depth 1, owner SYSTEM_1",
		"%DCL-S-RETURNED, control returned to process SYSTEM_1",
		"%DCL-S-RETURNED, control returned to process SYSTEM",
	}, "|")

	if got := dispatch(`SPAWN SPAWN @"` + inner + `"`); got != want {
		t.Errorf("SPAWN SPAWN:\n%q\nwant\n%q", got, want)
	}

	if status := c.Status(); status != 3 {
		t.Errorf("SPAWN SPAWN's $STATUS %08X, want 3 (the innermost's)", status)
	}

	want = strings.Join([]string{
		"%DCL-S-SPAWNED, process SYSTEM_1 spawned",
		"%DCL-S-ATTACHED, terminal now attached to process SYSTEM_1",
		"outer",
		"%DCL-S-SPAWNED, process SYSTEM_2 spawned",
		"%DCL-S-ATTACHED, terminal now attached to process SYSTEM_2",
		"inner, depth 1, owner SYSTEM_1",
		"%DCL-S-RETURNED, control returned to process SYSTEM_1",
		"outer again, %X00000003",
		"%DCL-S-SPAWNED, process SIDE spawned",
		"beside",
		"outer done",
		"%DCL-S-RETURNED, control returned to process SYSTEM",
	}, "|")

	if got := dispatch(`SPAWN @"` + outer + `"`); got != want {
		t.Errorf("SPAWN @outer:\n%q\nwant\n%q", got, want)
	}

	// A SPAWN that fails in the subprocess shows LIB$SPAWN's status
	// there, and the subprocess's SPAWN fails with it.
	want = strings.Join([]string{
		"%DCL-S-SPAWNED, process SYSTEM_1 spawned",
		"%DCL-S-ATTACHED, terminal now attached to process SYSTEM_1",
		"%SYSTEM-F-DUPLNAM, duplicate name",
		"%DCL-S-RETURNED, control returned to process SYSTEM",
		"%SYSTEM-F-DUPLNAM, duplicate name",
	}, "|")

	if got := dispatch(`SPAWN SPAWN/PROCESS=SYSTEM_1 WRITE SYS$OUTPUT "no"`); got != want {
		t.Errorf("SPAWN SPAWN/PROCESS=SYSTEM_1:\n%q\nwant\n%q", got, want)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes at the end, want only process 1", n)
	}
}

// TestSpawnDCL_nestedInherits: a subprocess's subprocess starts with a
// copy of its parent's symbols and its parent's verification, not
// process 1's (Phase 52: while a subprocess's DCL ran, its own state was
// where process 1's was kept, and a SPAWN there copied process 1's).
func TestSpawnDCL_nestedInherits(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	dir := t.TempDir()

	// The inner procedure's line is substituted by the inner process.
	inner := filepath.Join(dir, "inner.com")
	if err := os.WriteFile(inner, []byte(`$ WRITE SYS$OUTPUT "inner ''G' ''F$ENVIRONMENT("VERIFY_PROCEDURE")'"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outer := filepath.Join(dir, "outer.com")
	if err := os.WriteFile(outer, []byte(strings.Join([]string{
		`$ G == "outer"`,
		"$ SET VERIFY",
		`$ SPAWN @"` + inner + `"`,
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{`G == "one"`, `SPAWN @"` + outer + `"`} {
		out.Reset()

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	got := strings.Join(programLines(out.String()), "|")
	if !strings.Contains(got, "|inner outer TRUE|") {
		t.Errorf("SPAWN from a subprocess: %q; want the inner process to see its parent's G and verification", got)
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
