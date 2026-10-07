package corevms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
)

// withScheduler installs env's System as the scheduling hook of an
// engine on its CPU and memory, as the console does with
// vax.process.scheduler on.
func withScheduler(env *Environment) {
	env.InstallScheduler(cpu.NewEngine(env.cpu, env.mem))
}

// callWaiting calls a service as SystemService would, reporting a wait
// to the scheduler, and returns its error.
func callWaiting(env *Environment, fn ServiceFunc, argv ...uint32) error {
	_, err := fn(env, argv)
	env.enterWait(errors.Is(err, ErrWait))

	return err
}

// wantState checks env's scheduling state, and its resource.
func wantState(t *testing.T, env *Environment, state sched.State, resource sched.Resource) {
	t.Helper()

	info, _ := env.Scheduler().Info(handle(env))
	if info.State != state || info.Resource != resource {
		t.Errorf("state %s (%s), want %s (%s)", info.State, info.Resource, state, resource)
	}
}

// TestWaits_hibernate: $HIBER waits in HIB until a wakeup is pending.
func TestWaits_hibernate(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	if err := callWaiting(env, serviceSysHiber); !errors.Is(err, ErrWait) {
		t.Fatalf("err %v, want ErrWait", err)
	}

	wantState(t, env, sched.StateHIB, sched.ResourceNone)

	env.wakeWaiters()
	wantState(t, env, sched.StateHIB, sched.ResourceNone)

	env.Process.WakePending = true
	env.wakeWaiters()
	wantState(t, env, sched.StateCOM, sched.ResourceNone)

	if env.waiters != 0 || env.waiting != nil {
		t.Errorf("%d waiters, waiting %v after the wait ended", env.waiters, env.waiting)
	}

	// Ended by a wakeup: boosted by 3 (internal/sched, Table 10-3).
	if info, _ := env.Scheduler().Info(handle(env)); info.Priority != info.Base+3 {
		t.Errorf("priority %d, base %d: want a boost of 3", info.Priority, info.Base)
	}
}

// TestWaits_eventFlags: $WAITFR on a local flag waits in LEF until the
// flag is set; a common cluster's flag is a CEF wait.
func TestWaits_eventFlags(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	if err := callWaiting(env, serviceSysWaitfr, 3); !errors.Is(err, ErrWait) {
		t.Fatalf("err %v, want ErrWait", err)
	}

	wantState(t, env, sched.StateLEF, sched.ResourceNone)

	env.wakeWaiters()
	wantState(t, env, sched.StateLEF, sched.ResourceNone)

	env.Process.LocalEventFlags[0] |= 1 << 3
	env.wakeWaiters()
	wantState(t, env, sched.StateCOM, sched.ResourceNone)

	_ = env.waitOnFlag(70, nil)

	if w := env.pendingWait; w == nil || w.state != sched.StateCEF || w.class != eventFlagBoost {
		t.Errorf("a common flag's wait: %+v", w)
	}

	env.pendingWait = nil
}

// TestWaits_mailbox: a write to a full mailbox waits in RWMBX until a
// read makes room, and a $QIOW read on an empty mailbox in LEF until a
// write completes it.
func TestWaits_mailbox(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	a := newArena(t, env)
	_, ch := crembx(t, env, a, 0, 16, 16, "")
	buf := a.alloc(16)

	wantR0(t, mbxQIO(t, env, 0, ch, fnWriteNow, 0, 0, a.str("0123456789ABCDEF"), 16), ssNormal)

	if err := callWaiting(env, serviceSysQio, 0, ch, fnWriteNow, 0, 0, 0, a.str("x"), 1); !errors.Is(err, ErrWait) {
		t.Fatalf("write to a full mailbox: err %v, want ErrWait", err)
	}

	wantState(t, env, sched.StateMWAIT, sched.ResourceMailbox)

	env.wakeWaiters()
	wantState(t, env, sched.StateMWAIT, sched.ResourceMailbox)

	wantR0(t, mbxQIO(t, env, 0, ch, fnReadVBlk, 0, 0, buf, 16), ssNormal) // room now
	env.wakeWaiters()
	wantState(t, env, sched.StateCOM, sched.ResourceNone)

	// A $QIOW read of the now empty mailbox.
	env.cpu.SetGPR(vax.FP, 0x7000)

	if err := callWaiting(env, serviceSysQiow, 0, ch, fnReadVBlk, 0, 0, 0, buf, 16); !errors.Is(err, ErrWait) {
		t.Fatalf("read of an empty mailbox: err %v, want ErrWait", err)
	}

	wantState(t, env, sched.StateLEF, sched.ResourceNone)

	env.cpu.SetGPR(vax.FP, 0x6000)
	wantR0(t, callLNM(t, env, serviceSysQiow, 0, ch, fnWriteNow, 0, 0, 0, a.str("hi"), 2), ssNormal)

	env.wakeWaiters()
	wantState(t, env, sched.StateCOM, sched.ResourceNone)
}

