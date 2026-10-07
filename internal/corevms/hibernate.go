package corevms

import (
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Hibernation and wakeups (docs/PHASE-26.md subtask 14): $HIBER, $WAKE,
// $SCHDWK, and $CANWAK.
//
// Hibernation is VMS's simplest way for a process to sleep until someone
// wants it. The whole mechanism is one flag per process, "wake pending"
// (Process.WakePending):
//
//   - $WAKE sets it, immediately.
//   - $SCHDWK sets it later: it queues a wakeup on the timer queue
//     (timers.go), optionally repeating at a fixed interval.
//   - $HIBER waits until it is set, then clears it and returns. If it was
//     already set, $HIBER returns at once without sleeping.
//
// Because it's a flag and not a counter, two $WAKEs before a $HIBER end
// only that one $HIBER; the next one sleeps.
//
// On VMS, $WAKE and $SCHDWK usually come from *another* process (a
// server waking a client), or from an AST routine in the same process,
// interrupting its own $HIBER. $WAKE reaches any process in the process
// table (docs/PHASE-44.md, subtask 4); $SCHDWK and $CANWAK still act
// only on the caller, until Phase 45 makes timers reach other processes.
// With the scheduler, a hibernating process is in the HIB state, giving
// the CPU to others until its wakeup is pending (waits.go).

// serviceSysHiber is SYS$HIBER: waits until a wakeup is pending, then
// consumes it. It takes no arguments and always returns SS$_NORMAL.
//
// Waiting uses the event-flag waits' mechanism: while no wakeup is
// pending the service returns ErrWait, and its XFC runs again later (at
// the next instruction, or, with the scheduler, once a wakeup is
// pending), interrupts and ASTs being delivered in between. Each retry
// first expires due timers, so a $SCHDWK wakeup ends the wait on the
// first step after its time.
func serviceSysHiber(env *Environment, _ []uint32) (uint32, error) {
	env.expireTimers()

	if !env.Process.WakePending {
		p := env.Process

		return 0, env.waitOn(sched.StateHIB, sched.ResourceNone, hibernateBoost, func() bool { return p.WakePending })
	}

	env.Process.WakePending = false

	return ssNormal, nil
}

// serviceSysWake is SYS$WAKE:
//
//	SYS$WAKE [pidadr] ,[prcnam]
//
// It sets the wakeup flag of the process pidadr/prcnam name (the caller,
// with neither), ending its $HIBER now or its next one. See
// processTarget for how the process is picked and its error statuses.
// The privileges VMS requires to wake another process always pass.
// Another process hibernating is made computable at the scheduler's next
// choice, which is asked for now, so a woken process of higher priority
// preempts the caller at once.
func serviceSysWake(env *Environment, argv []uint32) (uint32, error) {
	target, st := env.processTarget(optArg(argv, 0), optArg(argv, 1), false)
	if st != 0 {
		return st, nil
	}

	target.Process.WakePending = true

	if target != env {
		env.requestReschedule()
	}

	return ssNormal, nil
}

// minWakeRepeat is the shortest $SCHDWK repeat interval, 10ms; a shorter
// reptim is raised to it.
const minWakeRepeat = vmsdef.TicksPerSecond / 100 // 10ms

// serviceSysSchdwk is SYS$SCHDWK:
//
//	SYS$SCHDWK [pidadr] ,[prcnam] ,daytim ,[reptim]
//
// It schedules a wakeup for the process pidadr/prcnam name (see
// processTarget) at daytim: a quadword that is an absolute time, or,
// if negative, a delta from now — as for $SETIMR. An absolute time
// already past wakes at the next check.
//
// reptim, if given and nonzero, is a delta time: the wakeup then repeats
// at that interval (10ms at least) until $CANWAK or image exit cancels
// it. A positive reptim is SS$_IVTIME, as is a repeating absolute daytim
// whose first repetition would already be in the past. daytim of 0, or
// either time unreadable, is SS$_ACCVIO.
//
// Not implemented: the ASTLM quota (SS$_EXQUOTA); the timer queue
// entry comes from Go's heap, so SS$_INSFMEM can't happen.
func serviceSysSchdwk(env *Environment, argv []uint32) (uint32, error) {
	daytim, reptim := optArg(argv, 2), optArg(argv, 3)

	if st := env.callerTarget(optArg(argv, 0), optArg(argv, 1), false); st != 0 {
		return st, nil
	}

	if daytim == 0 { // page 0 is never accessible on VMS
		return ssAccVio, nil
	}

	expiry, ok := env.loadQuad(daytim)
	if !ok {
		return ssAccVio, nil
	}

	now := env.Clock()
	absolute := int64(expiry) >= 0

	if !absolute {
		expiry = now + uint64(-int64(expiry))
	}

	var repeat uint64

	if reptim != 0 {
		v, ok := env.loadQuad(reptim)
		if !ok {
			return ssAccVio, nil
		}

		if int64(v) > 0 { // must be a delta time (0 means no repeat)
			return ssIvTime, nil
		}

		if v != 0 {
			repeat = max(uint64(-int64(v)), minWakeRepeat)

			if absolute && expiry+repeat < now {
				return ssIvTime, nil
			}
		}
	}

	env.timers = append(env.timers, &timerRequest{
		expiry: expiry,
		wake:   true,
		repeat: repeat,
	})

	return ssNormal, nil
}

// serviceSysCanwak is SYS$CANWAK:
//
//	SYS$CANWAK [pidadr] ,[prcnam]
//
// It cancels every scheduled ($SCHDWK) wakeup for the process pidadr/
// prcnam name (see processTarget), whoever scheduled it. A wakeup that
// already happened — the flag $WAKE or an expired $SCHDWK set — stays
// pending: $CANWAK only removes queued requests.
func serviceSysCanwak(env *Environment, argv []uint32) (uint32, error) {
	if st := env.callerTarget(optArg(argv, 0), optArg(argv, 1), false); st != 0 {
		return st, nil
	}

	remaining := env.timers[:0]

	for _, t := range env.timers {
		if !t.wake {
			remaining = append(remaining, t)
		}
	}

	env.timers = remaining

	return ssNormal, nil
}

func registerHibernateServices(t *ServiceTable) {
	t.Register("SYS$HIBER", serviceSysHiber)
	t.Register("SYS$WAKE", serviceSysWake)
	t.Register("SYS$SCHDWK", serviceSysSchdwk)
	t.Register("SYS$CANWAK", serviceSysCanwak)
}
