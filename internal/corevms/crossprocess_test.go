package corevms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
)

// The process-control services on other processes (docs/PHASE-45.md,
// subtask 9): $SCHDWK, $CANWAK, $FORCEX, $SETPRI, $SUSPND, $RESUME.
// ($WAKE and $DELPRC were earlier.) The scheduler can't switch contexts
// in this fixture, so these tests look at what each service does to the
// target's own state; the console's crossprocess_test.go runs them in
// booted machines.

// byPID calls a service with the PID by reference, then extra arguments.
func byPID(t *testing.T, env *Environment, fn ServiceFunc, a *arena, target *Environment, extra ...uint32) uint32 {
	t.Helper()

	return callLNM(t, env, fn, append([]uint32{a.long(target.Process.PID), 0}, extra...)...)
}

func TestCross_schdwkAndCanwak(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	now := fakeClock(parent)
	child := newSubprocess(t, parent)

	if err := callWaiting(child, serviceSysHiber); !errors.Is(err, ErrWait) {
		t.Fatalf("$HIBER = %v, want ErrWait", err)
	}

	// The wakeup goes to the child's queue, not the caller's.
	wantR0(t, byPID(t, parent, serviceSysSchdwk, a, child, a.quad(-50*ms)), ssNormal)

	if len(child.timers) != 1 || len(parent.timers) != 0 {
		t.Fatalf("timers: child %d, parent %d; want 1, 0", len(child.timers), len(parent.timers))
	}

	// Every process's timers expire at the scheduler's choice, and the
	// child's hibernation ends.
	*now += 51 * ms
	parent.pollEvents()

	if !child.Process.WakePending && child.waiting != nil {
		t.Error("the child was not woken")
	}

	wantState(t, child, sched.StateCOM, sched.ResourceNone)

	// $CANWAK of the child removes its queued wakeups, not other timers.
	child.Process.WakePending = false
	wantR0(t, byPID(t, parent, serviceSysSchdwk, a, child, a.quad(-50*ms)), ssNormal)
	child.timers = append(child.timers, &timerRequest{expiry: *now + 1})
	wantR0(t, byPID(t, parent, serviceSysCanwak, a, child), ssNormal)

	if len(child.timers) != 1 || child.timers[0].wake {
		t.Errorf("after $CANWAK: %d timers, want only the non-wake one", len(child.timers))
	}

	wantR0(t, byPID(t, parent, serviceSysCanwak, a, &Environment{Process: &Process{PID: 0x999}}), ssNonExpr)
}

func TestCross_forcex(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	child := newSubprocess(t, parent)

	if err := callWaiting(child, serviceSysHiber); !errors.Is(err, ErrWait) {
		t.Fatalf("$HIBER = %v, want ErrWait", err)
	}

	wantR0(t, byPID(t, parent, serviceSysForcex, a, child, 0x2C), ssNormal)
	wantR0(t, byPID(t, parent, serviceSysForcex, a, child, 0x2C), ssNormal) // not queued twice

	q := child.Process.ast.queue
	if len(q) != 1 || q[0].routine != exitEntryAddr || q[0].param != 0x2C || q[0].mode != uint32(vax.User) {
		t.Fatalf("child's AST queue = %+v", q)
	}

	if len(parent.Process.ast.queue) != 0 {
		t.Error("the caller got the AST")
	}
}

func TestCross_setpri(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	child := newSubprocess(t, parent)
	old := child.Process.BasePriority
	prv := a.long(0xFFFFFFFF)

	wantR0(t, byPID(t, parent, serviceSysSetpri, a, child, 12, prv), ssNormal)

	if got := a.readLong(prv); got != old {
		t.Errorf("prvpri %d, want %d", got, old)
	}

	if child.Process.BasePriority != 12 || child.Process.Priority != 12 {
		t.Errorf("child's priority %d/%d, want 12", child.Process.BasePriority, child.Process.Priority)
	}

	info, _ := child.Scheduler().Info(handle(child))
	if info.Base != 12 {
		t.Errorf("scheduler's base priority %d, want 12", info.Base)
	}

	if parent.Process.BasePriority == 12 {
		t.Error("the caller's priority changed")
	}

	// Without ALTPRI the caller's authorized priority limits it.
	parent.Process.CurrentPrivileges &^= privALTPRI
	parent.Process.AuthorizedPriority = 6
	wantR0(t, byPID(t, parent, serviceSysSetpri, a, child, 20), ssNormal)

	if child.Process.BasePriority != 6 {
		t.Errorf("child's priority %d, want 6", child.Process.BasePriority)
	}
}