// TestWaits_withoutScheduler: with no scheduler installed, a waiting
// service changes nothing in the scheduler: the process spins on its
// XFC as before.
func TestWaits_withoutScheduler(t *testing.T) {
	env, _ := fixture()

	if err := callWaiting(env, serviceSysHiber); !errors.Is(err, ErrWait) {
		t.Fatalf("err %v, want ErrWait", err)
	}

	wantState(t, env, sched.StateCOM, sched.ResourceNone)

	if env.waiting != nil || env.pendingWait != nil || env.waiters != 0 {
		t.Error("a wait was recorded without a scheduler")
	}
}

// TestWaits_unspecified: a service that waits without saying why waits
// in LEF for nothing in particular, and is computable again at the next
// choice; retryWaiters makes every waiter computable.
func TestWaits_unspecified(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	env.enterWait(true)
	wantState(t, env, sched.StateLEF, sched.ResourceNone)

	env.wakeWaiters()
	wantState(t, env, sched.StateCOM, sched.ResourceNone)

	_ = callWaiting(env, serviceSysHiber)
	env.retryWaiters()
	wantState(t, env, sched.StateCOM, sched.ResourceNone)

	if info, _ := env.Scheduler().Info(handle(env)); info.Priority != info.Base {
		t.Errorf("a retry boosted the priority to %d", info.Priority)
	}
}

// TestWaits_wakeAnotherProcess: $WAKE with another process's PID sets
// that process's wakeup, not the caller's.
func TestWaits_wakeAnotherProcess(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	other := newProcess(t, env)
	a := newArena(t, env)
	pidadr := a.alloc(4)

	if err := env.mem.StoreLongword(env.cpu, pidadr, other.Process.PID); err != nil {
		t.Fatal(err)
	}

	wantR0(t, callLNM(t, env, serviceSysWake, pidadr), ssNormal)

	if !other.Process.WakePending || env.Process.WakePending {
		t.Errorf("wake pending: caller %v, other %v; want only the other", env.Process.WakePending, other.Process.WakePending)
	}
}

// TestSetpri_scheduler: $SETPRI changes the scheduler's base and current
// priority, and $GETJPI's JPI$_PRI reports the scheduler's current
// priority, boost included.
func TestSetpri_scheduler(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	wantR0(t, callLNM(t, env, serviceSysSetpri, 0, 0, 6), ssNormal)

	if info, _ := env.Scheduler().Info(handle(env)); info.Base != 6 || info.Priority != 6 {
		t.Errorf("scheduler priorities %+v, want 6", info)
	}

	_ = callWaiting(env, serviceSysHiber)
	env.Process.WakePending = true
	env.wakeWaiters()

	if got := env.currentPriority(); got != 9 {
		t.Errorf("JPI$_PRI after a wakeup = %d, want 9 (6 + 3)", got)
	}

	if env.Process.BasePriority != 6 {
		t.Errorf("JPI$_PRIB = %d, want 6", env.Process.BasePriority)
	}
}
