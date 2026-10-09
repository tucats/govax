package rms

import (
	"testing"

	"github.com/tucats/govax/internal/lck"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// Tests of the record operations past $GET and $PUT and the stream
// context they share (stream.go, recordops.go; docs/PHASE-49.md).

// call runs one RAB service with RAB$L_ROP rop, returning its status.
func (s *sharer) call(service func(*Context, []uint32) (uint32, error), rop uint32) uint32 {
	s.t.Helper()
	putLongwordAt(s.t, s.ctx, testRabAddr+rabROP, rop)

	r0, err := service(s.ctx, []uint32{testRabAddr})
	if err != nil {
		s.t.Fatalf("%s: %v", s.name, err)
	}

	return r0
}

// putStatus writes one record with RAB$L_ROP rop, returning $PUT's
// status.
func (s *sharer) putStatus(record string, rop uint32) uint32 {
	s.t.Helper()
	putRecord(s.t, s.ctx, testRabAddr, []byte(record))

	return s.call(SysPut, rop)
}

// update replaces the current record with record, returning $UPDATE's
// status.
func (s *sharer) update(record string) uint32 {
	s.t.Helper()
	putRecord(s.t, s.ctx, testRabAddr, []byte(record))

	return s.call(SysUpdate, 0)
}

// byRFA sets RAB$B_RAC to RFA access, and RAB$W_RFA to r.
func (s *sharer) byRFA(r rfa) {
	s.t.Helper()
	putByte(s.t, s.ctx, testRabAddr+rabRAC, racRFA)

	if err := s.ctx.storeRFA(testRabAddr, r); err != nil {
		s.t.Fatal(err)
	}
}

// sequential sets RAB$B_RAC back to sequential access.
func (s *sharer) sequential() {
	s.t.Helper()
	putByte(s.t, s.ctx, testRabAddr+rabRAC, racSeq)
}

// wantGet reads a record, failing the test unless it is want with
// status sts.
func (s *sharer) wantGet(want string, sts uint32) {
	s.t.Helper()

	if rec, got := s.get(); rec != want || got != sts {
		s.t.Errorf("%s: $GET = %q, %#x; want %q, %#x", s.name, rec, got, want, sts)
	}
}

// wantRecStatus fails the test unless got is want.
func wantRecStatus(t *testing.T, what string, got, want uint32) {
	t.Helper()

	if got != want {
		t.Errorf("%s = %#x, want %#x", what, got, want)
	}
}

// TestFind_sequential: a sequential $FIND skips a record, making it the
// current one; a $GET right after it reads that record; RAB$W_RFA is the
// record's.
func TestFind_sequential(t *testing.T) {
	p, mounts := newSharers(t, 1)
	a := p[0]

	writeFile(t, mounts, "F.DAT", "R0", "R1", "R2", "R3")
	a.mustOpen("F.DAT", facGet, 0, false, 0)

	wantRecStatus(t, "$FIND", a.call(SysFind, 0), rmsNormal)

	if got := a.rabRFAOf(); got != (rfa{1, 0}) {
		t.Errorf("RFA after $FIND %+v, want block 1 offset 0", got)
	}

	a.wantGet("R0", rmsNormal) // the record $FIND found
	a.wantGet("R1", rmsNormal)
	wantRecStatus(t, "$FIND", a.call(SysFind, 0), rmsNormal) // R2
	wantRecStatus(t, "$FIND", a.call(SysFind, 0), rmsNormal) // R3
	a.wantGet("R3", rmsNormal)
	a.wantGet("", rmsEOF)
	wantRecStatus(t, "$FIND at the end", a.call(SysFind, 0), rmsEOF)

	a.close()
}

// TestFind_byRFA: $FIND by RFA makes a record current without moving
// the next record (but a sequential $GET after it reads the found record
// and goes on from there); $GET by RFA moves the next record past it; an
// RFA that names no record is RMS$_RFA.
func TestFind_byRFA(t *testing.T) {
	p, mounts := newSharers(t, 1)
	a := p[0]

	writeFile(t, mounts, "R.DAT", "R0", "R1", "R2", "R3")
	a.mustOpen("R.DAT", facGet, 0, false, 0)

	a.wantGet("R0", rmsNormal)
	a.wantGet("R1", rmsNormal)
	r1 := a.rabRFAOf()
	a.wantGet("R2", rmsNormal)

	a.byRFA(r1)
	wantRecStatus(t, "$FIND by RFA", a.call(SysFind, 0), rmsNormal)
	a.sequential()
	a.wantGet("R1", rmsNormal) // the current record, after a $FIND
	a.wantGet("R2", rmsNormal)

	a.byRFA(rfa{1, 0})
	a.wantGet("R0", rmsNormal)
	a.sequential()
	a.wantGet("R1", rmsNormal)

	for _, bad := range []rfa{{1, 1}, {9, 0}, {0, 0}, {1, 600}} {
		a.byRFA(bad)
		wantRecStatus(t, "$FIND of a bad RFA", a.call(SysFind, 0), rmsInvalidRFA)
	}

	a.close()
}

// TestUpdate: $UPDATE replaces the current record in place, the same
// length only; it needs a current record and FAB$V_UPD access.
func TestUpdate(t *testing.T) {
	p, mounts := newSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "U.DAT", "R0", "R1", "R2")
	a.mustOpen("U.DAT", facGet|facUpd, 0, false, 0)

	wantRecStatus(t, "$UPDATE before any $GET", a.update("XX"), rmsNoCurrent)

	a.wantGet("R0", rmsNormal)
	wantRecStatus(t, "$UPDATE", a.update("U0"), rmsNormal)

	if got := a.rabRFAOf(); got != (rfa{1, 0}) {
		t.Errorf("RFA after $UPDATE %+v, want block 1 offset 0", got)
	}

	wantRecStatus(t, "a second $UPDATE", a.update("V0"), rmsNoCurrent)

	wantRecStatus(t, "$FIND", a.call(SysFind, 0), rmsNormal) // R1
	wantRecStatus(t, "$UPDATE of another length", a.update("LONGER"), rmsRecordTooBig)
	wantRecStatus(t, "$UPDATE after a failure", a.update("U1"), rmsNoCurrent)

	// The next record didn't move: R2.
	a.wantGet("R2", rmsNormal)
	wantRecStatus(t, "$UPDATE of the last record", a.update("U2"), rmsNormal)
	a.close()

	wantRecords(t, records(t, mounts, "U.DAT"), "U0", "R1", "U2")

	b.mustOpen("U.DAT", facGet, 0, false, 0)
	b.wantGet("U0", rmsNormal)
	wantRecStatus(t, "$UPDATE without UPD access", b.update("Z0"), rmsFACNotAllowed)
	b.close()
	checkVolume(t, mounts)
}