func TestCross_privileges(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	other := newProcess(t, parent)
	other.Process.UIC = parent.Process.UIC + 0x10000
	parent.Process.CurrentPrivileges &^= privGROUP | privWORLD

	for name, fn := range map[string]ServiceFunc{
		"$WAKE": serviceSysWake, "$SCHDWK": serviceSysSchdwk, "$CANWAK": serviceSysCanwak,
		"$FORCEX": serviceSysForcex, "$SETPRI": serviceSysSetpri,
		"$SUSPND": serviceSysSuspnd, "$RESUME": serviceSysResume,
	} {
		if r0 := byPID(t, parent, fn, a, other, a.quad(-ms), 1); r0 != ssNoPriv {
			t.Errorf("%s without WORLD: %08X, want SS$_NOPRIV", name, r0)
		}
	}

	if other.suspended || other.Process.WakePending || len(other.timers) != 0 || len(other.Process.ast.queue) != 0 {
		t.Error("a refused service changed the target")
	}

	parent.Process.CurrentPrivileges |= privWORLD
	wantR0(t, byPID(t, parent, serviceSysSuspnd, a, other), ssNormal)
}

// TestCross_suspendComputable: a suspended process leaves the
// scheduler's choice, and a resumed one returns to it.
func TestCross_suspendComputable(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	child := newSubprocess(t, parent)

	wantR0(t, byPID(t, parent, serviceSysSuspnd, a, child), ssNormal)
	wantState(t, child, sched.StateSUSP, sched.ResourceNone)

	if !child.Suspended() {
		t.Error("Suspended() is false")
	}

	wantR0(t, byPID(t, parent, serviceSysSuspnd, a, child), ssSuspended)

	wantR0(t, byPID(t, parent, serviceSysResume, a, child), ssNormal)
	wantState(t, child, sched.StateCOM, sched.ResourceNone)

	// Resuming one that isn't suspended is no error.
	wantR0(t, byPID(t, parent, serviceSysResume, a, child), ssNormal)
	wantState(t, child, sched.StateCOM, sched.ResourceNone)
}

// TestCross_suspendWaiting: a suspended hibernating process isn't woken
// by a wakeup until it is resumed, and then goes back to HIB, or runs if
// the wakeup has come.
func TestCross_suspendWaiting(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	child := newSubprocess(t, parent)

	if err := callWaiting(child, serviceSysHiber); !errors.Is(err, ErrWait) {
		t.Fatalf("$HIBER = %v, want ErrWait", err)
	}

	wantR0(t, byPID(t, parent, serviceSysSuspnd, a, child), ssNormal)
	wantState(t, child, sched.StateSUSP, sched.ResourceNone)

	// Resumed with nothing happened: hibernating again.
	wantR0(t, byPID(t, parent, serviceSysResume, a, child), ssNormal)
	wantState(t, child, sched.StateHIB, sched.ResourceNone)

	// Woken while suspended: it stays suspended until the resume.
	wantR0(t, byPID(t, parent, serviceSysSuspnd, a, child), ssNormal)
	wantR0(t, byPID(t, parent, serviceSysWake, a, child), ssNormal)
	parent.pollEvents()
	wantState(t, child, sched.StateSUSP, sched.ResourceNone)

	wantR0(t, byPID(t, parent, serviceSysResume, a, child), ssNormal)
	parent.pollEvents()
	wantState(t, child, sched.StateCOM, sched.ResourceNone)
}

// TestCross_suspendSelf: a process may suspend itself, and is then SUSP.
func TestCross_suspendSelf(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	wantR0(t, callLNM(t, env, serviceSysSuspnd), ssNormal)
	wantState(t, env, sched.StateSUSP, sched.ResourceNone)
	wantR0(t, callLNM(t, env, serviceSysResume), ssNormal)
	wantState(t, env, sched.StateCOM, sched.ResourceNone)
}

// TestCross_deleteSuspended: $DELPRC of a suspended process resumes it,
// so it can run its deletion.
func TestCross_deleteSuspended(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	child := newSubprocess(t, parent)

	wantR0(t, byPID(t, parent, serviceSysSuspnd, a, child), ssNormal)
	wantR0(t, delprc(t, parent, a, child.Process.PID), ssNormal)

	if child.Suspended() || !child.deletePending {
		t.Errorf("suspended %v, marked %v; want resumed and marked", child.Suspended(), child.deletePending)
	}

	wantState(t, child, sched.StateCOM, sched.ResourceNone)

	parent.DeleteProcess(child)

	if !child.Deleted {
		t.Error("the child was not deleted")
	}
}
