package corevms

import (
	"fmt"
	"math"
	"time"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Idling (docs/planning/PHASE-44.md, subtask 5). When no process is computable,
// VMS runs its "null process" until an interrupt makes one computable.
// The events that can do that in govax with no instruction running are
// timers ($SETIMR, $SCHDWK), so the scheduler moves emulated time on to
// the next one due (cpu.Engine.IdleUntil: a jump in quantum-clock mode, a
// sleep in hardware-clock mode), expires it, and tests the waiters again,
// until one can run.
//
// With no timer pending, nothing but CTRL/C (or another key's AST) can
// end a wait, unless a process waits for the host's time (DCL's WAIT, or
// process 1 lending the CPU between its commands): then the scheduler
// waits in the host, a while, testing the waits (hostWait). Otherwise, or
// after that while, every waiter is made to try its wait again
// (retryWaiters): each spins on its service's XFC, as a lone waiting
// process always has, and the engine checks for CTRL/C and the run's
// instruction and time limits between tries. A retry runs instructions,
// which in quantum-clock mode are emulated time, so before hostWait a
// subprocess's WAIT beside process 1's moved the clock on by about a
// millisecond a retry: WAIT 00:00:05 took 33 emulated minutes.

// maxIdleJumps bounds the timers one idle call moves time past before
// going back to running instructions. Timers that end no wait (a
// repeating $SCHDWK for a process that isn't hibernating) could
// otherwise keep it jumping forever, never checking for CTRL/C.
const maxIdleJumps = 64

// idleWaitLimit is the longest one idle call sleeps in hardware-clock
// mode, or waits in the host for a host-time wait, before going back to
// running instructions.
const idleWaitLimit = 50 * time.Millisecond

// hostWaitPoll is how often hostWait tests the waits.
const hostWaitPoll = 2 * time.Millisecond

// nextTimer returns when the next timer of any process is due, or a
// wait's time limit (Environment.waitDeadline) runs out, and false if
// no process has either.
func (sys *System) nextTimer() (uint64, bool) {
	next := uint64(math.MaxUint64)
	found := false

	for _, env := range sys.procs.slots {
		if env == nil {
			continue
		}

		for _, t := range env.timers {
			if t.expiry < next {
				next, found = t.expiry, true
			}
		}

		if d := env.waitDeadline; d != 0 && d < next {
			next, found = d, true
		}
	}

	if d, ok := sys.Locks.NextDeadlockCheck(); ok && d < next {
		next, found = d, true
	}

	return next, found
}

// idle runs when the scheduler finds no computable process: it moves
// time on, timer by timer, until a process can run, and returns it (see
// this file's opening comment). If no timer brings one, every waiter
// retries, and the first is returned.
func (sys *System) idle(e *cpu.Engine) (sched.Handle, bool) {
	trace := sys.cpu.DebugEnabled(registers.DebugProcess)

	// In quantum-clock mode a wait for the host's time is waited for in
	// the host, the clock following the host's up to the next timer, not
	// jumping to it: a timer comes due about when it would with the host's
	// clock, after a WAIT shorter than it is over.
	if e != nil && !e.HardwareClock() && sys.hostTimedWaiter() {
		if h, ok := sys.hostWait(e); ok {
			return h, true
		}

		sys.retryWaiters()

		return sys.sched.Reschedule()
	}

	for jumps := 0; e != nil && jumps < maxIdleJumps; jumps++ {
		t, ok := sys.nextTimer()
		if !ok {
			break
		}

		if trace {
			now := e.SystemTime()
			fmt.Fprintf(sys.cpu.DebugWriter(), "DEBUG(PROCESS): IDLE for %d ms\n", (max(t, now)-now)/10_000)
		}

		sys.accountTime()

		reached := e.IdleUntil(t, idleWaitLimit)
		sys.lastCharge = sys.Clock() // the idle time is nobody's
		
		sys.pollEvents()

		if h, ok := sys.sched.Reschedule(); ok {
			return h, true
		}

		if !reached {
			break
		}
	}

	if _, ok := sys.nextTimer(); !ok && !sys.idleSpinning {
		sys.idleSpinning = true

		if trace {
			fmt.Fprintln(sys.cpu.DebugWriter(), "DEBUG(PROCESS): IDLE: every process waits and no timer is due; waiting for CTRL/C")
		}
	}

	// A process waiting for a line: wait for the terminal, a while, in
	// the host rather than spinning, then look again.
	if _, ok := sys.nextTimer(); !ok && sys.waitForTerminalInput(idleWaitLimit) {
		sys.pollEvents()

		if h, ok := sys.sched.Reschedule(); ok {
			return h, true
		}
	}

	if _, ok := sys.nextTimer(); !ok {
		if h, ok := sys.hostWait(e); ok {
			return h, true
		}
	}

	sys.retryWaiters()

	return sys.sched.Reschedule()
}

// hostWait waits in the host, up to idleWaitLimit, while a process waits
// for the host's time and no other can run, testing the waits every
// hostWaitPoll, and returns the process that can run when one can. The
// time is nobody's. In quantum-clock mode the emulated clock is moved on
// by the host's time, no further than the next timer, as HostIdle moves
// it, and stays where it is with no timer pending, as it does while
// process 1 waits alone. It returns false at once when no process waits
// for the host's time.
func (sys *System) hostWait(e *cpu.Engine) (sched.Handle, bool) {
	if !sys.hostTimedWaiter() {
		return 0, false
	}

	sys.accountTime()
	defer func() { sys.lastCharge = sys.Clock() }()

	start := time.Now()
	from := uint64(0)

	if e != nil {
		from = e.SystemTime()
	}

	for deadline := start.Add(idleWaitLimit); time.Now().Before(deadline); {
		if e != nil && e.AttentionRequested() {
			break
		}

		time.Sleep(hostWaitPoll)

		if t, ok := sys.nextTimer(); ok && e != nil && !e.HardwareClock() {
			elapsed := uint64(time.Since(start) / (time.Second / vmsdef.TicksPerSecond))
			e.IdleUntil(min(t, from+elapsed), 0)
		}

		sys.pollEvents()

		if h, ok := sys.sched.Reschedule(); ok {
			return h, true
		}
	}

	return 0, false
}

// hostTimedWaiter reports whether a process waits for the host's time.
func (sys *System) hostTimedWaiter() bool {
	for _, env := range sys.procs.slots {
		if env != nil && env.waiting != nil && env.waiting.hostTimed && !env.suspended {
			return true
		}
	}

	return false
}
