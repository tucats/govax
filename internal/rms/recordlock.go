package rms

import (
	"encoding/binary"
	"errors"

	"github.com/tucats/govax/internal/lck"
)

// Record locks (docs/PHASE-47.md, subtask 6): in a file several
// processes may write, RMS locks each record a stream reads, so another
// stream can't take it from under it; the record-processing options
// (RAB$L_ROP) say how. They're locks in the system's lock database
// (internal/lck), as VMS's RMS takes them in VMS's lock manager.
//
// The rules are the Guide to OpenVMS File Applications' (section 7.2)
// and the RMS manual's RAB$L_ROP options:
//
//   - A stream locks records when its opener lets others write the file
//     (FAB$B_SHR with PUT, UPD, or DEL), and not with FAB$V_UPI, which
//     turns RMS's locking off.
//   - $GET locks the record it returns, and unlocks the one before. With
//     RAB$V_ULK (manual unlocking) locks stay until $FREE, $RELEASE, or
//     the file's close, through errors too; otherwise an error (a record
//     locked by another stream) unlocks the stream's record as well.
//   - The lock is exclusive by default, for every stream, reading or not
//     (EX: no other stream may lock the record); RAB$V_RLK write-locks it
//     (PW: others may still read it with no lock), RAB$V_REA read-locks it
//     (PR: others may read-lock it too). The guide's Table 7-6 is the
//     lock modes' compatibility.
//   - RAB$V_NLK (no lock) takes a query: a CR lock taken and dropped at
//     once. Refused (an exclusive lock), the $GET fails with RMS$_RLK;
//     granted while another stream holds a write or read lock, it returns
//     RMS$_OK_RLK.
//   - A lock refused is RMS$_RLK, and the record isn't consumed: the
//     next $GET tries it again. With RAB$V_RRL the record is returned
//     without a lock (RMS$_SUC, as for any sequential file); with
//     RAB$V_WAT the $GET waits for the lock, and then returns
//     RMS$_OK_WAT; with RAB$V_TMO too, for at most RAB$B_TMO seconds,
//     and then it fails with RMS$_TMO (Phase 49). Locking a record the
//     stream already holds is RMS$_OK_ALK.
//
// A record is named by its record's file address (RFA): its virtual
// block and the offset in it of its first byte. Each stream's record
// locks are sublocks of a NL lock on the file, named by the volume and
// the file ID, system-wide and in executive mode, so image rundown
// leaves them to the file's close.

// The RAB$L_ROP record locking options.
var (
	ropRLK = vmsConst("RAB$M_RLK")
	ropREA = vmsConst("RAB$M_REA")
	ropNLK = vmsConst("RAB$M_NLK")
	ropRRL = vmsConst("RAB$M_RRL")
	ropULK = vmsConst("RAB$M_ULK")
	ropWAT = vmsConst("RAB$M_WAT")
)

// rmsLockMode is the access mode of RMS's locks: executive.
const rmsLockMode = 1

// rfa is a record's file address: its block and the offset in it.
type rfa struct {
	vbn    uint32
	offset uint16
}

// rfaAt is the RFA of the record starting at byte offset off.
func rfaAt(off int64) rfa {
	return rfa{vbn: uint32(off/512) + 1, offset: uint16(off % 512)}
}

// name is the record's lock name: the RFA's six bytes.
func (r rfa) name() string {
	b := make([]byte, 6)
	binary.LittleEndian.PutUint32(b, r.vbn)
	binary.LittleEndian.PutUint16(b[4:], r.offset)

	return string(b)
}

// storeRFA writes r to the RAB's RAB$W_RFA: the block in its first
// longword, the offset in the word after.
func (ctx *Context) storeRFA(rabAddr uint32, r rfa) error {
	if err := ctx.storeLongword(rabAddr+rabRFA, r.vbn); err != nil {
		return err
	}

	return ctx.storeWord(rabAddr+rabRFA+4, r.offset)
}

// loadRFA reads the RAB's RAB$W_RFA.
func (ctx *Context) loadRFA(rabAddr uint32) (rfa, error) {
	vbn, err := ctx.loadLongword(rabAddr + rabRFA)
	if err != nil {
		return rfa{}, err
	}

	off, err := ctx.loadWord(rabAddr + rabRFA + 4)

	return rfa{vbn: vbn, offset: off}, err
}

