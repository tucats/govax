package cpu

import (
	"time"

	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements govax's own -instruction-limit/-time-limit flags
// (docs/PHASE-15.md's sub-phase 2): an optional, per-run cap on how many
// instructions Engine.Step will execute and how much wall-clock time it will
// spend doing so, so a runaway program under development doesn't hang the
// emulator or the developer's terminal indefinitely. Not a port of any C
// reference mechanism -- eVAX has no equivalent.

// ErrInstructionLimitExceeded is returned by Engine.Step when the
// instruction limit configured via SetLimits has been reached for the
// current run (see BeginRun). No state changes when this is returned --
// the machine is left exactly as it stood after the last instruction that
// did execute.
var ErrInstructionLimitExceeded = vmserrors.New(vmserrors.VAX_INSTLIM)

// ErrTimeLimitExceeded is Engine.Step's time-limit counterpart to
// ErrInstructionLimitExceeded.
var ErrTimeLimitExceeded = vmserrors.New(vmserrors.VAX_TIMELIM)

// SetLimits configures the instruction-count and wall-clock limits applied
// to each top-level run (see BeginRun) -- govax's own -instruction-limit/
// -time-limit flags. Either limit is disabled by passing 0 (the zero value
// for both), matching "infinite" per those flags' own documented default.
func (e *Engine) SetLimits(maxInstructions int, maxDuration time.Duration) {
	e.instrLimit = maxInstructions
	e.timeLimit = maxDuration
}

// BeginRun resets the instruction/time budget, and any pending Ctrl-C
// interrupt (see Attention), for a new top-level run -- called once by
// Console.Execute/Call/Step at the start of their own Engine.Step loop, not
// once per instruction. This is what keeps time spent outside actual
// instruction execution (console output, formatting, sitting at a
// breakpoint waiting for the next command, single-stepping interactively)
// from ever counting against the budget: the clock (when a time limit is
// configured at all) only starts ticking here, and a runaway program in one
// run doesn't consume the budget of an unrelated later one, since each call
// to Execute/Call/Step starts fresh.
//
// A BeginRun call with no time limit configured touches nothing but a
// couple of int/time.Time fields -- no time.Now() call -- matching the
// requirement that leaving both flags unset costs nothing.
//
// Caveat: -time-limit measures real wall-clock time across the Step loop,
// not CPU/engine-only time. Pausing the *Go process itself* in a debugger
// (as opposed to stopping at one of this emulator's own breakpoints, which
// only pauses between top-level runs and so is unaffected -- see the
// "time spent... at a breakpoint" note above) while stopped partway through
// a run lets the deadline elapse in the background; resuming will report
// the time limit exceeded almost immediately, which can look like a false
// positive. Turn -time-limit off (or use -instruction-limit instead, which
// has no such issue) when debugging govax's own Go code with a debugger.
func (e *Engine) BeginRun() {
	e.instrCount = 0
	e.attentionRequested.Store(false)

	if e.timeLimit > 0 {
		e.runDeadline = time.Now().Add(e.timeLimit)
	}
}

// checkLimits is Step's own first action: refuse to start a new instruction
// once either configured budget is exhausted, leaving the machine exactly
// as the last successful instruction left it.
func (e *Engine) checkLimits() error {
	if e.instrLimit > 0 && e.instrCount >= e.instrLimit {
		return ErrInstructionLimitExceeded
	}

	if e.timeLimit > 0 && !time.Now().Before(e.runDeadline) {
		return ErrTimeLimitExceeded
	}
	
	return nil
}