// TestTruncate: $TRUNCATE ends the file at the current record (a file
// of several blocks cut off in its second), and $PUT appends there; it
// needs FAB$V_TRN access and a current record.
func TestTruncate(t *testing.T) {
	p, mounts := newSharers(t, 2)
	a, b := p[0], p[1]

	recs := numbered("R", 40, 30)
	writeFile(t, mounts, "T.DAT", recs...)

	b.mustOpen("T.DAT", facGet|facPut, 0, false, 0)
	b.wantGet(recs[0], rmsNormal)
	wantRecStatus(t, "$TRUNCATE without TRN access", b.call(SysTruncate, 0), rmsFACNotAllowed)
	b.close()

	a.mustOpen("T.DAT", facGet|facPut|facTrn, 0, false, 0)
	wantRecStatus(t, "$TRUNCATE before any $GET", a.call(SysTruncate, 0), rmsNoCurrent)

	for i := range 20 {
		a.wantGet(recs[i], rmsNormal)
	}

	wantRecStatus(t, "$TRUNCATE", a.call(SysTruncate, 0), rmsNormal)
	wantRecStatus(t, "a second $TRUNCATE", a.call(SysTruncate, 0), rmsNoCurrent)
	a.wantGet("", rmsEOF)
	a.put("NEW19")
	a.put("NEW20")
	a.close()

	wantRecords(t, records(t, mounts, "T.DAT"), append(recs[:19:19], "NEW19", "NEW20")...)
	checkVolume(t, mounts)
}

