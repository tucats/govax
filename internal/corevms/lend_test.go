package corevms

import (
	"testing"
	"time"
)

// TestHostIdle: the time the console waits in the host while every
// process waits (docs/PHASE-51, subtask 4) moves the quantum clock on
// by the host's time, but no further than the next timer, and not at
// all with no timer; it is no process's CPU time.
func TestHostIdle(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	// The system time is the engine's, as the console makes it.
	e := env.engine
	env.Clock = e.SystemTime
	sleep := func(d time.Duration) func() { return func() { time.Sleep(d) } }

	env.accountTime()
	cpuTime := env.cpuTime

	// A timer a second away: the clock moves on by about the 20 ms the
	// host waited.
	start := e.SystemTime()
	env.waitDeadline = start + 1000*ms

	env.HostIdle(e, sleep(20*time.Millisecond))

	if moved := e.SystemTime() - start; moved < 20*ms || moved >= 1000*ms {
		t.Errorf("the clock moved %d ms, want 20 ms or a little more", moved/ms)
	}

	// A timer 5 ms away: the clock stops at it.
	env.waitDeadline = e.SystemTime() + 5*ms

	env.HostIdle(e, sleep(30*time.Millisecond))

	if now := e.SystemTime(); now != env.waitDeadline {
		t.Errorf("the clock is at %d, want the timer's %d", now, env.waitDeadline)
	}

	// No timer: the clock stays.
	env.waitDeadline = 0
	start = e.SystemTime()

	env.HostIdle(e, sleep(10*time.Millisecond))

	if now := e.SystemTime(); now != start {
		t.Errorf("the clock moved %d with no timer", now-start)
	}

	if env.cpuTime != cpuTime {
		t.Errorf("the idle time was charged: CPU time %d, want %d", env.cpuTime, cpuTime)
	}
}
