package corevms

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/tucats/govax/internal/lck"
)

// lockCaller is a process calling the lock services in tests: its own
// memory region (the fixture's processes share physical memory), with an
// LKSB (and value block) and a resource name.
type lockCaller struct {
	t    *testing.T
	env  *Environment
	a    *arena
	lksb uint32
}

func newLockCaller(t *testing.T, env *Environment, base uint32) *lockCaller {
	a := &arena{t: t, env: env, next: base}

	return &lockCaller{t: t, env: env, a: a, lksb: a.alloc(24)}
}

// The completion and blocking AST routines' addresses (never run here).
const (
	testCplAST = 0x7000
	testBlkAST = 0x7100
)

// enq calls $ENQ (or, with wait, $ENQW) for name at mode with flags,
// event flag 5, and both ASTs.
func (c *lockCaller) enq(name string, mode lck.Mode, flags uint32) uint32 {
	c.t.Helper()

	return callLNM(c.t, c.env, serviceSysEnq, c.args(name, mode, flags)...)
}

func (c *lockCaller) args(name string, mode lck.Mode, flags uint32) []uint32 {
	resnam := uint32(0)
	if name != "" {
		resnam = c.a.desc(name)
	}

	return []uint32{5, uint32(mode), c.lksb, flags, resnam, 0, testCplAST, 0x42, testBlkAST, 0}
}

// status and id read the LKSB.
func (c *lockCaller) status() uint32 {
	var b [8]byte
	if err := c.env.mem.Load(c.env.cpu, c.lksb, b[:]); err != nil {
		c.t.Fatal(err)
	}

	return uint32(binary.LittleEndian.Uint16(b[:]))
}

func (c *lockCaller) id() uint32 {
	var b [4]byte
	if err := c.env.mem.Load(c.env.cpu, c.lksb+4, b[:]); err != nil {
		c.t.Fatal(err)
	}

	return binary.LittleEndian.Uint32(b[:])
}

// flagSet reports whether event flag 5 is set.
func (c *lockCaller) flagSet() bool {
	return c.env.Process.LocalEventFlags[0]&(1<<5) != 0
}

// asts returns the routines of the process's queued ASTs.
func (c *lockCaller) asts() []uint32 {
	var out []uint32
	for _, a := range c.env.Process.ast.queue {
		out = append(out, a.routine)
	}

	return out
}

func newLockPair(t *testing.T) (*lockCaller, *lockCaller) {
	env, _ := fixture()
	a := newProcess(t, env)
	b := newProcess(t, env)

	return newLockCaller(t, a, 0x10000), newLockCaller(t, b, 0x20000)
}

// TestEnq_grantAndWait: a lock granted at once completes at once (LKSB,
// event flag, AST); one that must wait is queued with its ID, and
// completes when the lock blocking it is dequeued, in its own process.
func TestEnq_grantAndWait(t *testing.T) {
	a, b := newLockPair(t)

	wantR0(t, a.enq("RES", lck.EX, 0), ssNormal)

	if a.status() != ssNormal || a.id() == 0 || !a.flagSet() || len(a.asts()) != 1 || a.asts()[0] != testCplAST {
		t.Fatalf("A: status %#x, id %#x, flag %v, ASTs %x", a.status(), a.id(), a.flagSet(), a.asts())
	}

	wantR0(t, b.enq("RES", lck.PR, 0), ssNormal)

	if b.status() != 0 || b.id() == 0 || b.flagSet() || len(b.asts()) != 0 {
		t.Fatalf("B queued: status %#x, id %#x, flag %v, ASTs %x", b.status(), b.id(), b.flagSet(), b.asts())
	}

	// A's lock blocks B's: A's blocking AST is queued.
	if got := a.asts(); len(got) != 2 || got[1] != testBlkAST {
		t.Errorf("A's ASTs %x, want the completion and the blocking AST", got)
	}

	wantR0(t, callLNM(t, a.env, serviceSysDeq, a.id(), 0, 0, 0), ssNormal)

	if b.status() != ssNormal || !b.flagSet() || len(b.asts()) != 1 {
		t.Errorf("B after A's $DEQ: status %#x, flag %v, ASTs %x", b.status(), b.flagSet(), b.asts())
	}
}

// TestEnq_flags: NOQUEUE, SYNCSTS, and an invalid mode or name.
func TestEnq_flags(t *testing.T) {
	a, b := newLockPair(t)

	wantR0(t, a.enq("RES", lck.EX, lckSyncSts), ssSynch)

	if a.flagSet() || len(a.asts()) != 0 || a.status() != ssNormal {
		t.Errorf("SYNCSTS: flag %v, ASTs %x, status %#x; want no flag or AST", a.flagSet(), a.asts(), a.status())
	}

	wantR0(t, b.enq("RES", lck.CR, lckNoQueue), ssNotQueued)

	if st := b.status(); st == ssNotQueued {
		t.Errorf("NOQUEUE: the LKSB's status is SS$_NOTQUEUED; VMS leaves it alone")
	}
	wantR0(t, b.enq("RES", lck.Mode(6), 0), ssBadParam)
	wantR0(t, b.enq("", lck.NL, 0), ssAccVio)
	wantR0(t, b.enq("A_NAME_THAT_IS_MUCH_TOO_LONG_TO_BE_ONE", lck.NL, 0), ssIvBufLen)
}

