package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// quantumEngine returns a kernel-mode Engine in quantum (deterministic)
// clock mode, one instruction per emulated millisecond, so tests can move
// its clock forward exactly with tickQuantum.
func quantumEngine() *Engine {
	e := kernelEngine()
	e.hardwareClock = false
	e.SetQuantum(1)

	return e
}

// TestTODRFollowsSystemTime: TODR counts 10 ms units of the emulated system
// time, so a second of emulated time (1,000 ticks at one millisecond each)
// moves it on by exactly 100, with nothing having to update it.
func TestTODRFollowsSystemTime(t *testing.T) {
	e := quantumEngine()
	start := e.TODR()

	for i := 0; i < 1000; i++ {
		e.tickQuantum()
	}

	if got := e.TODR() - start; got != 100 {
		t.Errorf("TODR advanced %d over 1 s of emulated time, want 100", got)
	}
}

// TestTODRWithinYear: unwritten, TODR is the 10 ms units since January 1st,
// so it can never exceed a leap year's worth.
func TestTODRWithinYear(t *testing.T) {
	e := quantumEngine()

	const leapYear = 366 * 24 * 60 * 60 * 100 // in 10 ms units
	if got := e.TODR(); got >= leapYear {
		t.Errorf("TODR = %d, want less than a year (%d)", got, leapYear)
	}
}

// TestMtprMfprTODR: MTPR sets TODR, which then goes on counting from the
// value written; MFPR reads the count. The register file's own TODR slot
// isn't used.
func TestMtprMfprTODR(t *testing.T) {
	e := quantumEngine()

	e.cpu.SetGPR(vax.R1, 12345)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.TODR))...)

	// Step counts an instruction's tick before running it, so the MFPR's
	// own Step adds one more tick before it reads: 49 here make 50 ms in
	// all since the write.
	for i := 0; i < 49; i++ {
		e.tickQuantum()
	}

	stepInstruction(t, e, mfprBytes(uint32(vax.TODR), vax.R2)...)

	if got := e.cpu.GPR(vax.R2); got != 12350 {
		t.Errorf("MFPR TODR = %d, want 12350 (12345 written, then 50 ms)", got)
	}
}

// TestReadPR: ReadPR is the register file for an ordinary register, and the
// computed value for TODR.
func TestReadPR(t *testing.T) {
	e := quantumEngine()
	e.cpu.SetPR(vax.P0BR, 0x80001000)
	e.SetTODR(777)

	if got := e.ReadPR(vax.P0BR); got != 0x80001000 {
		t.Errorf("ReadPR(P0BR) = %#x, want 0x80001000", got)
	}

	if got := e.ReadPR(vax.TODR); got != 777 {
		t.Errorf("ReadPR(TODR) = %d, want 777", got)
	}
}

// TestPollHostClockIdleWhenStopped: with the interval clock stopped and no
// interrupt queued, hardware-clock mode doesn't even read the host clock.
func TestPollHostClockIdleWhenStopped(t *testing.T) {
	e := kernelEngine()
	e.hardwareClock = true
	e.lastClock = 0

	e.pollHostClock()

	if e.lastClock != 0 {
		t.Errorf("lastClock = %d, want 0 (host clock not read while the interval clock is stopped)", e.lastClock)
	}
}

// TestPollHostClockTicksWhenRunning: once a program starts the interval
// clock, a poll that finds a new host millisecond takes one tick (ICR
// counts up by one).
func TestPollHostClockTicksWhenRunning(t *testing.T) {
	e := kernelEngine()
	e.hardwareClock = true
	e.lastClock = 0
	e.cpu.SetPR(vax.NICR, 0xFFFFFF00) // a long count, so no interrupt yet
	e.cpu.SetPR(vax.ICR, 0xFFFFFF00)
	e.cpu.SetPR(vax.ICCS, iccsRun)

	e.pollHostClock()

	if e.lastClock == 0 {
		t.Fatal("lastClock = 0, want the host clock read")
	}

	if got := e.cpu.PR(vax.ICR); got != 0xFFFFFF01 {
		t.Errorf("ICR = %#x, want 0xFFFFFF01 (one tick)", got)
	}
}

// TestHardwareClockPollsEveryInterval: Step looks at the host clock only on
// instructions whose count is a multiple of hostClockPollInterval.
func TestHardwareClockPollsEveryInterval(t *testing.T) {
	e := kernelEngine()
	e.hardwareClock = true
	e.cpu.SetPR(vax.NICR, 0xFFFFFF00)
	e.cpu.SetPR(vax.ICR, 0xFFFFFF00)
	e.cpu.SetPR(vax.ICCS, iccsRun)

	e.lastClock = 0
	e.instrCount = 1 // Step makes it 2: no poll
	stepInstruction(t, e, 0x01) // NOP

	if e.lastClock != 0 {
		t.Errorf("lastClock = %d after instruction 2, want 0 (no poll)", e.lastClock)
	}

	e.instrCount = hostClockPollInterval - 1 // Step makes it a multiple
	stepInstruction(t, e, 0x01)

	if e.lastClock == 0 {
		t.Error("lastClock = 0 after instruction 1024, want the host clock polled")
	}
}
