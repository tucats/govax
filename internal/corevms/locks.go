package corevms

import (
	"github.com/tucats/govax/internal/lck"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
)

// The lock manager's rundowns (Phase 47). The lock database itself is
// System.Locks (internal/lck); $ENQ and $DEQ, and RMS's file and record
// locks, put their locks in it.

// dequeueUserLocks is image rundown's part: the image's user-mode locks
// go ($DEQ with LCK$M_DEQALL at user mode), and whatever waited for them
// is granted. Locks taken in an inner mode, as RMS takes its own, stay
// with the process.
func (env *Environment) dequeueUserLocks() {
	if env.Locks == nil {
		return
	}

	events, _ := env.Locks.DequeueAll(lck.Owner(env.Process.PID), uint8(vax.User), 0)
	lck.Deliver(events)
}

// releaseLocks is process deletion's: every lock the process has goes.
// A lock still held at PW or EX now (the program never dequeued it)
// leaves its resource's value block invalid, as a process "terminated
// abnormally" does (System Services manual, $ENQ). That a process ended
// by $EXIT counts as abnormal here is unconfirmed: VMS's rundown may
// tell the two apart.
func (env *Environment) releaseLocks() {
	if env.Locks == nil {
		return
	}

	lck.Deliver(env.Locks.Release(lck.Owner(env.Process.PID), true))
}

// awaitLock makes the process wait, in LEF as for a lock's event flag,
// until over reports true: an RMS stream waiting for a record lock
// (RAB$V_WAT). A deadline that isn't 0 ends the wait then too
// (RAB$V_TMO): the service, called again, fails with RMS$_TMO.
func (env *Environment) awaitLock(over func() bool, deadline uint64) error {
	env.waitDeadline = deadline

	return env.waitOn(sched.StateLEF, sched.ResourceNone, resourceBoost, func() bool {
		return over() || deadline != 0 && env.Clock() >= deadline
	})
}

// lockWaker is the lck.Notifier of a lock a process waits for: its grant
// ends the wait at once (Phase 46's reportEvent), rather than at the
// scheduler's next look.
type lockWaker struct{ env *Environment }

func (w lockWaker) Notify(ev lck.Event) {
	if ev.Kind == lck.EventGranted || ev.Kind == lck.EventDeadlock {
		w.env.reportEvent(resourceBoost)
	}
}

// checkDeadlocks runs the lock manager's deadlock search for every
// request that has waited DEADLOCK_WAIT (internal/lck's deadlock.go),
// telling the refused requests' owners: the scheduler's step before each
// choice (pollEvents), and idling moves time to the next due search
// (nextTimer), since in a deadlock every process in it waits.
func (sys *System) checkDeadlocks() {
	if due, ok := sys.Locks.NextDeadlockCheck(); ok && due <= sys.Clock() {
		lck.Deliver(sys.Locks.CheckDeadlocks(sys.Clock()))
	}
}
