package corevms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
)

// $DELPRC of another process, and subprocesses deleted with their owner
// (docs/PHASE-45.md, subtask 8). The fixture has no hardware PCBs, so
// the scheduler can't switch to a marked process here: these tests mark,
// wait, and then run each deletion as switchTo would. The deletions in
// each process's own context, chosen by the scheduler, are tested with a
// booted machine, in internal/console's delprc_test.go.

// delprc calls $DELPRC with the PID pid, by reference.
func delprc(t *testing.T, env *Environment, a *arena, pid uint32) uint32 {
	t.Helper()

	r0, err := serviceSysDelprc(env, []uint32{a.long(pid)})
	if err != nil {
		t.Fatalf("$DELPRC of %08X: %v", pid, err)
	}

	return r0
}

// TestDelprc_markOther: with the scheduler, $DELPRC of another process
// returns at once and only marks it: a hibernating target is made
// computable (boosted), and its final status is SS$_ABORT. Marking it
// again succeeds and changes nothing. Its deletion, when it gets the CPU,
// is the full one.
func TestDelprc_markOther(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	child := newSubprocess(t, parent)

	if err := callWaiting(child, serviceSysHiber); !errors.Is(err, ErrWait) {
		t.Fatalf("$HIBER = %v, want ErrWait", err)
	}

	wantState(t, child, sched.StateHIB, sched.ResourceNone)

	wantR0(t, delprc(t, parent, a, child.Process.PID), ssNormal)

	if child.Deleted || !child.deletePending {
		t.Fatalf("deleted %v, marked %v; want only marked", child.Deleted, child.deletePending)
	}

	wantState(t, child, sched.StateCOM, sched.ResourceNone)

	if child.waiting != nil || parent.waiters != 0 {
		t.Error("the child still counts as waiting")
	}

	if child.Process.ExitStatus != ssAbort {
		t.Errorf("final status %08X, want SS$_ABORT", child.Process.ExitStatus)
	}

	wantR0(t, delprc(t, parent, a, child.Process.PID), ssNormal)

	// What switchTo does once the child has the CPU.
	parent.DeleteProcess(child)

	if !child.Deleted || child.deletePending {
		t.Errorf("deleted %v, marked %v; want deleted", child.Deleted, child.deletePending)
	}

	if parent.Process.SubprocessCount != 0 {
		t.Errorf("the owner's count is %d, want 0", parent.Process.SubprocessCount)
	}

	wantR0(t, delprc(t, parent, a, child.Process.PID), ssNonExpr)
}

// TestDelprc_exitStatusKept: a process marked after its image called
// $EXIT keeps that status as its final one.
func TestDelprc_exitStatusKept(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	child := newSubprocess(t, parent)
	child.Process.ExitStatus = 0x1000_0003

	wantR0(t, delprc(t, parent, a, child.Process.PID), ssNormal)

	if child.Process.ExitStatus != 0x1000_0003 {
		t.Errorf("final status %08X, want 10000003", child.Process.ExitStatus)
	}
}

// TestDelprc_noScheduler: without the scheduler, nothing would give a
// marked process the CPU, so $DELPRC deletes it at once.
func TestDelprc_noScheduler(t *testing.T) {
	parent, _ := fixture()
	a := newArena(t, parent)
	child := newSubprocess(t, parent)

	wantR0(t, delprc(t, parent, a, child.Process.PID), ssNormal)

	if !child.Deleted {
		t.Error("the child wasn't deleted")
	}

	if _, found := parent.FindProcess(child.Process.PID); found {
		t.Error("the child is still in the process table")
	}
}

// TestDelprc_privileges: a process with the caller's UIC needs no
// privilege; another in its group needs GROUP (or WORLD); one in another
// group needs WORLD. Without, SS$_NOPRIV, and the target is untouched.
func TestDelprc_privileges(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	p := parent.Process

	sameGroup := newProcess(t, parent)
	sameGroup.Process.UIC = p.UIC + 1

	otherGroup := newProcess(t, parent)
	otherGroup.Process.UIC = p.UIC + 0x10000

	sameUIC := newProcess(t, parent)

	p.CurrentPrivileges &^= privGROUP | privWORLD

	for _, target := range []*Environment{sameGroup, otherGroup} {
		wantR0(t, delprc(t, parent, a, target.Process.PID), ssNoPriv)

		if target.deletePending {
			t.Errorf("%08X was marked without the privilege", target.Process.PID)
		}
	}

	wantR0(t, delprc(t, parent, a, sameUIC.Process.PID), ssNormal)

	p.CurrentPrivileges |= privGROUP

	wantR0(t, delprc(t, parent, a, otherGroup.Process.PID), ssNoPriv)
	wantR0(t, delprc(t, parent, a, sameGroup.Process.PID), ssNormal)

	p.CurrentPrivileges = p.CurrentPrivileges&^privGROUP | privWORLD

	wantR0(t, delprc(t, parent, a, otherGroup.Process.PID), ssNormal)

	for _, target := range []*Environment{sameGroup, otherGroup, sameUIC} {
		if !target.deletePending {
			t.Errorf("%08X wasn't marked", target.Process.PID)
		}
	}
}