// TestTruncate_trnOnly: a FAB whose only access is TRN opens, reads, and
// truncates (TRN is write access: the new end of file is written back
// at $CLOSE); on a volume mounted read-only it is RMS$_PRV.
func TestTruncate_trnOnly(t *testing.T) {
	path := newTestVolumeFile(t, "TRN")

	newSharer := func(writable bool) (*sharer, *MountTable) {
		mounts := NewMountTable()
		if err := mounts.Mount("DUA0", path, writable); err != nil {
			t.Fatal(err)
		}

		return &sharer{t: t, name: "A", ctx: &Context{
			Mem: vm.NewMemory(1 << 20), CPU: vax.New(), Mounts: mounts,
			Files: NewFileTable(nil), Logicals: newTestLogicals(t),
		}}, mounts
	}

	a, mounts := newSharer(true)
	writeFile(t, mounts, "T.DAT", "ONE", "TWO", "THREE")

	a.mustOpen("T.DAT", facTrn, 0, false, 0)
	a.wantGet("ONE", rmsNormal)
	a.wantGet("TWO", rmsNormal)
	wantRecStatus(t, "$TRUNCATE", a.call(SysTruncate, 0), rmsNormal)
	a.close()

	wantRecords(t, records(t, mounts, "T.DAT"), "ONE")
	checkVolume(t, mounts)

	if err := mounts.Dismount("DUA0"); err != nil {
		t.Fatal(err)
	}

	ro, _ := newSharer(false)
	wantRecStatus(t, "$OPEN FAC=TRN, read-only volume", ro.open("T.DAT", facTrn, 0, false, 0), rmsPrivilegeViolation)
}

// TestPut_truncateOnPut: a $PUT away from the end of the file is
// RMS$_NEF; with RAB$V_TPT it goes at the next record, the file ending
// after it (RMS$_FAC without FAB$V_TRN).
func TestPut_truncateOnPut(t *testing.T) {
	p, mounts := newSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "P.DAT", "R0", "R1", "R2")

	b.mustOpen("P.DAT", facGet|facPut, 0, false, 0)
	b.wantGet("R0", rmsNormal)
	wantRecStatus(t, "$PUT in the middle", b.putStatus("T1", 0), rmsNotAtEOF)
	wantRecStatus(t, "$PUT with TPT, without TRN", b.putStatus("T1", ropTPT), rmsFACNotAllowed)
	b.close()

	a.mustOpen("P.DAT", facGet|facPut|facTrn, 0, false, 0)
	a.wantGet("R0", rmsNormal)
	wantRecStatus(t, "$PUT with TPT", a.putStatus("T1", ropTPT), rmsNormal)

	if got := a.rabRFAOf(); got != (rfa{1, 4}) {
		t.Errorf("RFA after the $PUT %+v, want block 1 offset 4 (R1's)", got)
	}

	a.put("T2")
	a.wantGet("", rmsEOF)
	a.close()

	wantRecords(t, records(t, mounts, "P.DAT"), "R0", "T1", "T2")
	checkVolume(t, mounts)
}

// TestStream_getAndPut: one stream reading and writing (FAB$B_FAC GET
// and PUT): $PUT after reading to the end of the file appends, $REWIND
// goes back to the first record, and the stream reads what it wrote.
func TestStream_getAndPut(t *testing.T) {
	p, mounts := newSharers(t, 1)
	a := p[0]

	writeFile(t, mounts, "GP.DAT", "R0", "R1")
	a.mustOpen("GP.DAT", facGet|facPut, 0, false, 0)

	a.wantGet("R0", rmsNormal)
	a.wantGet("R1", rmsNormal)
	a.wantGet("", rmsEOF)
	a.put("R2")
	a.wantGet("", rmsEOF) // the next record is the end of the file

	wantRecStatus(t, "$REWIND", a.call(SysRewind, 0), rmsNormal)
	a.wantGet("R0", rmsNormal)
	a.wantGet("R1", rmsNormal)
	a.wantGet("R2", rmsNormal)
	a.close()

	wantRecords(t, records(t, mounts, "GP.DAT"), "R0", "R1", "R2")
	checkVolume(t, mounts)
}

