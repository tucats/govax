package rms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/lck"
)

var errTestWait = errors.New("waiting")

// newLockingSharers is newSharers with record locking: one lock database
// for all, each process its own PID, and a wait hook that records the
// condition and returns errTestWait.
func newLockingSharers(t *testing.T, n int) ([]*sharer, *MountTable, []func() bool) {
	t.Helper()

	p, mounts := newSharers(t, n)
	locks := lck.NewManager()
	waits := make([]func() bool, n)

	for i, s := range p {
		s.ctx.Locks = locks
		s.ctx.PID = uint32(0x301 + i)
		s.ctx.AwaitLock = func(over func() bool, _ uint64) error {
			waits[i] = over

			return errTestWait
		}
	}

	return p, mounts, waits
}

// getWith is get with RAB$L_ROP rop.
func (s *sharer) getWith(rop uint32) (string, uint32) {
	s.t.Helper()
	putLongwordAt(s.t, s.ctx, testRabAddr+rabROP, rop)

	return s.get()
}

// rabRFAOf reads the RAB's RAB$W_RFA.
func (s *sharer) rabRFAOf() rfa {
	s.t.Helper()

	r, err := s.ctx.loadRFA(testRabAddr)
	if err != nil {
		s.t.Fatal(err)
	}

	return r
}

var shrAll = shrGet | shrPut | shrUpd

// TestRecordLock_basics: a stream that may write locks each record it
// reads (EX); another's $GET of it is RMS$_RLK and doesn't consume it;
// RRL reads it anyway; and the lock moves on with the next $GET.
func TestRecordLock_basics(t *testing.T) {
	p, mounts, _ := newLockingSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "REC.DAT", "R0", "R1", "R2")

	a.mustOpen("REC.DAT", facGet|facUpd, shrAll, false, 0)
	b.mustOpen("REC.DAT", facGet, shrAll, false, 0)

	if rec, sts := a.get(); rec != "R0" || sts != rmsNormal {
		t.Fatalf("A's first $GET: %q, %#x", rec, sts)
	}

	if got := a.rabRFAOf(); got != (rfa{1, 0}) {
		t.Errorf("A's RFA %+v, want block 1 offset 0", got)
	}

	for range 2 {
		if _, sts := b.get(); sts != rmsRecordLocked {
			t.Errorf("B's $GET of A's record: %#x, want RMS$_RLK", sts)
		}
	}

	if rec, sts := b.getWith(ropNLK); sts != rmsRecordLocked {
		t.Errorf("B's query of an EX-locked record: %q, %#x; want RMS$_RLK", rec, sts)
	}

	if rec, sts := b.getWith(ropRRL); rec != "R0" || sts != rmsNormal {
		t.Errorf("B's read-regardless $GET: %q, %#x; want R0", rec, sts)
	}

	if rec, _ := a.get(); rec != "R1" {
		t.Fatalf("A's second $GET: %q", rec)
	}

	if got := a.rabRFAOf(); got != (rfa{1, 4}) {
		t.Errorf("A's RFA %+v, want block 1 offset 4", got)
	}

	// B's next record is R1, which A holds now; R0 is free.
	if _, sts := b.getWith(0); sts != rmsRecordLocked {
		t.Errorf("B's $GET of R1: %#x, want RMS$_RLK", sts)
	}

	a.close()

	if rec, sts := b.get(); rec != "R1" || sts != rmsNormal {
		t.Errorf("B's $GET after A closed: %q, %#x", rec, sts)
	}

	b.close()
}

// TestRecordLock_rlkAndQuery: RAB$V_RLK locks a record so others may
// still read it with a query (RMS$_OK_RLK), but not lock it.
func TestRecordLock_rlkAndQuery(t *testing.T) {
	p, mounts, _ := newLockingSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "Q.DAT", "R0")

	a.mustOpen("Q.DAT", facGet|facUpd, shrAll, false, 0)
	b.mustOpen("Q.DAT", facGet, shrAll, false, 0)

	if rec, _ := a.getWith(ropRLK); rec != "R0" {
		t.Fatalf("A: %q", rec)
	}

	if rec, sts := b.getWith(ropNLK); rec != "R0" || sts != rmsOKRecordLocked {
		t.Errorf("B's query: %q, %#x; want R0, RMS$_OK_RLK", rec, sts)
	}

	a.close()
	b.close()
}