// streamLocks is a stream's record locks: the NL lock on its file, the
// record locks under it by RFA, and a lock it waits for (RAB$V_WAT) and
// the record that lock is for.
type streamLocks struct {
	mgr     *lck.Manager
	file    *lck.Lock
	records map[rfa]*lck.Lock

	pendingAt rfa
	waiting   *lck.Lock

	// deadline is when the wait for waiting runs out (RAB$V_TMO), in
	// system time; 0 for no limit.
	deadline uint64
}

// locksRecords reports whether handle's stream takes record locks.
func (ctx *Context) locksRecords(handle *FileHandle) bool {
	return ctx.Locks != nil && handle.Accessor != nil && sharingOps(handle.Access, handle.Share)&opWrites != 0 &&
		handle.Share&shrUPI == 0
}

// fileLock returns handle's NL lock on its file, taking it the first
// time.
func (ctx *Context) fileLock(handle *FileHandle) (*lck.Lock, error) {
	s := &handle.locks
	if s.file != nil {
		return s.file, nil
	}

	fid := handle.File.Header.Fid
	name := make([]byte, 0, lck.MaxNameLength)
	name = append(name, "RMS$"...)
	name = binary.LittleEndian.AppendUint16(name, fid.Num)
	name = binary.LittleEndian.AppendUint16(name, fid.Seq)
	name = append(name, fid.Rvn, fid.Nmx)
	name = append(name, handle.File.Device.Home.VolumeName...)

	l, _, err := ctx.Locks.Enqueue(lck.Request{
		Owner: lck.Owner(ctx.PID), Mode: lck.NL, Name: string(name[:min(len(name), lck.MaxNameLength)]),
		AccessMode: rmsLockMode,
	})
	if err != nil {
		return nil, err
	}

	s.mgr, s.file = ctx.Locks, l

	return l, nil
}

// recordLockMode is the mode $GET locks a record in, from RAB$L_ROP:
// exclusive unless RLK (write lock) or REA (read lock; RLK wins if both).
func recordLockMode(rop uint32) lck.Mode {
	switch {
	case rop&ropRLK != 0:
		return lck.PW
	case rop&ropREA != 0:
		return lck.PR
	}

	return lck.EX
}

// lockResult is what locking a record came to.
type lockResult int

const (
	lockHeld    lockResult = iota // the record is locked for the stream
	lockAlready                   // the stream had it locked already
	lockNone                      // returned without a lock (NLK, RRL)
	lockRefused                   // RMS$_RLK
	lockWait                      // waiting for it (WAT)
)

// lockRecord locks the record at r for handle's stream as rop says,
// returning the result and, for a query, whether another stream holds
// the record (RMS$_OK_RLK).
func (ctx *Context) lockRecord(handle *FileHandle, r rfa, rop uint32) (lockResult, bool, error) {
	s := &handle.locks
	if s.records[r] != nil {
		return lockAlready, false, nil
	}

	parent, err := ctx.fileLock(handle)
	if err != nil {
		return 0, false, err
	}

	owner := lck.Owner(ctx.PID)
	req := lck.Request{Owner: owner, Name: r.name(), AccessMode: rmsLockMode, Parent: parent.ID, NoQueue: true}

	if rop&ropNLK != 0 {
		req.Mode = lck.CR

		l, _, err := ctx.Locks.Enqueue(req)
		if errors.Is(err, lck.ErrNotQueued) {
			if rop&ropRRL != 0 {
				return lockNone, false, nil
			}

			return lockRefused, false, nil
		}

		if err != nil {
			return 0, false, err
		}

		held := len(l.Resource.Locks()) > 1
		events, _ := ctx.Locks.Dequeue(owner, l.ID, lck.DequeueOptions{})
		lck.Deliver(events)

		return lockNone, held, nil
	}

	req.Mode = recordLockMode(rop)
	req.NoQueue = rop&ropWAT == 0
	req.Data = ctx.Waker

	l, events, err := ctx.Locks.Enqueue(req)
	lck.Deliver(events)

	switch {
	case errors.Is(err, lck.ErrNotQueued):
		if rop&ropRRL != 0 {
			return lockNone, false, nil
		}

		return lockRefused, false, nil
	case err != nil:
		return 0, false, err
	case l.State != lck.Granted:
		s.waiting = l

		return lockWait, false, nil
	}

	ctx.holdRecord(handle, r, l)

	return lockHeld, false, nil
}

// holdRecord records l as the stream's lock on r.
func (ctx *Context) holdRecord(handle *FileHandle, r rfa, l *lck.Lock) {
	s := &handle.locks
	if s.records == nil {
		s.records = map[rfa]*lck.Lock{}
	}

	s.records[r] = l
}

