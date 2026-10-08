package corevms

import (
	"fmt"
	"math"
	"time"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
)

// Idling (docs/PHASE-44.md, subtask 5). When no process is computable,
// VMS runs its "null process" until an interrupt makes one computable.
// The events that can do that in govax with no instruction running are
// timers ($SETIMR, $SCHDWK), so the scheduler moves emulated time on to
// the next one due (cpu.Engine.IdleUntil: a jump in quantum-clock mode, a
// sleep in hardware-clock mode), expires it, and tests the waiters again,
// until one can run.
//
// With no timer pending, nothing but CTRL/C (or another key's AST) can
// end a wait. Then every waiter is made to try its wait again
// (retryWaiters): each spins on its service's XFC, as a lone waiting
// process always has, and the engine checks for CTRL/C and the run's
// instruction and time limits between tries.

// maxIdleJumps bounds the timers one idle call moves time past before
// going back to running instructions. Timers that end no wait (a
// repeating $SCHDWK for a process that isn't hibernating) could
// otherwise keep it jumping forever, never checking for CTRL/C.
const maxIdleJumps = 64

// idleWaitLimit is the longest one idle call sleeps in hardware-clock
// mode before going back to running instructions.
const idleWaitLimit = 50 * time.Millisecond

// nextTimer returns when the next timer of any process is due, and false
// if no process has one.
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
	}

	return next, found
}

// idle runs when the scheduler finds no computable process: it moves
// time on, timer by timer, until a process can run, and returns it (see
// this file's opening comment). If no timer brings one, every waiter
// retries, and the first is returned.
func (sys *System) idle(e *cpu.Engine) (sched.Handle, bool) {
	trace := sys.cpu.DebugEnabled(vax.DebugProcess)

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

	sys.retryWaiters()

	return sys.sched.Reschedule()
}
