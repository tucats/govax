package corevms

import (
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
)

// Process deletion (docs/PHASE-45.md, subtask 6). The fixture has no
// address spaces or S0 pool, so these tests are of the rundown and the
// bookkeeping; giving the memory back is tested with a booted machine,
// in internal/console's deleteprc_test.go.

// TestDeleteProcess_rundown: deleting a subprocess deassigns its
// channels in every mode (its kernel-mode temporary mailbox, with the
// mailbox's name in the job table, goes with its last channel), forgets
// its ASTs and exit handlers in every mode, takes it out of the table,
// the scheduler, and its job's and owner's counts, and gives its owner
// back the CPU time it took and didn't use.
func TestDeleteProcess_rundown(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)
	defineTestDevice(parent, "TTA0", iodev.DeviceClassTT)

	child := newSubprocess(t, parent)
	a := newArena(t, parent)

	// The fixture runs in kernel mode: image rundown alone would keep
	// all of this.
	tt := assignCall(t, child, a, "TTA0", uint32(vax.Kernel))
	r0, _ := crembx(t, child, a, 0, 0, 0, "CHILD_MBX")
	wantR0(t, r0, ssNormal)

	child.queueAST(0x1000, 0, uint32(vax.Kernel))
	child.Process.exitHandlers[vax.Executive] = []uint32{0x2000}

	// The child took 400 of its owner's 1000 (10ms units) and used 100.
	parent.Process.CPULimit = 600
	child.Process.cpuDeducted = 400
	child.cpuTime = 100 * cpuQuotaUnit

	pid := child.Process.PID
	parent.DeleteProcess(child)

	if !child.Deleted {
		t.Error("the child isn't marked deleted")
	}

	if _, found := parent.FindProcess(pid); found {
		t.Error("the child is still in the process table")
	}

	if _, ok := parent.Scheduler().Info(handle(child)); ok {
		t.Error("the child is still in the scheduler")
	}

	if len(child.channels) != 0 || tt == 0 {
		t.Errorf("%d channels left", len(child.channels))
	}

	if len(parent.Mailboxes.All()) != 0 {
		t.Error("the child's temporary mailbox outlived it")
	}

	if _, err := parent.Logicals.Translate("LNM$JOB", "CHILD_MBX", lnm.User, 0); err == nil {
		t.Error("the mailbox's name is still in the job table")
	}

	if d, _ := parent.Devices.Find("TTA0"); d.RefCnt != 0 {
		t.Errorf("TTA0's reference count is %d", d.RefCnt)
	}

	if len(child.Process.ast.queue) != 0 || child.Process.exitHandlers[vax.Executive] != nil {
		t.Error("the child's kernel AST or executive exit handler survived")
	}

	if parent.Process.SubprocessCount != 0 || parent.Process.Job.SubprocessCount != 0 {
		t.Errorf("owner's count %d, job's %d; want 0", parent.Process.SubprocessCount, parent.Process.Job.SubprocessCount)
	}

	if parent.Process.CPULimit != 900 {
		t.Errorf("the owner's CPU limit is %d, want 900 (600 + the 300 unused)", parent.Process.CPULimit)
	}

	// Deleting it again does nothing.
	parent.DeleteProcess(child)

	if parent.Process.CPULimit != 900 {
		t.Errorf("a second deletion changed the owner's CPU limit to %d", parent.Process.CPULimit)
	}
}

// TestDeleteProcess_overspentCPU: a subprocess that used more than the
// CPU time it took gives nothing back.
func TestDeleteProcess_overspentCPU(t *testing.T) {
	parent, _ := fixture()
	child := newSubprocess(t, parent)

	parent.Process.CPULimit = 600
	child.Process.cpuDeducted = 400
	child.cpuTime = 500 * cpuQuotaUnit

	parent.DeleteProcess(child)

	if parent.Process.CPULimit != 600 {
		t.Errorf("the owner's CPU limit is %d, want 600", parent.Process.CPULimit)
	}
}

// TestDeleteProcess_jobTable: deleting a detached process whose job
// $CREPRC gave a logical-name table deletes the table; deleting one that
// shares process 1's names leaves process 1's job table alone.
func TestDeleteProcess_jobTable(t *testing.T) {
	parent, _ := fixture()
	a := newArena(t, parent)

	detached := newProcess(t, parent)
	table := parent.Logicals.NewJobTable()
	detached.Logicals = parent.Logicals.NewProcessView(detached.Process.UIC, table)
	detached.Process.Job.LogicalTable = table

	list := a.items(item{code: lnmString, buflen: 1, buf: a.str("X")})
	wantR0(t, callLNM(t, detached, serviceSysCrelnm, 0, a.desc("LNM$JOB"), a.desc("JOBNAME"), 0, list), ssNormal)

	sharing := newProcess(t, parent)

	parent.DeleteProcess(detached)
	parent.DeleteProcess(sharing)

	if _, err := parent.Logicals.ResolveTables(table, lnm.User); err == nil {
		t.Errorf("the detached process's job table %s outlived it", table)
	}

	if _, err := parent.Logicals.ResolveTables(parent.Logicals.JobTableName, lnm.User); err != nil {
		t.Errorf("process 1's job table went: %v", err)
	}

	if got := len(parent.Processes()); got != 1 {
		t.Errorf("%d processes left, want 1", got)
	}
}