// TestDelprc_processOne: another process's $DELPRC of process 1 doesn't
// delete it: its exit handlers are forgotten and a user-mode AST to
// $EXIT, with SS$_NORMAL, is queued, once.
func TestDelprc_processOne(t *testing.T) {
	one, _ := fixture()
	withScheduler(one)

	child := newSubprocess(t, one)
	a := newArena(t, child)

	one.Process.exitHandlers[vax.User] = []uint32{0x2000}

	wantR0(t, delprc(t, child, a, one.Process.PID), ssNormal)
	wantR0(t, delprc(t, child, a, one.Process.PID), ssNormal)

	if one.Deleted || one.deletePending {
		t.Error("process 1 was marked for deletion")
	}

	if one.ExitHandlers(vax.User) != 0 {
		t.Error("process 1's exit handler survived")
	}

	q := one.Process.ast.queue
	if len(q) != 1 || q[0].routine != exitEntryAddr || q[0].param != ssNormal || q[0].mode != uint32(vax.User) {
		t.Errorf("process 1's AST queue is %+v, want one user-mode $EXIT(SS$_NORMAL)", q)
	}
}

// TestDeleteProcess_ownerWaits: deleting a process that owns
// subprocesses marks them and makes it wait (MWAIT, RWAST), marked
// itself, ASTs or not, until the last of them has gone; then its own
// deletion runs. A grandchild is marked when its parent's deletion
// runs, and its parent waits for it in turn.
func TestDeleteProcess_ownerWaits(t *testing.T) {
	one, _ := fixture()
	withScheduler(one)

	owner := newSubprocess(t, one)
	first := newSubprocess(t, owner)
	second := newSubprocess(t, owner)
	grandchild := newSubprocess(t, second)

	one.DeleteProcess(owner)

	if owner.Deleted || !owner.deletePending {
		t.Fatalf("owner deleted %v, marked %v; want marked, waiting", owner.Deleted, owner.deletePending)
	}

	wantState(t, owner, sched.StateMWAIT, sched.ResourceAST)

	for _, sub := range []*Environment{first, second} {
		if !sub.deletePending || sub.Process.ExitStatus != ssAbort {
			t.Errorf("subprocess %08X: marked %v, status %08X; want marked, SS$_ABORT",
				sub.Process.PID, sub.deletePending, sub.Process.ExitStatus)
		}
	}

	if grandchild.deletePending {
		t.Error("the grandchild was marked before its owner's deletion ran")
	}

	// An AST doesn't end the owner's wait.
	owner.queueAST(0x1000, 0, uint32(vax.User))

	if one.wakeWaiters() {
		t.Error("the waiting owner was woken with subprocesses left")
	}

	one.DeleteProcess(first)

	// The second's deletion runs: it marks the grandchild and waits.
	one.DeleteProcess(second)

	if second.Deleted || !grandchild.deletePending {
		t.Fatalf("second deleted %v, grandchild marked %v; want the second waiting for it",
			second.Deleted, grandchild.deletePending)
	}

	if one.wakeWaiters() {
		t.Error("a waiter was woken while the grandchild lives")
	}

	one.DeleteProcess(grandchild)

	if !one.wakeWaiters() {
		t.Fatal("the grandchild's deletion woke nobody")
	}

	wantState(t, second, sched.StateCOM, sched.ResourceNone)
	wantState(t, owner, sched.StateMWAIT, sched.ResourceAST)

	one.DeleteProcess(second)

	if !one.wakeWaiters() {
		t.Fatal("the last subprocess's deletion didn't wake the owner")
	}

	one.DeleteProcess(owner)

	for _, env := range []*Environment{owner, first, second, grandchild} {
		if !env.Deleted || env.deletePending || env.waiting != nil {
			t.Errorf("%08X: deleted %v, marked %v, waiting %v; want deleted only",
				env.Process.PID, env.Deleted, env.deletePending, env.waiting != nil)
		}
	}

	if got := len(one.Processes()); got != 1 || one.waiters != 0 {
		t.Errorf("%d processes, %d waiters left; want 1, 0", got, one.waiters)
	}

	if one.Process.SubprocessCount != 0 || one.Process.Job.SubprocessCount != 0 {
		t.Errorf("process 1's count %d, job's %d; want 0", one.Process.SubprocessCount, one.Process.Job.SubprocessCount)
	}
}

// TestDeleteProcess_treeNoScheduler: without the scheduler, deleting an
// owner deletes its whole tree of subprocesses at once, leaves first.
func TestDeleteProcess_treeNoScheduler(t *testing.T) {
	one, _ := fixture()

	owner := newSubprocess(t, one)
	child := newSubprocess(t, owner)
	grandchild := newSubprocess(t, child)

	var order []uint32

	one.ProcessDeleted = func(env *Environment) { order = append(order, env.Process.PID) }

	one.DeleteProcess(owner)

	want := []uint32{grandchild.Process.PID, child.Process.PID, owner.Process.PID}
	if len(order) != 3 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
		t.Errorf("deleted %08X, want %08X", order, want)
	}

	if got := len(one.Processes()); got != 1 {
		t.Errorf("%d processes left, want 1", got)
	}
}
