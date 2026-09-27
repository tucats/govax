package cpu

import "testing"

// TestSystemTimeQuantum: in quantum mode, each quantum of instructions is
// one emulated millisecond, whatever the host clock does.
func TestSystemTimeQuantum(t *testing.T) {
	e := newEngine()
	e.hardwareClock = false
	e.SetQuantum(20)

	start := e.SystemTime()

	for i := 0; i < 20*5; i++ {
		e.tickQuantum()
	}

	if got := e.SystemTime() - start; got != 5*vmsTicksPerMillisecond {
		t.Errorf("SystemTime advanced %d after 100 instructions, want 5ms (%d)", got, 5*vmsTicksPerMillisecond)
	}
}
