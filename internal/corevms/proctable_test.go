package corevms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// newProcess adds another process to env's System, failing the test if
// it can't.
func newProcess(t *testing.T, env *Environment) *Environment {
	t.Helper()

	next, err := NewEnvironment(env.System, env.Logicals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	return next
}

// TestProcessTable_PIDs: process 1 keeps PID 00000301; later processes
// take the next free index with their slot's first sequence number; the
// first process is the current one.
func TestProcessTable_PIDs(t *testing.T) {
	env, _ := fixture()

	if env.Process.PID != 0x301 {
		t.Errorf("process 1's PID = %08X, want 00000301", env.Process.PID)
	}

	p2 := newProcess(t, env)
	p3 := newProcess(t, env)

	if p2.Process.PID != 0x302 || p3.Process.PID != 0x303 {
		t.Errorf("PIDs %08X, %08X; want 00000302, 00000303", p2.Process.PID, p3.Process.PID)
	}

	if env.Current() != env {
		t.Error("the first process isn't the current one")
	}

	list := env.Processes()
	if len(list) != 3 || list[0] != env || list[1] != p2 || list[2] != p3 {
		t.Errorf("Processes() = %v, want the three in index order", list)
	}

	for _, p := range list {
		if found, ok := env.FindProcess(p.Process.PID); !ok || found != p {
			t.Errorf("FindProcess(%08X) didn't find its process", p.Process.PID)
		}
	}
}

// TestProcessTable_reuse: a deleted process's slot goes to the next new
// process, with the slot's sequence number moved on, so the old PID
// finds nothing (VAX/VMS Internals and Data Structures, 20.1.4).
func TestProcessTable_reuse(t *testing.T) {
	env, _ := fixture()
	p2 := newProcess(t, env)
	old := p2.Process.PID

	env.RemoveProcess(p2)

	if _, ok := env.FindProcess(old); ok {
		t.Errorf("FindProcess(%08X) found a removed process", old)
	}

	p2b := newProcess(t, env)
	if p2b.Process.PID != 0x402 {
		t.Errorf("reused slot's PID = %08X, want 00000402", p2b.Process.PID)
	}

	if _, ok := env.FindProcess(old); ok {
		t.Errorf("the old PID %08X found the slot's new process", old)
	}

	// Removing the current process leaves none current.
	env.RemoveProcess(env)

	if env.Current() != nil {
		t.Error("removing the current process left it current")
	}

	env.SetCurrent(p2b)

	if env.Current() != p2b {
		t.Error("SetCurrent didn't make its process current")
	}
}

// TestProcessTable_sequenceWraps: a slot's sequence number wraps within
// its 13 bits, skipping 0.
func TestProcessTable_sequenceWraps(t *testing.T) {
	env, _ := fixture()
	env.procs.sequence[2] = pidSequenceMask

	if p := newProcess(t, env); p.Process.PID != pid(2, 1) {
		t.Errorf("PID after the sequence wraps = %08X, want %08X", p.Process.PID, pid(2, 1))
	}
}

// TestProcessTable_full: once every slot is taken, a new process fails
// with SS$_NOSLOT.
func TestProcessTable_full(t *testing.T) {
	env, _ := fixture()

	for range MaxProcesses - 1 {
		newProcess(t, env)
	}

	_, err := NewEnvironment(env.System, env.Logicals, nil, nil)

	var vmsErr vmserrors.VMSError
	if !errors.As(err, &vmsErr) || vmsErr.Status != ssNoSlot {
		t.Errorf("NewEnvironment on a full table: %v, want SS$_NOSLOT", err)
	}
}

// TestProcessTable_names: a process name is unique within a UIC group. A
// new process whose default name is taken in its group gets none, and
// FindProcessName only looks in the group it's given.
func TestProcessTable_names(t *testing.T) {
	env, _ := fixture()
	p2 := newProcess(t, env)

	if p2.Process.Name != "" {
		t.Errorf("a second SYSTEM process is named %q, want no name", p2.Process.Name)
	}

	other := newProcess(t, env)
	other.Process.UIC = 0o200<<16 | 1 // [200,1]: group 200 octal
	other.Process.Name = "SYSTEM"

	if found, ok := env.FindProcessName(1, "SYSTEM"); !ok || found != env {
		t.Error("FindProcessName(group 1, SYSTEM) didn't find process 1")
	}

	if found, ok := env.FindProcessName(0o200, "SYSTEM"); !ok || found != other {
		t.Error("FindProcessName(group 200, SYSTEM) didn't find that group's process")
	}

	if _, ok := env.FindProcessName(1, ""); ok {
		t.Error("an empty name found a process")
	}
}

// TestProcessTarget_table: processTarget finds any process in the table
// by PID, and by name in the caller's group, writing a named process's
// PID back; callerTarget still answers SS$_NONEXPR for anyone but the
// caller.
func TestProcessTarget_table(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p2 := newProcess(t, env)
	p2.Process.Name = "WORKER"

	if target, st := env.processTarget(a.long(p2.Process.PID), 0, false); st != 0 || target != p2 {
		t.Errorf("by PID: %v, status %d; want process 2", target, st)
	}

	pidadr := a.long(0)
	if target, st := env.processTarget(pidadr, a.desc("WORKER"), false); st != 0 || target != p2 {
		t.Errorf("by name: %v, status %d; want process 2", target, st)
	}

	if a.readLong(pidadr) != p2.Process.PID {
		t.Errorf("PID written back = %08X, want %08X", a.readLong(pidadr), p2.Process.PID)
	}

	if _, st := env.processTarget(a.long(0x999), 0, false); st != ssNonExpr {
		t.Errorf("an unknown PID: status %d, want SS$_NONEXPR", st)
	}

	// A name in another group isn't found.
	p2.Process.UIC = 0o200<<16 | 1

	if _, st := env.processTarget(0, a.desc("WORKER"), false); st != ssNonExpr {
		t.Errorf("a name in another group: status %d, want SS$_NONEXPR", st)
	}

	wantR0(t, env.callerTarget(a.long(p2.Process.PID), 0, false), ssNonExpr)
	wantR0(t, env.callerTarget(a.long(env.Process.PID), 0, false), 0)
}

// TestSetprn_duplicateName: $SETPRN can't take a name another process in
// the caller's group has, but may take one from another group, or the
// caller's own name again.
func TestSetprn_duplicateName(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p2 := newProcess(t, env)
	p2.Process.Name = "WORKER"

	wantR0(t, callLNM(t, env, serviceSysSetprn, a.desc("WORKER")), ssDuplNam)

	if env.Process.Name != "SYSTEM" {
		t.Errorf("name = %q after SS$_DUPLNAM, want SYSTEM", env.Process.Name)
	}

	wantR0(t, callLNM(t, env, serviceSysSetprn, a.desc("SYSTEM")), ssNormal)

	p2.Process.UIC = 0o200<<16 | 1
	wantR0(t, callLNM(t, env, serviceSysSetprn, a.desc("WORKER")), ssNormal)
}
