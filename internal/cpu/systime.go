package cpu

import (
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// The emulated system time (docs/PHASE-26.md subtask 11): the clock the
// RTL's time services ($GETTIM, $SETIMR, ...) read, in VMS's 64-bit
// format — 100ns units since 17-Nov-1858 00:00, local time, as VMS keeps
// it (vmsdef.Time).
//
// With vax.hardware.clock set, the system time simply is the host's local
// time. Without it (the default), the engine counts instructions into
// emulated milliseconds (tickQuantum, one millisecond per quantum), and the
// system time is the local time the Engine was created at plus one
// millisecond per tick — deterministic, so a program's timers expire after
// the same number of instructions every run. Either way it doesn't depend
// on the guest's interval-clock interrupt, which the microkernel no longer
// starts (clock.go). The time-of-year register, TODR, is computed from
// this same time when it's read, so the two always agree.

// vmsTicksPerMillisecond is one millisecond in VMS time units.
const vmsTicksPerMillisecond = 10_000

// SystemTime returns the current emulated system time in VMS format.
func (e *Engine) SystemTime() uint64 {
	if e.hardwareClock {
		return vmsdef.Time(time.Now())
	}

	return e.bootTime + e.clockTicks*vmsTicksPerMillisecond
}
