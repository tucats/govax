package corevms

import (
	"errors"
	"testing"
)

// hiber calls $HIBER, reporting whether it returned (true) or is still
// waiting (false).
func hiber(t *testing.T, env *Environment) bool {
	t.Helper()

	r0, err := serviceSysHiber(env, nil)
	if errors.Is(err, ErrWait) {
		return false
	}

	if err != nil {
		t.Fatalf("$HIBER: %v", err)
	}

	wantR0(t, r0, ssNormal)

	return true
}

func TestServiceSysHiberWake(t *testing.T) {
	env, _ := fixture()

	// Nothing pending: $HIBER waits, however often it's retried.
	for i := 0; i < 3; i++ {
		if hiber(t, env) {
			t.Fatalf("attempt %d: $HIBER returned with no wakeup", i)
		}
	}

	// $WAKE ends it, and the wakeup is consumed.
	wantR0(t, callLNM(t, env, serviceSysWake), ssNormal)

	if !hiber(t, env) {
		t.Fatal("$HIBER still waiting after $WAKE")
	}

	if env.Process.WakePending {
		t.Error("wakeup still pending after $HIBER consumed it")
	}

	// Wakeups aren't counted: two $WAKEs end one $HIBER.
	callLNM(t, env, serviceSysWake)
	callLNM(t, env, serviceSysWake)

	if !hiber(t, env) {
		t.Fatal("first $HIBER after two $WAKEs waited")
	}

	if hiber(t, env) {
		t.Fatal("second $HIBER after two $WAKEs returned: wakeups were counted")
	}
}

// TestServiceSysWakeTarget covers picking the process for $WAKE; $SCHDWK
// and $CANWAK use the same processTarget.
func TestServiceSysWakeTarget(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	pid := env.Process.PID

	// Our own PID, and a PID of 0 filled in.
	wantR0(t, callLNM(t, env, serviceSysWake, a.long(pid)), ssNormal)

	pidadr := a.long(0)
	wantR0(t, callLNM(t, env, serviceSysWake, pidadr), ssNormal)

	if got := a.readLong(pidadr); got != pid {
		t.Errorf("pidadr = %#x after $WAKE, want our PID %#x", got, pid)
	}

	// Our own name.
	wantR0(t, callLNM(t, env, serviceSysWake, 0, a.desc("SYSTEM")), ssNormal)

	env.Process.WakePending = false

	// Other processes don't exist; -1 is no wildcard here.
	wantR0(t, callLNM(t, env, serviceSysWake, a.long(pid+1)), ssNonExpr)
	wantR0(t, callLNM(t, env, serviceSysWake, a.long(0xFFFFFFFF)), ssNonExpr)
	wantR0(t, callLNM(t, env, serviceSysWake, 0, a.desc("OTHER")), ssNonExpr)
	wantR0(t, callLNM(t, env, serviceSysWake, 0, a.desc("")), ssIvLogNam)
	wantR0(t, callLNM(t, env, serviceSysWake, 0x7FFFFFF0), ssAccVio)

	if env.Process.WakePending {
		t.Error("a failed $WAKE set the wakeup flag")
	}
}

func TestServiceSysSchdwkDelta(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	wantR0(t, callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(-30*ms)), ssNormal)

	*now += 29 * ms

	if hiber(t, env) {
		t.Fatal("$HIBER returned before the scheduled wakeup")
	}

	*now += ms

	if !hiber(t, env) {
		t.Fatal("$HIBER still waiting at the scheduled wakeup")
	}

	if env.PendingTimers() != 0 {
		t.Errorf("%d requests queued after a one-shot wakeup, want 0", env.PendingTimers())
	}
}

func TestServiceSysSchdwkAbsolute(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	// In the future: wakes at that time.
	wantR0(t, callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(int64(*now+5*ms))), ssNormal)

	*now += 4 * ms

	if hiber(t, env) {
		t.Fatal("$HIBER returned before the absolute wakeup time")
	}

	*now += ms

	if !hiber(t, env) {
		t.Fatal("$HIBER still waiting at the absolute wakeup time")
	}

	// Already past: wakes at the next check.
	wantR0(t, callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(int64(*now-ms))), ssNormal)

	if !hiber(t, env) {
		t.Fatal("$HIBER waited for a wakeup whose time had passed")
	}
}

