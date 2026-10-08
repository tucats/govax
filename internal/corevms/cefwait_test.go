package corevms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/sched"
)

// Common event flags across processes (docs/PHASE-46.md, subtask 4): a
// flag set in a common cluster, by $SETEF, a timer, or a completing
// request, ends the CEF waits it satisfies in every process associated
// with the cluster, at once (postFlag, as VMS's SCH$POSTEF), and a
// temporary cluster goes when its last associated process does.

// wantCEFWait calls a wait service in env and checks that it waits in CEF.
func wantCEFWait(t *testing.T, env *Environment, fn ServiceFunc, argv ...uint32) {
	t.Helper()

	if err := callWaiting(env, fn, argv...); !errors.Is(err, ErrWait) {
		t.Fatalf("err %v, want ErrWait", err)
	}

	wantState(t, env, sched.StateCEF, sched.ResourceNone)
}

// TestCommonFlags_setef: one process's $SETEF ends another's $WAITFR on
// the cluster at once, without a boost; a $WFLAND waits for all its
// flags; a process with the same cluster under another cluster number
// sees the same flags; a process in another UIC group, naming a cluster
// the same, has a different cluster and goes on waiting.
func TestCommonFlags_setef(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	a := newArena(t, env)
	name := a.desc("SHARED")

	waiter, waitall, stranger := newProcess(t, env), newProcess(t, env), newProcess(t, env)
	stranger.Process.UIC = env.Process.UIC + 0x10000 // the next UIC group

	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, name), ssNormal)
	wantR0(t, callLNM(t, waiter, serviceSysAscefc, 64, name), ssNormal)
	wantR0(t, callLNM(t, waitall, serviceSysAscefc, 96, name), ssNormal)
	wantR0(t, callLNM(t, stranger, serviceSysAscefc, 64, name), ssNormal)

	if waiter.Process.CommonClusters[0] != waitall.Process.CommonClusters[1] ||
		stranger.Process.CommonClusters[0] == waiter.Process.CommonClusters[0] {
		t.Fatal("cluster identity: same group should share SHARED, another group should not")
	}

	wantCEFWait(t, waiter, serviceSysWaitfr, 65)
	wantCEFWait(t, waitall, serviceSysWfland, 96, 1<<1|1<<2) // flags 97 and 98
	wantCEFWait(t, stranger, serviceSysWaitfr, 65)

	wantR0(t, callLNM(t, env, serviceSysSetef, 65), ssWasClr)

	wantState(t, waiter, sched.StateCOM, sched.ResourceNone)
	wantPriority(t, waiter, 0)
	wantState(t, waitall, sched.StateCEF, sched.ResourceNone) // flag 98 still clear
	wantState(t, stranger, sched.StateCEF, sched.ResourceNone)

	wantR0(t, callLNM(t, env, serviceSysSetef, 66), ssWasClr)

	wantState(t, waitall, sched.StateCOM, sched.ResourceNone)
	wantState(t, stranger, sched.StateCEF, sched.ResourceNone)

	// The woken processes' services now finish.
	wantR0(t, callLNM(t, waiter, serviceSysWaitfr, 65), ssNormal)
	wantR0(t, callLNM(t, waitall, serviceSysWfland, 96, 1<<1|1<<2), ssNormal)
}

// TestCommonFlags_timer: a timer setting a common flag ends another
// process's wait for it, with a timer's boost (3), and a completing
// $GETJPI's flag likewise with an I/O completion's (2).
func TestCommonFlags_timer(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	now := uint64(0x00A0_0000_0000_0000)
	env.Clock = func() uint64 { return now }

	a := newArena(t, env)
	name := a.desc("TICK")

	waiter := newProcess(t, env)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, name), ssNormal)
	wantR0(t, callLNM(t, waiter, serviceSysAscefc, 64, name), ssNormal)

	wantCEFWait(t, waiter, serviceSysWaitfr, 70)

	env.timers = append(env.timers, &timerRequest{expiry: now, efn: 70})
	env.expireTimers()

	wantState(t, waiter, sched.StateCOM, sched.ResourceNone)
	wantPriority(t, waiter, 3)

	// A completing request's flag: $GETJPIW of the setter itself.
	other := newProcess(t, env)
	wantR0(t, callLNM(t, other, serviceSysAscefc, 64, name), ssNormal)
	wantCEFWait(t, other, serviceSysWaitfr, 71)

	itmlst := a.alloc(4) // an empty item list: just its terminator
	wantR0(t, callLNM(t, env, serviceSysGetjpi, 71, 0, 0, itmlst, 0, 0, 0), ssNormal)

	wantState(t, other, sched.StateCOM, sched.ResourceNone)
	wantPriority(t, other, 2)
}

// TestCommonFlags_lifetime: a temporary cluster lives while any process
// is associated with it, whichever created it, and goes with the last
// association, whether given up by $DACEFC or by the deletion of the
// process that held it; a permanent one stays.
func TestCommonFlags_lifetime(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	a := newArena(t, env)
	table := env.EventFlagClusters
	group := env.Process.UICGroup()

	child := newProcess(t, env)

	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc("TEMP")), ssNormal)
	wantR0(t, callLNM(t, child, serviceSysAscefc, 96, a.desc("TEMP")), ssNormal)
	wantR0(t, callLNM(t, child, serviceSysAscefc, 64, a.desc("PERM"), 0, 1), ssNormal)

	wantR0(t, callLNM(t, child, serviceSysSetef, 100), ssWasClr)

	// The creator gives it up: the child's association keeps it, and its
	// flags.
	wantR0(t, callLNM(t, env, serviceSysDacefc, 64), ssNormal)

	c, ok := table.Lookup(group, "TEMP")
	if !ok || c.References() != 1 || c.Flags != 1<<4 {
		t.Fatalf("TEMP after the creator's $DACEFC: %v, %+v", ok, c)
	}

	// The child is deleted: its rundown takes the last association.
	env.DeleteProcess(child)

	if _, ok := table.Lookup(group, "TEMP"); ok {
		t.Error("TEMP outlived its last associated process")
	}

	if c, ok := table.Lookup(group, "PERM"); !ok || c.References() != 0 {
		t.Errorf("PERM after its process's deletion: %v, %+v", ok, c)
	}
}
