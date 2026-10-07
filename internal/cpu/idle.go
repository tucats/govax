package cpu

import (
	"time"

	"github.com/tucats/govax/internal/vax"
)

// Idling, for the scheduler (docs/PHASE-44.md, subtask 5).
//
// When every process waits, VMS runs its "null process", a loop that
// does nothing until an interrupt (a clock tick ending a timer, an I/O
// completion) makes some process computable. govax doesn't run
// instructions for it. The scheduler instead asks the engine to move
// time on to when the next timer is due:
//
//   - In quantum-clock mode, where emulated time is counted from
//     instructions, the clock jumps there at once. A run stays
//     deterministic: the jump depends only on the timers' times.
//   - In hardware-clock mode, where emulated time is the host's, the
//     engine sleeps until then.

// idleSleepSlice is the longest single sleep while idling in
// hardware-clock mode, so a CTRL/C typed meanwhile is noticed promptly.
const idleSleepSlice = 10 * time.Millisecond

// IdleUntil moves emulated time on to t (a VMS system time), for a
// scheduler with no process to run, and reports whether time has reached
// t. It returns false, having moved time on less or not at all, when:
//
//   - the interval clock is running or interrupts are queued (quantum
//     mode): their ticks can't be skipped;
//   - getting to t would take longer than limit (hardware mode): it
//     sleeps for limit and stops, so its caller can go back to running
//     instructions, where limits and CTRL/C are checked;
//   - a control key is typed while it sleeps.
func (e *Engine) IdleUntil(t uint64, limit time.Duration) bool {
	now := e.SystemTime()
	if t <= now {
		return true
	}

	if !e.hardwareClock {
		if e.cpu.PR(vax.ICCS)&iccsRun != 0 || len(e.iqueue) > 0 {
			return false
		}

		// Whole milliseconds, rounding up, so the time reached isn't
		// short of t; the count toward the next millisecond starts over.
		ms := (t - now + vmsTicksPerMillisecond - 1) / vmsTicksPerMillisecond
		e.clockTicks += ms
		e.quantumCurrent = e.quantumInitial

		return true
	}

	wait := time.Duration(t-now) * 100 // VMS time is in 100ns units
	reached := wait <= limit

	wait = min(wait, limit)
	for wait > 0 && !e.AttentionRequested() {
		slice := min(wait, idleSleepSlice)
		time.Sleep(slice)
		wait -= slice
	}

	return reached && !e.AttentionRequested()
}

// InstructionsUntil returns how many instructions run before emulated
// time reaches t, in quantum-clock mode (at least 1). In hardware-clock
// mode time isn't counted in instructions, so it returns
// hostClockTimerPoll, a count short enough that a timer is noticed
// within a fraction of a millisecond at govax's speed.
func (e *Engine) InstructionsUntil(t uint64) int {
	if e.hardwareClock {
		return hostClockTimerPoll
	}

	now := e.SystemTime()
	if t <= now || e.quantumInitial <= 0 {
		return 1
	}

	// The next millisecond begins after quantumCurrent more
	// instructions, and each after that quantumInitial later.
	ms := (t - now + vmsTicksPerMillisecond - 1) / vmsTicksPerMillisecond
	n := uint64(e.quantumCurrent) + (ms-1)*uint64(e.quantumInitial)

	return int(min(n, maxInstructionsUntil))
}

// hostClockTimerPoll is InstructionsUntil's answer in hardware-clock mode.
const hostClockTimerPoll = 1 << 14

// maxInstructionsUntil caps InstructionsUntil's answer, for a timer far
// in the future.
const maxInstructionsUntil = 1 << 30