func TestServiceSysSchdwkRepeat(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	// First at 10ms, then every 20ms.
	wantR0(t, callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(-10*ms), a.quad(-20*ms)), ssNormal)

	for i, at := range []uint64{10, 30, 50} {
		*now = 1_000_000_000 + (at-1)*ms

		if hiber(t, env) {
			t.Fatalf("wakeup %d came before %dms", i, at)
		}

		*now += ms

		if !hiber(t, env) {
			t.Fatalf("wakeup %d didn't come at %dms", i, at)
		}
	}

	// Missed repetitions collapse into one wakeup, and the next is
	// still on the 20ms grid: 50 + 20k.
	*now = 1_000_000_000 + 125*ms

	if !hiber(t, env) {
		t.Fatal("no wakeup after missing several repetitions")
	}

	if hiber(t, env) {
		t.Fatal("missed repetitions were counted")
	}

	if got := env.timers[0].expiry; got != 1_000_000_000+130*ms {
		t.Errorf("next repetition at %d, want 130ms", (got-1_000_000_000)/ms)
	}
}

func TestServiceSysSchdwkMinimumRepeat(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	fakeClock(env)

	wantR0(t, callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(-ms), a.quad(-1)), ssNormal)

	if got := env.timers[0].repeat; got != 10*ms {
		t.Errorf("repeat = %d, want 10ms (%d)", got, 10*ms)
	}
}

func TestServiceSysSchdwkErrors(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	wantR0(t, callLNM(t, env, serviceSysSchdwk, 0, 0, 0), ssAccVio)          // no daytim
	wantR0(t, callLNM(t, env, serviceSysSchdwk), ssAccVio)                   // nothing at all
	wantR0(t, callLNM(t, env, serviceSysSchdwk, 0, 0, 0x7FFFFFF0), ssAccVio) // unreadable daytim
	wantR0(t, callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(-ms), 0x7FFFFFF0), ssAccVio)

	// reptim must be a delta; an absolute daytim plus one repetition must
	// not already be past.
	wantR0(t, callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(-ms), a.quad(int64(20*ms))), ssIvTime)
	wantR0(t, callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(int64(*now-50*ms)), a.quad(-20*ms)), ssIvTime)

	// Another process.
	wantR0(t, callLNM(t, env, serviceSysSchdwk, a.long(env.Process.PID+1), 0, a.quad(-ms)), ssNonExpr)

	if env.PendingTimers() != 0 {
		t.Errorf("%d requests queued by failed calls, want 0", env.PendingTimers())
	}
}

func TestServiceSysCanwak(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	// Two wakeups and a $SETIMR timer queued; a $WAKE already pending.
	callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(-10*ms))
	callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(-20*ms), a.quad(-20*ms))
	callLNM(t, env, serviceSysSetimr, 3, a.quad(-10*ms))
	callLNM(t, env, serviceSysWake)

	// $CANTIM leaves the wakeups alone.
	wantR0(t, callLNM(t, env, serviceSysCantim), ssNormal)

	if env.PendingTimers() != 2 {
		t.Fatalf("%d requests left after $CANTIM, want the 2 wakeups", env.PendingTimers())
	}

	callLNM(t, env, serviceSysSetimr, 3, a.quad(-10*ms))

	// $CANWAK removes both wakeups, not the timer or the pending $WAKE.
	wantR0(t, callLNM(t, env, serviceSysCanwak), ssNormal)

	if env.PendingTimers() != 1 {
		t.Errorf("%d requests left after $CANWAK, want the timer", env.PendingTimers())
	}

	if !hiber(t, env) {
		t.Fatal("$CANWAK cancelled the pending $WAKE")
	}

	*now += 30 * ms

	if hiber(t, env) {
		t.Fatal("a cancelled wakeup still woke the process")
	}

	wantR0(t, callLNM(t, env, readefState, 3), ssWasSet)

	// Other processes.
	wantR0(t, callLNM(t, env, serviceSysCanwak, 0, a.desc("OTHER")), ssNonExpr)
}

func TestScheduledWakeupsImageRundown(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	callLNM(t, env, serviceSysSchdwk, 0, 0, a.quad(-10*ms), a.quad(-10*ms))
	env.ImageRundown()

	*now += 50 * ms

	if hiber(t, env) {
		t.Fatal("a wakeup scheduled by the previous image still woke the process")
	}
}