// TestEnq_conversionAndValueBlock: an EX holder writes the value block
// on its way down; a new lock with VALBLK reads it.
func TestEnq_conversionAndValueBlock(t *testing.T) {
	a, b := newLockPair(t)

	wantR0(t, a.enq("RES", lck.EX, lckValBlk), ssNormal)

	value := []byte("sixteen bytes!!!")
	putBytes(t, a.env, a.lksb+8, value)

	wantR0(t, callLNM(t, a.env, serviceSysEnq, a.args("", lck.NL, lckConvert|lckValBlk)...), ssNormal)

	if a.status() != ssNormal {
		t.Fatalf("conversion status %#x", a.status())
	}

	wantR0(t, b.enq("RES", lck.PR, lckValBlk), ssNormal)

	got := make([]byte, 16)
	if err := b.env.mem.Load(b.env.cpu, b.lksb+8, got); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, value) {
		t.Errorf("B's value block %q, want %q", got, value)
	}

	// A conversion of a lock that isn't the caller's.
	putLongword(t, b.env, b.lksb+4, a.id())
	wantR0(t, callLNM(t, b.env, serviceSysEnq, b.args("", lck.EX, lckConvert)...), ssIvLockID)
}

// TestDeq_errorsAndCancel: $DEQ's refusals, CANCEL of a waiting request
// (SS$_ABORT in its LKSB), and DEQALL.
func TestDeq_errorsAndCancel(t *testing.T) {
	a, b := newLockPair(t)

	wantR0(t, callLNM(t, a.env, serviceSysDeq, 0, 0, 0, 0), ssIvLockID)

	wantR0(t, a.enq("RES", lck.EX, 0), ssNormal)
	wantR0(t, callLNM(t, b.env, serviceSysDeq, a.id(), 0, 0, 0), ssIvLockID)
	wantR0(t, callLNM(t, a.env, serviceSysDeq, a.id(), 0, 0, lckCancel), ssCancelGrant)

	wantR0(t, b.enq("RES", lck.EX, 0), ssNormal)
	wantR0(t, callLNM(t, b.env, serviceSysDeq, b.id(), 0, 0, lckCancel), ssNormal)

	if b.status() != ssAbort || !b.flagSet() {
		t.Errorf("the canceled request: status %#x, flag %v; want SS$_ABORT, set", b.status(), b.flagSet())
	}

	wantR0(t, a.enq("OTHER", lck.EX, 0), ssNormal)
	wantR0(t, callLNM(t, a.env, serviceSysDeq, 0, 0, 0, lckDeqAll), ssNormal)

	if n := len(a.env.Locks.Locks(lck.Owner(a.env.Process.PID))); n != 0 {
		t.Errorf("A has %d locks after DEQALL", n)
	}
}

// TestEnqw_waits: $ENQW of a lock that must wait waits (ErrWait, with
// its wait condition), and returns once it's granted.
func TestEnqw_waits(t *testing.T) {
	a, b := newLockPair(t)

	wantR0(t, a.enq("RES", lck.EX, 0), ssNormal)

	args := b.args("RES", lck.PR, 0)
	if _, err := serviceSysEnqw(b.env, args); !errors.Is(err, ErrWait) {
		t.Fatalf("$ENQW: %v, want ErrWait", err)
	}

	if b.env.pendingWait == nil || b.env.pendingWait.over() {
		t.Fatal("no wait, or the wait is over already")
	}

	if _, err := serviceSysEnqw(b.env, args); !errors.Is(err, ErrWait) {
		t.Fatalf("$ENQW again, still blocked: %v", err)
	}

	wantR0(t, callLNM(t, a.env, serviceSysDeq, a.id(), 0, 0, 0), ssNormal)

	if !b.env.pendingWait.over() {
		t.Fatal("the wait isn't over after the $DEQ")
	}

	wantR0(t, callLNM(t, b.env, serviceSysEnqw, args...), ssNormal)

	if b.status() != ssNormal || len(b.env.enqWaits) != 0 {
		t.Errorf("after $ENQW: status %#x, %d waits left", b.status(), len(b.env.enqWaits))
	}
}

// TestEnq_processDeletion: a deleted process's locks go, and the request
// waiting for them is granted in its owner's process.
func TestEnq_processDeletion(t *testing.T) {
	a, b := newLockPair(t)

	wantR0(t, a.enq("RES", lck.EX, 0), ssNormal)
	wantR0(t, b.enq("RES", lck.EX, 0), ssNormal)

	a.env.processRundown()

	if b.status() != ssNormal || !b.flagSet() {
		t.Errorf("B after A's deletion: status %#x, flag %v", b.status(), b.flagSet())
	}
}
