package cpu

import (
	"testing"
	"time"

	"github.com/tucats/govax/internal/vax"
)

// TestInstructionsUntil: the count InstructionsUntil gives is exactly the
// number of instructions after which the quantum clock reaches the time.
func TestInstructionsUntil(t *testing.T) {
	e := kernelEngine()
	e.hardwareClock = false
	e.SetQuantum(20)

	for range 7 { // part way into a millisecond
		e.tickQuantum()
	}

	target := e.SystemTime() + 3*vmsTicksPerMillisecond - 1 // within the third ms
	n := e.InstructionsUntil(target)

	if n != 13+2*20 {
		t.Fatalf("InstructionsUntil = %d, want %d", n, 13+2*20)
	}

	for range n - 1 {
		e.tickQuantum()
	}

	if e.SystemTime() >= target {
		t.Errorf("reached the time an instruction early")
	}

	e.tickQuantum()

	if e.SystemTime() < target {
		t.Errorf("not at the time after %d instructions", n)
	}

	if got := e.InstructionsUntil(e.SystemTime()); got != 1 {
		t.Errorf("InstructionsUntil(now) = %d, want 1", got)
	}

	e.hardwareClock = true
	if got := e.InstructionsUntil(e.SystemTime() + 1); got != hostClockTimerPoll {
		t.Errorf("hardware clock: %d, want %d", got, hostClockTimerPoll)
	}
}

// TestIdleUntilQuantum: in quantum-clock mode the clock jumps to the
// time, rounded up to a whole millisecond, unless the interval clock is
// running.
func TestIdleUntilQuantum(t *testing.T) {
	e := quantumEngine()
	start := e.SystemTime()
	target := start + 2500*vmsTicksPerMillisecond + 1

	if !e.IdleUntil(target, 0) {
		t.Fatal("IdleUntil didn't reach the time")
	}

	if got := e.SystemTime() - start; got != 2501*vmsTicksPerMillisecond {
		t.Errorf("moved on %d ms, want 2501", got/vmsTicksPerMillisecond)
	}

	if !e.IdleUntil(start, 0) {
		t.Error("a time already past isn't reached")
	}

	e.cpu.SetPR(vax.ICCS, iccsRun)

	now := e.SystemTime()
	if e.IdleUntil(now+vmsTicksPerMillisecond, 0) || e.SystemTime() != now {
		t.Error("jumped with the interval clock running")
	}
}

// TestIdleUntilHardware: in hardware-clock mode the engine sleeps until
// the time, or for the limit if that's sooner, or until a control key.
func TestIdleUntilHardware(t *testing.T) {
	e := kernelEngine()
	e.hardwareClock = true

	begin := time.Now()

	if !e.IdleUntil(e.SystemTime()+20*vmsTicksPerMillisecond, time.Second) {
		t.Error("didn't reach a time 20ms away")
	}

	if d := time.Since(begin); d < 20*time.Millisecond {
		t.Errorf("slept %v, want at least 20ms", d)
	}

	begin = time.Now()
	
	if e.IdleUntil(e.SystemTime()+10_000*vmsTicksPerMillisecond, 15*time.Millisecond) {
		t.Error("reached a time 10s away with a 15ms limit")
	}

	if d := time.Since(begin); d < 15*time.Millisecond || d > 5*time.Second {
		t.Errorf("slept %v, want about 15ms", d)
	}

	e.AttentionKey(AttentionCtrlC)

	begin = time.Now()
	if e.IdleUntil(e.SystemTime()+10_000*vmsTicksPerMillisecond, time.Second) || time.Since(begin) > 500*time.Millisecond {
		t.Error("a pending CTRL/C didn't end the idle")
	}
}
