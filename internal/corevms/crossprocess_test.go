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

// callerTarget is processTarget reduced to a status, with any process
// other than the caller's own a SS$_NONEXPR: what the tests that name
// only their own process check.
func (env *Environment) callerTarget(pidadr, prcnam uint32, wildcard bool) uint32 {
	target, st := env.processTarget(pidadr, prcnam, wildcard)
	if st != 0 {
		return st
	}

	if target != env {
		return ssNonExpr
	}

	return 0
}

// $GETJPI of other processes and wildcard scans (docs/PHASE-45.md,
// subtask 10).

func TestGetjpi_otherProcess(t *testing.T) {
	parent, _ := fixture()
	withScheduler(parent)

	a := newArena(t, parent)
	child := newSubprocess(t, parent)
	child.Process.Name = "KID"

	pidBuf, ownerBuf, masterBuf, stateBuf, nameBuf := a.alloc(4), a.alloc(4), a.alloc(4), a.alloc(4), a.alloc(15)
	list := a.items(
		item{code: jpiCode(t, "JPI$_PID"), buflen: 4, buf: pidBuf},
		item{code: jpiCode(t, "JPI$_OWNER"), buflen: 4, buf: ownerBuf},
		item{code: jpiCode(t, "JPI$_MASTER_PID"), buflen: 4, buf: masterBuf},
		item{code: jpiCode(t, "JPI$_STATE"), buflen: 4, buf: stateBuf},
		item{code: jpiCode(t, "JPI$_PRCNAM"), buflen: 15, buf: nameBuf},
	)

	// By PID, and by name; the child computable.
	wantR0(t, getjpi(t, parent, 0, a.long(child.Process.PID), 0, list, 0), ssNormal)

	if a.readLong(pidBuf) != child.Process.PID || a.readLong(ownerBuf) != parent.Process.PID ||
		a.readLong(masterBuf) != parent.Process.PID || a.readLong(stateBuf) != uint32(sched.StateCOM) {
		t.Errorf("pid %08X owner %08X master %08X state %d", a.readLong(pidBuf), a.readLong(ownerBuf),
			a.readLong(masterBuf), a.readLong(stateBuf))
	}

	// Hibernating: the state follows.
	if err := callWaiting(child, serviceSysHiber); !errors.Is(err, ErrWait) {
		t.Fatal(err)
	}

	wantR0(t, getjpi(t, parent, 0, 0, a.desc("KID"), list, 0), ssNormal)

	if a.readLong(stateBuf) != uint32(sched.StateHIB) || a.readLong(pidBuf) != child.Process.PID {
		t.Errorf("by name: pid %08X state %d", a.readLong(pidBuf), a.readLong(stateBuf))
	}

	// A process that doesn't exist, and one the caller may not see.
	wantR0(t, getjpi(t, parent, 0, a.long(0x999), 0, list, 0), ssNonExpr)

	other := newProcess(t, parent)
	other.Process.UIC = parent.Process.UIC + 0x10000
	parent.Process.CurrentPrivileges &^= privGROUP | privWORLD
	wantR0(t, getjpi(t, parent, 0, a.long(other.Process.PID), 0, list, 0), ssNoPriv)
}

func TestGetjpi_wildcard(t *testing.T) {
	parent, _ := fixture()
	a := newArena(t, parent)
	child := newSubprocess(t, parent)
	other := newProcess(t, parent)
	other.Process.UIC = parent.Process.UIC + 0x10000

	pidBuf := a.alloc(4)
	list := a.items(item{code: jpiCode(t, "JPI$_PID"), buflen: 4, buf: pidBuf})

	scan := func() []uint32 {
		var pids []uint32

		ctx := a.long(0xFFFFFFFF)

		for range 10 {
			r0 := getjpi(t, parent, 0, ctx, 0, list, 0)
			if r0 == ssNoMoreProc {
				return pids
			}

			wantR0(t, r0, ssNormal)
			pids = append(pids, a.readLong(pidBuf))
		}

		t.Fatal("the scan didn't end")

		return nil
	}

	// Everyone, in table order.
	got := scan()
	want := []uint32{parent.Process.PID, child.Process.PID, other.Process.PID}

	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("scan = %08X, want %08X", got, want)
	}

	// Without WORLD or GROUP, the other group's process is skipped.
	parent.Process.CurrentPrivileges &^= privGROUP | privWORLD

	if got = scan(); len(got) != 2 || got[1] != child.Process.PID {
		t.Errorf("scan without privileges = %08X, want two", got)
	}

	// A process deleted during a scan is just not seen.
	parent.DeleteProcess(child)

	if got = scan(); len(got) != 1 {
		t.Errorf("scan after a deletion = %08X, want only the caller", got)
	}
}
