package corevms

import (
	"errors"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Events reported to the scheduler (docs/PHASE-46.md, subtask 1): an
// I/O completion, or a timer's expiry, ends a waiting process's wait
// when it happens, with the event's boost, rather than at the
// scheduler's next look at its waiters.

// wantPriority checks env's current priority against its base.
func wantPriority(t *testing.T, env *Environment, boost int) {
	t.Helper()

	if info, _ := env.Scheduler().Info(handle(env)); info.Priority != info.Base+boost {
		t.Errorf("priority %d, base %d: want a boost of %d", info.Priority, info.Base, boost)
	}
}

// TestReportEvent: an event ends a wait only if the wait is over, and
// not while the process is suspended; it then boosts by the event's
// class, not the wait's.
func TestReportEvent(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	if err := callWaiting(env, serviceSysWaitfr, 3); !errors.Is(err, ErrWait) {
		t.Fatalf("err %v, want ErrWait", err)
	}

	env.reportEvent(sched.ClassTerminalInput) // flag 3 is still clear
	wantState(t, env, sched.StateLEF, sched.ResourceNone)

	env.Process.LocalEventFlags[0] |= 1 << 3
	env.suspended = true
	env.reportEvent(sched.ClassTerminalInput)
	wantState(t, env, sched.StateLEF, sched.ResourceNone)

	env.suspended = false
	env.reportEvent(sched.ClassTerminalInput)
	wantState(t, env, sched.StateCOM, sched.ResourceNone)
	wantPriority(t, env, 6)

	if env.waiters != 0 || env.waiting != nil {
		t.Errorf("%d waiters, waiting %v after the wait ended", env.waiters, env.waiting)
	}

	env.reportEvent(sched.ClassTerminalInput) // not waiting: ignored
	wantState(t, env, sched.StateCOM, sched.ResourceNone)
}

// TestReportEvent_mailboxRead: another process's write to a mailbox
// completes a waiting $QIOW read, which is computable at once (with no
// wakeWaiters), boosted as an I/O completion (2).
func TestReportEvent_mailboxRead(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	a := newArena(t, env)
	_, ch := crembx(t, env, a, 0, 16, 16, "")
	buf := a.alloc(16)

	reader := newProcess(t, env)
	rch := assignCall(t, reader, a, mailboxOn(t, env, ch).Device.Name, 0)

	reader.cpu.SetGPR(vax.FP, 0x7000)

	if err := callWaiting(reader, serviceSysQiow, 0, rch, fnReadVBlk, 0, 0, 0, buf, 16); !errors.Is(err, ErrWait) {
		t.Fatalf("read of an empty mailbox: err %v, want ErrWait", err)
	}

	wantState(t, reader, sched.StateLEF, sched.ResourceNone)

	wantR0(t, mbxQIO(t, env, 0, ch, fnWriteNow, 0, 0, a.str("hi"), 2), ssNormal)

	wantState(t, reader, sched.StateCOM, sched.ResourceNone)
	wantPriority(t, reader, 2)
}

// TestReportEvent_timer: a timer that sets the flag a process waits for
// ends its wait with a timer's boost (PRI$_TIMER, 3), not an event
// flag's.
func TestReportEvent_timer(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	now := uint64(0x00A0_0000_0000_0000)
	env.Clock = func() uint64 { return now }

	if err := callWaiting(env, serviceSysWaitfr, 5); !errors.Is(err, ErrWait) {
		t.Fatalf("err %v, want ErrWait", err)
	}

	env.timers = append(env.timers, &timerRequest{expiry: now + 10, efn: 5})

	env.expireTimers() // not due yet
	wantState(t, env, sched.StateLEF, sched.ResourceNone)

	now += 10
	env.expireTimers()
	wantState(t, env, sched.StateCOM, sched.ResourceNone)
	wantPriority(t, env, 3)
}

// TestIOBoost: a terminal read completes with terminal input's boost,
// any other terminal function with terminal output's, and every other
// device's I/O with an I/O completion's.
func TestIOBoost(t *testing.T) {
	tt := &iodev.Device{DevClass: iodev.DeviceClassTT}
	mbx := &iodev.Device{DevClass: iodev.DeviceClassMailbox}
	disk := &iodev.Device{DevClass: iodev.DeviceClassDisk}

	for _, c := range []struct {
		device   *iodev.Device
		function string
		want     sched.Class
	}{
		{tt, "IO$_READVBLK", sched.ClassTerminalInput},
		{tt, "IO$_READPROMPT", sched.ClassTerminalInput},
		{tt, "IO$_TTYREADALL", sched.ClassTerminalInput},
		{tt, "IO$_WRITEVBLK", sched.ClassTerminalOutput},
		{tt, "IO$_SETMODE", sched.ClassTerminalOutput},
		{mbx, "IO$_READVBLK", sched.ClassIOCompletion},
		{mbx, "IO$_WRITEVBLK", sched.ClassIOCompletion},
		{disk, "IO$_READVBLK", sched.ClassIOCompletion},
	} {
		if got := ioBoost(c.device, vmsdef.Symbols[c.function]); got != c.want {
			t.Errorf("%v %s: %s, want %s", c.device.DevClass, c.function, got, c.want)
		}
	}
}

// TestReportEvent_mailboxRoom: a process waiting to write to a full
// mailbox (RWMBX) becomes computable as soon as another process's read
// makes room, with a resource's boost (3), without the scheduler's next
// look at its waiters.
func TestReportEvent_mailboxRoom(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	a := newArena(t, env)
	_, ch := crembx(t, env, a, 0, 4, 4, "")
	buf := a.alloc(4)

	writer := newProcess(t, env)
	wch := assignCall(t, writer, a, mailboxOn(t, env, ch).Device.Name, 0)

	wantR0(t, mbxQIO(t, writer, 0, wch, fnWriteNow, 0, 0, a.str("abcd"), 4), ssNormal)

	if err := callWaiting(writer, serviceSysQio, 0, wch, fnWriteNow, 0, 0, 0, a.str("efgh"), 4); !errors.Is(err, ErrWait) {
		t.Fatalf("write to a full mailbox: err %v, want ErrWait", err)
	}

	wantState(t, writer, sched.StateMWAIT, sched.ResourceMailbox)

	wantR0(t, mbxQIO(t, env, 0, ch, fnReadNow, 0, 0, buf, 4), ssNormal)
	wantState(t, writer, sched.StateCOM, sched.ResourceNone)
	wantPriority(t, writer, 3)
}
