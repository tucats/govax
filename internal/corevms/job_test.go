package corevms

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// newSubprocess creates a subprocess of owner, failing the test if it
// can't.
func newSubprocess(t *testing.T, owner *Environment) *Environment {
	t.Helper()

	child, err := NewSubprocess(owner, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	return child
}

// jpiLong returns a longword $GETJPI item of env, as the item table
// computes it.
func jpiLong(t *testing.T, env *Environment, name string) uint32 {
	t.Helper()

	return binary.LittleEndian.Uint32([]byte(jpiItemsByName[name](env).data))
}

// TestJob_processOne: process 1 is a detached process, the master of a
// job of its own, with no owner and no subprocesses.
func TestJob_processOne(t *testing.T) {
	env, _ := fixture()
	p := env.Process

	if p.Job == nil || p.Job.MasterPID != p.PID || p.Owner != 0 || p.SubprocessCount != 0 {
		t.Fatalf("process 1: job %+v, owner %08X, %d subprocesses; want its own job, no owner",
			p.Job, p.Owner, p.SubprocessCount)
	}

	if p.Job.SubprocessLimit != nominalPRCLM || p.Job.SubprocessCount != 0 {
		t.Errorf("job PRCLM %d, count %d; want %d, 0", p.Job.SubprocessLimit, p.Job.SubprocessCount, nominalPRCLM)
	}

	// Another process made by NewEnvironment is detached too: a second
	// job.
	other := newProcess(t, env)
	if other.Process.Job == p.Job || other.Process.Job.MasterPID != other.Process.PID || other.Process.Owner != 0 {
		t.Errorf("a second detached process shares process 1's job, or has an owner")
	}
}

// TestJob_subprocesses: a subprocess is in its creator's job, owned by
// it; each process counts the subprocesses it created, and the job
// counts them all (VAX/VMS Internals and Data Structures, figure 20-2).
// $GETJPI reports the same relationships.
func TestJob_subprocesses(t *testing.T) {
	w, _ := fixture()
	x := newSubprocess(t, w)
	y := newSubprocess(t, w)
	z := newSubprocess(t, y)

	job := w.Process.Job

	for _, c := range []struct {
		env   *Environment
		owner uint32
		count uint32
	}{
		{w, 0, 2},
		{x, w.Process.PID, 0},
		{y, w.Process.PID, 1},
		{z, y.Process.PID, 0},
	} {
		p := c.env.Process
		if p.Job != job || p.Owner != c.owner || p.SubprocessCount != c.count {
			t.Errorf("%08X: owner %08X, %d subprocesses, same job %v; want %08X, %d, true",
				p.PID, p.Owner, p.SubprocessCount, p.Job == job, c.owner, c.count)
		}

		if got := jpiLong(t, c.env, "JPI$_OWNER"); got != c.owner {
			t.Errorf("%08X: JPI$_OWNER = %08X, want %08X", p.PID, got, c.owner)
		}

		if got := jpiLong(t, c.env, "JPI$_PRCCNT"); got != c.count {
			t.Errorf("%08X: JPI$_PRCCNT = %d, want %d", p.PID, got, c.count)
		}

		if got := jpiLong(t, c.env, "JPI$_MASTER_PID"); got != w.Process.PID {
			t.Errorf("%08X: JPI$_MASTER_PID = %08X, want %08X", p.PID, got, w.Process.PID)
		}

		if got := jpiLong(t, c.env, "JPI$_JOBPRCCNT"); got != 3 {
			t.Errorf("%08X: JPI$_JOBPRCCNT = %d, want 3", p.PID, got)
		}
	}

	// A subprocess takes its creator's identity.
	if z.Process.UIC != w.Process.UIC || z.Process.Username != w.Process.Username {
		t.Errorf("subprocess UIC %08X, user %q; want its creator's", z.Process.UIC, z.Process.Username)
	}

	// Deleting a subprocess gives its place back to its owner and job.
	w.RemoveProcess(z)

	if y.Process.SubprocessCount != 0 || job.SubprocessCount != 2 {
		t.Errorf("after deleting Z: Y has %d subprocesses, the job %d; want 0, 2",
			y.Process.SubprocessCount, job.SubprocessCount)
	}
}

// TestJob_PRCLM: a job can't have more subprocesses than its PRCLM
// quota; creating one more is SS$_EXQUOTA and creates nothing, and
// deleting a subprocess makes room for another.
func TestJob_PRCLM(t *testing.T) {
	env, _ := fixture()
	env.Process.Job.SubprocessLimit = 2

	a := newSubprocess(t, env)
	newSubprocess(t, a) // a subprocess's subprocess counts against the same job

	before := len(env.Processes())

	_, err := NewSubprocess(env, nil, nil)

	var vmsErr vmserrors.VMSError
	if !errors.As(err, &vmsErr) || vmsErr.Status != ssExQuota {
		t.Fatalf("a third subprocess: %v, want SS$_EXQUOTA", err)
	}

	if len(env.Processes()) != before || env.Process.SubprocessCount != 1 || env.Process.Job.SubprocessCount != 2 {
		t.Errorf("the failed creation left %d processes, owner count %d, job count %d",
			len(env.Processes()), env.Process.SubprocessCount, env.Process.Job.SubprocessCount)
	}

	env.RemoveProcess(a)
	newSubprocess(t, env)
}

// TestJob_noSlot: a subprocess that finds the process table full is
// SS$_NOSLOT, and its reservation against PRCLM is given back.
func TestJob_noSlot(t *testing.T) {
	env, _ := fixture()
	env.Process.Job.SubprocessLimit = MaxProcesses + 1

	for range MaxProcesses - 1 {
		newProcess(t, env)
	}

	_, err := NewSubprocess(env, nil, nil)

	var vmsErr vmserrors.VMSError
	if !errors.As(err, &vmsErr) || vmsErr.Status != ssNoSlot {
		t.Fatalf("NewSubprocess on a full table: %v, want SS$_NOSLOT", err)
	}

	if env.Process.Job.SubprocessCount != 0 || env.Process.SubprocessCount != 0 {
		t.Errorf("the failed creation left job count %d, owner count %d; want 0, 0",
			env.Process.Job.SubprocessCount, env.Process.SubprocessCount)
	}
}

// TestJob_logicalNames: a subprocess has a process table of its own and
// shares its job's table with its owner: a name it defines in its
// process table isn't seen by its owner, one in the job table is
// (docs/PHASE-45.md, subtask 3). A temporary mailbox's name goes in the
// job table (LNM$TEMPORARY_MAILBOX is LNM$JOB, docs/PHASE-43.md's bug 4),
// so a subprocess finds its owner's mailbox by name.
func TestJob_logicalNames(t *testing.T) {
	parent, _ := fixture()
	child := newSubprocess(t, parent)
	a := newArena(t, parent)

	define := func(env *Environment, table, name, value string) {
		t.Helper()

		list := a.items(item{code: lnmString, buflen: uint16(len(value)), buf: a.str(value)})
		wantR0(t, callLNM(t, env, serviceSysCrelnm, 0, a.desc(table), a.desc(name), 0, list), ssNormal)
	}

	translate := func(env *Environment, name string) uint32 {
		t.Helper()

		return callLNM(t, env, serviceSysTrnlnm, 0, a.desc("LNM$FILE_DEV"), a.desc(name), 0, 0)
	}

	define(child, "LNM$PROCESS", "CHILD_ONLY", "X")
	define(child, "LNM$JOB", "WHOLE_JOB", "Y")

	wantR0(t, translate(parent, "CHILD_ONLY"), vmserrors.SS_NOLOGNAM)
	wantR0(t, translate(child, "CHILD_ONLY"), ssNormal)
	wantR0(t, translate(parent, "WHOLE_JOB"), ssNormal)

	// The console's terminal names are process 1's own.
	wantR0(t, translate(child, "TT"), vmserrors.SS_NOLOGNAM)

	// A detached process is in a job of its own.
	other := newProcess(t, parent)
	other.Logicals = parent.Logicals.NewProcessView(other.Process.UIC, parent.Logicals.NewJobTable())
	wantR0(t, translate(other, "WHOLE_JOB"), vmserrors.SS_NOLOGNAM)

	r0, ch := crembx(t, parent, a, 0, 0, 0, "PARENT_MBX")
	wantR0(t, r0, ssNormal)

	r0, childCh := crembx(t, child, a, 0, 0, 0, "PARENT_MBX")
	wantR0(t, r0, ssNormal)

	if mailboxOn(t, parent, ch) != mailboxOn(t, child, childCh) {
		t.Error("the subprocess's $CREMBX of the same name made a second mailbox")
	}
}
