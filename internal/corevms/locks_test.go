package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/lck"
	"github.com/tucats/govax/internal/vax"
)

// lockEvents is a lck.Notifier that records the events its lock gets.
type lockEvents []lck.EventKind

func (e *lockEvents) Notify(ev lck.Event) { *e = append(*e, ev.Kind) }

// enqueueFor takes a lock on name for env's process, at access mode
// mode, recording its events in got.
func enqueueFor(t *testing.T, env *Environment, name string, lm lck.Mode, mode vax.AccessMode, got *lockEvents) *lck.Lock {
	t.Helper()

	l, events, err := env.Locks.Enqueue(lck.Request{
		Owner: lck.Owner(env.Process.PID), Mode: lm, Name: name,
		Group: 1, AccessMode: uint8(mode), Data: got,
	})
	if err != nil {
		t.Fatal(err)
	}

	lck.Deliver(events)

	return l
}

// TestLocks_processRundown: deleting a process releases its locks, in
// every access mode, and a lock another process waited for is granted
// (its owner told).
func TestLocks_processRundown(t *testing.T) {
	env, _ := fixture()
	a := newProcess(t, env)
	b := newProcess(t, env)

	var aEvents, bEvents lockEvents

	enqueueFor(t, a, "RES", lck.EX, vax.Executive, &aEvents)
	waiter := enqueueFor(t, b, "RES", lck.PR, vax.Executive, &bEvents)

	if waiter.State != lck.Waiting {
		t.Fatalf("the second lock is %s", waiter.State)
	}

	a.processRundown()

	if waiter.State != lck.Granted || len(bEvents) != 1 || bEvents[0] != lck.EventGranted {
		t.Errorf("waiter %s, events %v", waiter.State, bEvents)
	}

	if n := len(env.Locks.Locks(lck.Owner(a.Process.PID))); n != 0 {
		t.Errorf("the deleted process has %d locks", n)
	}
}

// TestLocks_imageRundown: an image's end takes its user-mode locks, and
// leaves those of inner modes.
func TestLocks_imageRundown(t *testing.T) {
	env, _ := fixture()
	a := newProcess(t, env)

	var events lockEvents

	user := enqueueFor(t, a, "USER", lck.EX, vax.User, &events)
	exec := enqueueFor(t, a, "EXEC", lck.EX, vax.Executive, &events)

	a.dequeueUserLocks()

	if _, ok := env.Locks.Lock(user.ID); ok {
		t.Error("the user-mode lock survived image rundown")
	}

	if _, ok := env.Locks.Lock(exec.ID); !ok {
		t.Error("the executive-mode lock went at image rundown")
	}
}

// TestLocks_waitDeadline: a record lock wait with a time limit
// (RAB$V_TMO) ends when the clock reaches it; idling moves time to it as
// to a timer; it is no timer request ($GETJPI's count), and expiring it
// clears it.
func TestLocks_waitDeadline(t *testing.T) {
	env, _ := fixture()
	now := fakeClock(env)
	deadline := *now + 5*1000*ms

	if err := env.awaitLock(func() bool { return false }, deadline); err != ErrWait {
		t.Fatalf("awaitLock: %v, want ErrWait", err)
	}

	over := env.pendingWait.over
	env.pendingWait = nil

	if over() {
		t.Error("the wait is over before its deadline")
	}

	if next, ok := env.nextTimer(); !ok || next != deadline {
		t.Errorf("nextTimer = %d, %v; want the deadline %d", next, ok, deadline)
	}

	if n := env.PendingTimers(); n != 0 {
		t.Errorf("%d timer requests, want 0", n)
	}

	*now = deadline
	env.expireTimers()

	if !over() {
		t.Error("the wait isn't over at its deadline")
	}

	if _, ok := env.nextTimer(); ok {
		t.Error("the deadline is still due after expiring")
	}
}