// unlockRecords drops the stream's record locks, but for keep's
// (nil: all of them), reporting how many it dropped.
func (ctx *Context) unlockRecords(handle *FileHandle, keep *rfa) int {
	s := &handle.locks
	n := 0

	for r, l := range s.records {
		if keep != nil && r == *keep {
			continue
		}

		events, _ := s.mgr.Dequeue(l.Owner, l.ID, lck.DequeueOptions{})
		lck.Deliver(events)
		delete(s.records, r)

		n++
	}

	return n
}

// release drops every lock the stream has, its file lock and a lock it
// waits for too: at $CLOSE, and at rundown.
func (s *streamLocks) release() {
	if s.file == nil {
		return
	}

	drop := func(l *lck.Lock) {
		events, _ := s.mgr.Dequeue(l.Owner, l.ID, lck.DequeueOptions{})
		lck.Deliver(events)
	}

	if s.waiting != nil {
		drop(s.waiting)
	}

	for _, l := range s.records {
		drop(l)
	}

	drop(s.file)

	*s = streamLocks{}
}

// dropWaiting gives up the record lock the stream waits for.
func (ctx *Context) dropWaiting(h *FileHandle) {
	ls := &h.locks

	events, _ := ls.mgr.Dequeue(ls.waiting.Owner, ls.waiting.ID, lck.DequeueOptions{})
	lck.Deliver(events)
	ls.waiting, ls.deadline = nil, 0
}

// awaitLock waits for the stream's lock l (RAB$V_WAT): the service is
// called again when it's granted, or when the wait's time limit, if it
// has one, runs out.
func (ctx *Context) awaitLock(h *FileHandle, l *lck.Lock) error {
	if ctx.AwaitLock == nil {
		return errors.New("rms: a record lock wait with no way to wait")
	}

	return ctx.AwaitLock(func() bool { return l.State == lck.Granted }, h.locks.deadline)
}

// The RAB fields of a record lock wait's time limit: RAB$V_TMO asks for
// one, RAB$B_TMO is its length in seconds (0 to 255; RMS manual).
var (
	ropTMO = vmsConst("RAB$M_TMO")
	rabTMO = rabOffset("TMO")
)

// lockDeadline is when a record lock wait starting now runs out, by the
// RAB at rabAddr's RAB$V_TMO and RAB$B_TMO; 0 for no limit (no TMO, or
// no Clock to measure one by).
func (ctx *Context) lockDeadline(rabAddr, rop uint32) (uint64, error) {
	if rop&ropTMO == 0 || ctx.Clock == nil {
		return 0, nil
	}

	seconds, err := ctx.loadByte(rabAddr + rabTMO)
	if err != nil {
		return 0, err
	}

	// A deadline of 0 would mean none: a wait of 0 seconds ends a tick
	// after it starts, as good as at once.
	return ctx.Clock() + uint64(seconds)*10_000_000 + 1, nil
}

// SysFree implements SYS$FREE (RAB at argv[0]): every record the stream
// has locked is unlocked; RMS$_RNL if none was.
func SysFree(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	handle, sts, err := ctx.streamOf(rabAddr)
	if err != nil || sts != 0 {
		return sts, err
	}

	if ctx.Locks == nil || ctx.unlockRecords(handle, nil) == 0 {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsRecordNotLocked)
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// SysRelease implements SYS$RELEASE (RAB at argv[0]): the record whose
// RFA is in RAB$W_RFA is unlocked; RMS$_RNL if the stream hadn't it
// locked.
func SysRelease(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	handle, sts, err := ctx.streamOf(rabAddr)
	if err != nil || sts != 0 {
		return sts, err
	}

	r, err := ctx.loadRFA(rabAddr)
	if err != nil {
		return 0, err
	}

	l := handle.locks.records[r]
	if l == nil {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsRecordNotLocked)
	}

	events, _ := ctx.Locks.Dequeue(l.Owner, l.ID, lck.DequeueOptions{})
	lck.Deliver(events)
	delete(handle.locks.records, r)

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// streamOf returns the open stream the RAB at rabAddr is connected to,
// or a nonzero status (RMS$_ISI, stored in the RAB) if none.
func (ctx *Context) streamOf(rabAddr uint32) (*FileHandle, uint32, error) {
	isi, err := ctx.loadWord(rabAddr + rabISI)
	if err != nil {
		return nil, 0, err
	}

	handle, ok := ctx.Files.Lookup(isi)
	if !ok {
		sts, err := storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsInvalidISI)

		return nil, sts, err
	}

	return handle, 0, nil
}
