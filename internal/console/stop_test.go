package console_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
)

// Phase 45's subtask 12: STOP, and the deletion of every other process
// when the machine is reinitialized or the session ends.

// TestStop: both of VMS's forms, STOP process-name and
// STOP/IDENTIFICATION=pid (and /ID, an abbreviation), delete the
// process; process 1 and a process that isn't there are refused.
func TestStop(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, counter())
	c.RTL.Process.Name = "SYSTEM"

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	byName := handBuiltProcess(t, c, counter())
	byName.Process.Name = "VICTIM"
	byPID := handBuiltProcess(t, c, counter())
	byAbbrev := handBuiltProcess(t, c, counter())

	if _, err := sayConsole(t, c, d, "STOP VICTIM"); err != nil || !byName.Deleted {
		t.Errorf("STOP VICTIM: %v, deleted %v", err, byName.Deleted)
	}

	line := "STOP/IDENTIFICATION=" + strings.TrimLeft(hex(byPID.Process.PID), "0")
	if _, err := sayConsole(t, c, d, line); err != nil || !byPID.Deleted {
		t.Errorf("%s: %v, deleted %v", line, err, byPID.Deleted)
	}

	line = "STOP/ID=" + hex(byAbbrev.Process.PID)
	if _, err := sayConsole(t, c, d, line); err != nil || !byAbbrev.Deleted {
		t.Errorf("%s: %v, deleted %v", line, err, byAbbrev.Deleted)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes left, want only process 1", n)
	}

	for _, tt := range []struct{ line, want string }{
		{"STOP VICTIM", "NONEXPR"},
		{"STOP/IDENTIFICATION=399", "NONEXPR"},
		{"STOP/IDENTIFICATION=XYZ", "IVIDENT"},
		{"STOP SYSTEM", "NOPRIV"},
		{"STOP", "NOPRIV"},
		{"STOP/IDENTIFICATION=301", "NOPRIV"},
	} {
		if _, err := sayConsole(t, c, d, tt.line); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: %v, want %s", tt.line, err, tt.want)
		}
	}
}

// TestStop_subprocesses: stopping an owner deletes its subprocesses, and
// INIT deletes all that are left.
func TestStop_subprocessesAndInit(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, counter())
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	owner := handBuiltProcess(t, c, counter())
	sub := newSub(t, owner)
	other := handBuiltProcess(t, c, counter())

	if _, err := sayConsole(t, c, d, "STOP/ID="+hex(owner.Process.PID)); err != nil {
		t.Fatal(err)
	}

	if !owner.Deleted || !sub.Deleted || other.Deleted {
		t.Errorf("owner %v, subprocess %v, other %v; want true, true, false", owner.Deleted, sub.Deleted, other.Deleted)
	}

	one := c.RTL

	if err := c.Init(1 << 20); err != nil {
		t.Fatal(err)
	}

	if !other.Deleted {
		t.Error("INIT left a process alive")
	}

	if len(one.Processes()) != 1 {
		t.Errorf("the old system holds %d processes, want only process 1", len(one.Processes()))
	}
}

func hex(pid uint32) string { return fmt.Sprintf("%08X", pid) }

// newSub makes a subprocess of owner.
func newSub(t *testing.T, owner *corevms.Environment) *corevms.Environment {
	t.Helper()

	sub, err := corevms.NewSubprocess(owner, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	return sub
}