// TestDelete_sequential: a sequential file's records can't be deleted:
// RMS$_IOP.
func TestDelete_sequential(t *testing.T) {
	p, mounts := newSharers(t, 1)
	a := p[0]

	writeFile(t, mounts, "D.DAT", "R0")
	a.mustOpen("D.DAT", facGet|facDel, 0, false, 0)
	a.wantGet("R0", rmsNormal)
	wantRecStatus(t, "$DELETE", a.call(SysDelete, 0), rmsIOP)
	a.close()
}

// TestUpdate_locked: in a file shared for writing, $UPDATE needs the
// record locked by the stream (RMS$_RNL after a $GET with no lock), and
// a stream that waited for the record reads it as updated.
func TestUpdate_locked(t *testing.T) {
	p, mounts, waits := newLockingSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "L.DAT", "R0", "R1")
	a.mustOpen("L.DAT", facGet|facUpd, shrAll, false, 0)
	b.mustOpen("L.DAT", facGet|facUpd, shrAll, false, 0)

	a.wantGet("R0", rmsNormal)

	// B waits for R0 (RAB$V_WAT).
	putLongwordAt(t, b.ctx, testRabAddr+rabROP, ropWAT)
	putLongwordAt(t, b.ctx, testRabAddr+rabUBF, testRecordAddr)
	putWord(t, b.ctx, testRabAddr+rabUSZ, 1024)

	if _, err := SysGet(b.ctx, []uint32{testRabAddr}); err != errTestWait {
		t.Fatalf("B's $GET: %v, want a wait", err)
	}

	wantRecStatus(t, "A's $UPDATE", a.update("U0"), rmsNormal)

	if waits[1]() {
		t.Fatal("B's wait is over before A unlocked R0")
	}

	wantRecStatus(t, "A's $FREE", a.call(SysFree, 0), rmsNormal)

	if !waits[1]() {
		t.Fatal("B's wait isn't over after A's $FREE")
	}

	b.wantGet("U0", rmsOKWaited)
	b.close()

	// A query lock is no lock: $UPDATE is refused.
	a.sequential()
	a.wantGet("R1", rmsNormal)
	wantRecStatus(t, "$FREE", a.call(SysFree, 0), rmsNormal)
	putLongwordAt(t, a.ctx, testRabAddr+rabROP, ropNLK)
	wantRecStatus(t, "$REWIND", a.call(SysRewind, ropNLK), rmsNormal)
	a.wantGet("U0", rmsNormal)
	wantRecStatus(t, "$UPDATE of an unlocked record", a.update("V0"), rmsRecordNotLocked)
	a.close()

	wantRecords(t, records(t, mounts, "L.DAT"), "U0", "R1")
}

// TestRecordLock_timeout: a $GET waiting for a record with RAB$V_TMO
// gives up when RAB$B_TMO's seconds have passed: RMS$_TMO, the record
// still next; a wait whose lock comes in time succeeds as before.
func TestRecordLock_timeout(t *testing.T) {
	p, mounts, _ := newLockingSharers(t, 2)
	a, b := p[0], p[1]

	var now uint64 = 1_000_000_000

	var deadlines []uint64

	b.ctx.Clock = func() uint64 { return now }
	b.ctx.AwaitLock = func(_ func() bool, deadline uint64) error {
		deadlines = append(deadlines, deadline)

		return errTestWait
	}

	writeFile(t, mounts, "TMO.DAT", "R0", "R1")
	a.mustOpen("TMO.DAT", facGet|facUpd, shrAll, false, 0)
	b.mustOpen("TMO.DAT", facGet|facUpd, shrAll, false, 0)

	a.wantGet("R0", rmsNormal)

	putByte(t, b.ctx, testRabAddr+rabTMO, 5)
	putLongwordAt(t, b.ctx, testRabAddr+rabROP, ropWAT|ropTMO)
	putLongwordAt(t, b.ctx, testRabAddr+rabUBF, testRecordAddr)
	putWord(t, b.ctx, testRabAddr+rabUSZ, 1024)

	get := func() (uint32, error) { return SysGet(b.ctx, []uint32{testRabAddr}) }

	if _, err := get(); err != errTestWait {
		t.Fatalf("B's $GET: %v, want a wait", err)
	}

	if want := now + 5*10_000_000 + 1; len(deadlines) != 1 || deadlines[0] != want {
		t.Fatalf("deadlines %v, want [%d]", deadlines, want)
	}

	now += 4 * 10_000_000

	if _, err := get(); err != errTestWait {
		t.Fatalf("B's $GET after 4 seconds: %v, want a wait", err)
	}

	now += 10_000_000 + 1

	if r0, err := get(); err != nil || r0 != rmsTimedOut {
		t.Fatalf("B's $GET after 5 seconds: %#x, %v; want RMS$_TMO", r0, err)
	}

	// The record is still B's next; this time A lets it go in time.
	if _, err := get(); err != errTestWait {
		t.Fatalf("B's second $GET: %v, want a wait", err)
	}

	wantRecStatus(t, "A's $FREE", a.call(SysFree, 0), rmsNormal)
	b.wantGet("R0", rmsOKWaited)

	// No TMO: no deadline.
	a.wantGet("R1", rmsNormal)
	putLongwordAt(t, b.ctx, testRabAddr+rabROP, ropWAT)

	if _, err := get(); err != errTestWait || deadlines[len(deadlines)-1] != 0 {
		t.Fatalf("B's $GET without TMO: %v, deadline %d; want a wait with none", err, deadlines[len(deadlines)-1])
	}

	a.close()
	b.close()
}

