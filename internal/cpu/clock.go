package cpu

import (
	"time"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// The engine's clocks, kept "synthetically" (docs/PERFORMANCE.md, Study 1,
// R1).
//
// A real VAX has two clocks a program can see:
//
//   - The interval clock: three privileged registers (ICCS, the control
//     and status register; NICR, the reload value; ICR, the running count)
//     that interrupt the CPU at a fixed rate once software starts them. VMS
//     uses that interrupt to keep time and to schedule processes.
//   - The time-of-year register (TODR): a battery-backed counter of 10 ms
//     units, which VMS reads at boot to learn the date.
//
// govax inherited from eVAX a microkernel that started the interval clock at
// boot and kept TODR moving from its interrupt handler. That meant an
// interrupt every few instructions in quantum mode, and a read of the host's
// clock before every instruction in hardware-clock mode, though nothing in
// the microkernel needs either any more: its console output goes through
// XFCs, and the RTL's time services read Engine.SystemTime (systime.go).
//
// So the microkernel now leaves the interval clock stopped, and the engine
// keeps time by itself:
//
//   - TODR is worked out when it's read (MFPR, or the console), from the
//     same emulated system time the time services use, so the two always
//     agree. No instruction has to keep it up to date.
//   - In hardware-clock mode (vax.hardware.clock), Step looks at the host
//     clock only every hostClockPollInterval instructions, and only when
//     something needs it: the interval clock is running (a program started
//     it), or an interrupt is waiting in the queue for a tick to admit it.
//     Otherwise the per-instruction cost is one AND and one compare.
//   - In quantum mode, tickQuantum (interrupt.go) still counts instructions
//     into emulated milliseconds, which is what keeps that mode's timers
//     deterministic; with the interval clock stopped each tick is only a
//     couple of tests.

// hostClockPollInterval is how many instructions run between looks at the
// host clock in hardware-clock mode. It must be a power of two, so the test
// in Step is a mask instead of a division. At the tens of millions of
// instructions per second govax runs, 1024 instructions is well under the
// interval clock's 1 ms tick, so ticks are still taken on time.
const hostClockPollInterval = 1024

// hostClockPollMask turns "instruction count is a multiple of
// hostClockPollInterval" into a single AND.
const hostClockPollMask = hostClockPollInterval - 1

// todrUnit is the length of one TODR count: 10 milliseconds.
const todrUnit = 10 * time.Millisecond

// pollHostClock is hardware-clock mode's clock step, run by Step every
// hostClockPollInterval instructions. If the interval clock is running, or
// an interrupt is queued waiting for a tick, and the host clock has moved
// on to a new millisecond since the last look, it takes one interval-clock
// tick and gives queued interrupts a chance to be admitted, as tickQuantum
// does in quantum mode. Ticks missed between looks aren't made up: one tick
// per look at most, as before this was batched.
func (e *Engine) pollHostClock() {
	if e.cpu.PR(vax.ICCS)&iccsRun == 0 && len(e.iqueue) == 0 {
		return
	}

	now := uint64(time.Now().UnixMilli())
	if now == e.lastClock {
		return
	}

	e.lastClock = now
	e.tickIntervalClock()

	if !e.interruptPending {
		e.scanInterruptQueue()
	}
}

// todrClock is TODR's value without any adjustment a program or the console
// has made by writing it: the number of 10 ms units since midnight on
// January 1st of the current year, by the emulated system time. That is the
// convention SHOW CLOCK reads it by.
//
// SystemTime is local time in VMS's format, and vmsdef.GoTime turns it into
// a Go time.Time whose fields (year, day, hour, ...) are that local reading,
// labeled UTC. January 1st is built in the same UTC-labeled frame, so the
// subtraction counts local time without any time-zone arithmetic.
func (e *Engine) todrClock() uint32 {
	now := vmsdef.GoTime(e.SystemTime())
	jan1 := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)

	return uint32(now.Sub(jan1) / todrUnit)
}

// TODR returns the time-of-year register's current value: the clock's
// reading (todrClock) plus whatever offset the last write to TODR set up.
// The arithmetic is modulo 2^32, as a 32-bit hardware counter's is.
func (e *Engine) TODR() uint32 {
	return e.todrClock() + e.todrOffset
}

// SetTODR sets the time-of-year register to v, as MTPR to TODR (or the
// console's SET TODR=) does. The register goes on counting from v: rather
// than store v, this remembers how far v is from the clock's own reading,
// and TODR adds that back on every read.
func (e *Engine) SetTODR(v uint32) {
	e.todrOffset = v - e.todrClock()
}

// ReadPR returns privileged register reg's value as an MFPR instruction
// would see it, for callers outside this package (the console's SHOW
// commands). Most registers are simply stored in the CPU's register file
// and read from there; TODR is computed when it's read (see TODR).
func (e *Engine) ReadPR(reg vax.PrivReg) uint32 {
	if reg == vax.TODR {
		return e.TODR()
	}

	return e.cpu.PR(reg)
}