// TestRecordLock_ulkFreeRelease: with RAB$V_ULK records stay locked
// until $RELEASE (by RFA) or $FREE; with nothing locked, RMS$_RNL.
func TestRecordLock_ulkFreeRelease(t *testing.T) {
	p, mounts, _ := newLockingSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "U.DAT", "R0", "R1", "R2")

	a.mustOpen("U.DAT", facGet|facUpd, shrAll, false, 0)
	b.mustOpen("U.DAT", facGet, shrAll, false, 0)

	a.getWith(ropULK)
	a.getWith(ropULK) // R0 and R1 locked

	if n := len(a.ctx.Files.handles[readWord(t, a.ctx, testFabAddr+fabIFI)].locks.records); n != 2 {
		t.Fatalf("A holds %d record locks, want 2", n)
	}

	// $RELEASE R1 (the RAB's RFA is the last record's).
	if r0, err := SysRelease(a.ctx, []uint32{testRabAddr}); err != nil || r0 != rmsNormal {
		t.Fatalf("$RELEASE: %#x, %v", r0, err)
	}

	if r0, _ := SysRelease(a.ctx, []uint32{testRabAddr}); r0 != rmsRecordNotLocked {
		t.Errorf("$RELEASE again: %#x, want RMS$_RNL", r0)
	}

	// R0 is still A's.
	if _, sts := b.get(); sts != rmsRecordLocked {
		t.Errorf("B's $GET of R0: %#x, want RMS$_RLK", sts)
	}

	if r0, _ := SysFree(a.ctx, []uint32{testRabAddr}); r0 != rmsNormal {
		t.Errorf("$FREE: %#x", r0)
	}

	if r0, _ := SysFree(a.ctx, []uint32{testRabAddr}); r0 != rmsRecordNotLocked {
		t.Errorf("$FREE again: %#x, want RMS$_RNL", r0)
	}

	if rec, sts := b.get(); rec != "R0" || sts != rmsNormal {
		t.Errorf("B after $FREE: %q, %#x", rec, sts)
	}

	a.close()
	b.close()
}

// TestRecordLock_wait: with RAB$V_WAT a $GET of a locked record waits,
// and, called again once the lock is granted, returns the record with
// RMS$_OK_WAT.
func TestRecordLock_wait(t *testing.T) {
	p, mounts, waits := newLockingSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "W.DAT", "R0", "R1")

	a.mustOpen("W.DAT", facGet|facUpd, shrAll, false, 0)
	b.mustOpen("W.DAT", facGet, shrAll, false, 0)

	a.get()

	putLongwordAt(t, b.ctx, testRabAddr+rabROP, ropWAT)

	if _, err := SysGet(b.ctx, []uint32{testRabAddr}); !errors.Is(err, errTestWait) {
		t.Fatalf("B's waiting $GET: %v", err)
	}

	if waits[1]() {
		t.Fatal("the wait is over before A lets go")
	}

	a.get() // A moves on to R1, letting R0 go

	if !waits[1]() {
		t.Fatal("the wait isn't over after A let go")
	}

	if rec, sts := b.get(); rec != "R0" || sts != rmsOKWaited {
		t.Errorf("B's $GET, called again: %q, %#x; want R0, RMS$_OK_WAT", rec, sts)
	}

	a.close()
	b.close()
}

// TestRecordLock_unshared: a file no one else may write takes no record
// locks.
func TestRecordLock_unshared(t *testing.T) {
	p, mounts, _ := newLockingSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "N.DAT", "R0")

	a.mustOpen("N.DAT", facGet, shrGet, false, 0)
	b.mustOpen("N.DAT", facGet, shrGet, false, 0)

	a.get()

	if rec, sts := b.get(); rec != "R0" || sts != rmsNormal {
		t.Errorf("B: %q, %#x", rec, sts)
	}

	a.close()
	b.close()
}

// TestRecordLock_readersDefault: by default every stream locks
// exclusively, readers too (the guide, 7.2.2.5); read locks (REA) share.
func TestRecordLock_readersDefault(t *testing.T) {
	p, mounts, _ := newLockingSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "RD.DAT", "R0", "R1")

	a.mustOpen("RD.DAT", facGet, shrAll, false, 0)
	b.mustOpen("RD.DAT", facGet, shrAll, false, 0)

	a.get()

	if _, sts := b.get(); sts != rmsRecordLocked {
		t.Errorf("a second reader's default $GET: %#x, want RMS$_RLK", sts)
	}

	a.close()
	b.close()

	a.mustOpen("RD.DAT", facGet, shrAll, false, 0)
	b.mustOpen("RD.DAT", facGet, shrAll, false, 0)

	a.getWith(ropREA)

	if rec, sts := b.getWith(ropREA); rec != "R0" || sts != rmsNormal {
		t.Errorf("two read locks: %q, %#x", rec, sts)
	}

	if rec, sts := p[0].getWith(ropNLK); rec != "R1" || sts != rmsNormal {
		t.Errorf("A's query of a free record: %q, %#x", rec, sts)
	}

	a.close()
	b.close()
}

// TestRecordLock_errorUnlocks: a refused $GET unlocks the stream's
// record, unless ULK.
func TestRecordLock_errorUnlocks(t *testing.T) {
	p, mounts, _ := newLockingSharers(t, 3)
	a, b, c := p[0], p[1], p[2]

	writeFile(t, mounts, "E.DAT", "R0", "R1")

	a.mustOpen("E.DAT", facGet, shrAll, false, 0)
	b.mustOpen("E.DAT", facGet, shrAll, false, 0)
	c.mustOpen("E.DAT", facGet, shrAll, false, 0)

	b.getWith(ropRRL) // B is past R0, unlocked
	b.get()           // B holds R1
	a.get()           // A holds R0

	// A's next $GET (R1) is refused: A's R0 goes too.
	if _, sts := a.get(); sts != rmsRecordLocked {
		t.Fatalf("A's $GET of B's record: %#x", sts)
	}

	if rec, sts := c.get(); rec != "R0" || sts != rmsNormal {
		t.Errorf("C after A's error: %q, %#x; want R0 free", rec, sts)
	}

	a.close()
	b.close()
	c.close()
}
