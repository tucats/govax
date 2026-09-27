package cpu

import (
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// The emulated system time (docs/PHASE-26.md subtask 11): the clock the
// RTL's timer services ($SETIMR) read, in VMS's 64-bit format — 100ns
// units since 17-Nov-1858 00:00 UTC.
//
// It is driven by the same thing that drives the interval clock, so the
// two always agree: one interval-clock tick is one millisecond. With
// vax.hardware.clock set, ticks come from the wall clock once a
// millisecond, and the system time simply is the wall-clock time. Without
// it (the default), ticks come every quantum of instructions
// (tickQuantum), and the system time is the wall-clock time the Engine was
// created at plus one millisecond per tick — deterministic, so a program's
// timers expire after the same number of instructions every run.

// vmsTicksPerMillisecond is one millisecond in VMS time units.
const vmsTicksPerMillisecond = 10_000

// SystemTime returns the current emulated system time in VMS format.
func (e *Engine) SystemTime() uint64 {
	if e.hardwareClock {
		return vmsdef.Time(time.Now())
	}

	return e.bootTime + e.clockTicks*vmsTicksPerMillisecond
}
