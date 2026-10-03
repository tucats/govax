package corevms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// fakeClock makes env's clock a variable the test advances by hand.
func fakeClock(env *Environment) *uint64 {
	now := uint64(1_000_000_000)
	env.Clock = func() uint64 { return now }

	return &now
}

// quad stores a 64-bit time at a fresh address.
func (a *arena) quad(v int64) uint32 {
	addr := a.alloc(8)
	putLongword(a.t, a.env, addr, uint32(v))
	putLongword(a.t, a.env, addr+4, uint32(uint64(v)>>32))

	return addr
}

const ms = 10_000 // one millisecond in VMS time units

func flagSet(env *Environment, efn uint32) bool {
	return env.Process.LocalEventFlags[efn/32]&(1<<(efn%32)) != 0
}

func TestServiceSysSetimrDelta(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	// The flag is cleared when the timer is set, and set once the delta
	// has passed — seen by the next event-flag service.
	env.Process.LocalEventFlags[0] = 1 << 4
	wantR0(t, callLNM(t, env, serviceSysSetimr, 4, a.quad(-50*ms)), ssNormal)

	if flagSet(env, 4) {
		t.Fatal("flag 4 still set after $SETIMR")
	}

	*now += 49 * ms

	wantR0(t, callLNM(t, env, serviceSysReadef, 4), ssWasClr)

	*now += 1 * ms

	wantR0(t, callLNM(t, env, serviceSysReadef, 4), ssWasSet)

	if env.PendingTimers() != 0 {
		t.Errorf("%d timers queued after expiry, want 0", env.PendingTimers())
	}
}

func TestServiceSysSetimrAbsoluteAndDefaults(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	// An absolute time: fires once reached. efn omitted means flag 0.
	wantR0(t, callLNM(t, env, serviceSysSetimr, 0, a.quad(int64(*now+10*ms))), ssNormal)

	*now += 10 * ms

	wantR0(t, callLNM(t, env, serviceSysReadef, 0), ssWasSet)

	// An absolute time already past fires at the next check.
	wantR0(t, callLNM(t, env, serviceSysSetimr, 7, a.quad(1)), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysReadef, 7), ssWasSet)

	// A $WAITFR on a timer's flag waits until the time passes.
	wantR0(t, callLNM(t, env, serviceSysSetimr, 9, a.quad(-5*ms)), ssNormal)

	if _, err := serviceSysWaitfr(env, []uint32{9}); !errors.Is(err, ErrWait) {
		t.Fatalf("$WAITFR before expiry: err = %v, want ErrWait", err)
	}

	*now += 5 * ms

	wantR0(t, callLNM(t, env, serviceSysWaitfr, 9), ssNormal)
}

func TestServiceSysSetimrErrors(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	fakeClock(env)

	wantR0(t, callLNM(t, env, serviceSysSetimr, 128, a.quad(-ms)), ssIllEfc)
	wantR0(t, callLNM(t, env, serviceSysSetimr, 64, a.quad(-ms)), ssUnasEfc)
	wantR0(t, callLNM(t, env, serviceSysSetimr, 1, 0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysSetimr, 1, 0x7FFF0000), ssAccVio)

	if env.PendingTimers() != 0 {
		t.Errorf("%d timers queued by failed calls, want 0", env.PendingTimers())
	}
}

// TestServiceSysSetimrCommonCluster: a timer on a common cluster's flag
// sets it in the cluster; if the cluster is gone by then, nothing happens.
func TestServiceSysSetimrCommonCluster(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc("T"), 0, 1), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysSetimr, 65, a.quad(-ms)), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysSetimr, 66, a.quad(-2*ms)), ssNormal)

	*now += ms

	wantR0(t, callLNM(t, env, serviceSysReadef, 65), ssWasSet)
	wantR0(t, callLNM(t, env, serviceSysDacefc, 64), ssNormal)

	*now += ms

	wantR0(t, callLNM(t, env, serviceSysReadef, 1), ssWasClr) // runs the expiry

	if c, _ := env.EventFlagClusters.Lookup(1, "T"); c.Flags&(1<<2) != 0 {
		t.Error("timer set a flag in a cluster the process had disassociated")
	}
}

func TestServiceSysCantim(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	set := func(efn, reqidt uint32) {
		t.Helper()
		wantR0(t, callLNM(t, env, serviceSysSetimr, efn, a.quad(-ms), 0, reqidt), ssNormal)
	}

	// Cancel by request ID; the others still fire.
	set(1, 7)
	set(2, 7)
	set(3, 8)
	wantR0(t, callLNM(t, env, serviceSysCantim, 7), ssNormal)

	*now += ms

	wantR0(t, callLNM(t, env, serviceSysReadef, 3), ssWasSet)

	if flagSet(env, 1) || flagSet(env, 2) {
		t.Error("a cancelled timer set its flag")
	}

	// reqidt 0 cancels everything the caller's mode may cancel: not a
	// kernel request when called from supervisor mode.
	set(10, 1)
	setCurMod(env, vax.Supervisor)
	set(11, 2)
	wantR0(t, callLNM(t, env, serviceSysCantim), ssNormal)

	if env.PendingTimers() != 1 {
		t.Fatalf("%d timers left, want the kernel-mode one", env.PendingTimers())
	}

	// acmode is maximized: asking for kernel from supervisor still can't
	// reach the kernel request. From kernel mode it can.
	wantR0(t, callLNM(t, env, serviceSysCantim, 0, 0), ssNormal)

	if env.PendingTimers() != 1 {
		t.Fatal("supervisor mode cancelled a kernel-mode timer")
	}

	setCurMod(env, vax.Kernel)
	wantR0(t, callLNM(t, env, serviceSysCantim, 0, 0), ssNormal)

	if env.PendingTimers() != 0 {
		t.Errorf("%d timers left after kernel $CANTIM, want 0", env.PendingTimers())
	}
}

func TestTimersImageRundown(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)

	wantR0(t, callLNM(t, env, serviceSysSetimr, 5, a.quad(-ms)), ssNormal)
	env.ImageRundown()

	*now += ms

	wantR0(t, callLNM(t, env, serviceSysReadef, 5), ssWasClr)
}