// TestRecordLock_quota: a record lock past the job's ENQLM quota is
// RMS$_EXENQLM.
func TestRecordLock_quota(t *testing.T) {
	p, mounts, _ := newLockingSharers(t, 1)
	a := p[0]

	writeFile(t, mounts, "Q.DAT", "R0")
	a.mustOpen("Q.DAT", facGet|facUpd, shrAll, false, 0)

	a.ctx.CanLock = func() bool { return false }
	a.wantGet("", rmsExEnqLm)

	a.ctx.CanLock = func() bool { return true }
	a.wantGet("R0", rmsNormal)
	a.close()
}

// TestRecordLock_deadlock: two streams each waiting (RAB$V_WAT) for the
// record the other holds; when the lock manager's search refuses one's
// wait, its $GET is RMS$_DEADLOCK.
func TestRecordLock_deadlock(t *testing.T) {
	p, mounts, _ := newLockingSharers(t, 2)
	a, b := p[0], p[1]

	now := uint64(1_000)
	locks := a.ctx.Locks
	locks.Now = func() uint64 { return now }
	locks.DeadlockWait = 100

	writeFile(t, mounts, "DL.DAT", "R0", "R1")
	a.mustOpen("DL.DAT", facGet|facUpd, shrAll, false, ropULK)
	b.mustOpen("DL.DAT", facGet|facUpd, shrAll, false, ropULK)

	a.wantGet("R0", rmsNormal)
	b.call(SysFind, ropULK) // B: R0 is A's
	b.byRFA(rfa{1, 4})
	b.wantGet("R1", rmsNormal)

	// A waits for R1 (B's), B for R0 (A's).
	a.byRFA(rfa{1, 4})
	b.byRFA(rfa{1, 0})

	for _, s := range []*sharer{a, b} {
		putLongwordAt(t, s.ctx, testRabAddr+rabROP, ropWAT|ropULK)
		putLongwordAt(t, s.ctx, testRabAddr+rabUBF, testRecordAddr)
		putWord(t, s.ctx, testRabAddr+rabUSZ, 1024)

		if _, err := SysGet(s.ctx, []uint32{testRabAddr}); err != errTestWait {
			t.Fatalf("%s's $GET: %v, want a wait", s.name, err)
		}
	}

	now += 100
	lck.Deliver(locks.CheckDeadlocks(now))

	if r0, err := SysGet(a.ctx, []uint32{testRabAddr}); err != nil || r0 != rmsDeadlock {
		t.Errorf("A's $GET after the search: %#x, %v; want RMS$_DEADLOCK", r0, err)
	}

	if _, err := SysGet(b.ctx, []uint32{testRabAddr}); err != errTestWait {
		t.Errorf("B's $GET: %v, still a wait (A holds R0)", err)
	}

	a.close()

	b.wantGet("R0", rmsOKWaited)
	b.close()
}
