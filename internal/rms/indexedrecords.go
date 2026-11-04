package rms

import (
	"bytes"
	"errors"

	"github.com/tucats/govax/internal/isam"
	"github.com/tucats/govax/internal/lck"
)

// The record operations on an indexed file (Phase 88, subtask 8; docs/PHASE-88
// - keyed index files.md): $GET, $FIND, $PUT, $UPDATE, $DELETE, and $REWIND, on
// the stream context below. The file's structure is internal/isam's; this file
// is what RMS adds to it: the RAB's fields, the statuses, the stream's place,
// and record locks. Each operation opens the file's prolog again (Decision 3),
// so a split another stream made since is seen, and nothing of the file is kept
// between services but the stream's place, which names a record by its key and
// RFA rather than by a bucket.
//
// The rules are VMS 7.1's, as testdata/idx88's cases 120 to 153 show them (the
// phase doc's "What the record services settled"):
//
//   - A stream follows one key of reference in sequence. $CONNECT and $REWIND
//     take it from RAB$B_KRF, a keyed $GET or $FIND changes it to the key it
//     searched, and an access by RFA to the primary key; a sequential $GET pays
//     no attention to RAB$B_KRF (case 125).
//   - A sequential $GET right after a $FIND reads the record the $FIND found,
//     and any successful $GET or $FIND moves the stream's place to its record.
//     A failed one leaves the place alone (case 131: a keyed $GET past the end,
//     then the record after the last one read).
//   - A failed $GET or $FIND leaves RAB$W_RFA 0 and a $GET RAB$W_RSZ 0; a failed
//     $PUT, $UPDATE, or $DELETE leaves both as they were; a successful $DELETE
//     leaves RFA 0 (cases 139, 143, 153).
//   - After a $DELETE the stream goes on from the record after the one deleted,
//     by the stream's key (case 146).

// The statuses of the record services on an indexed file, beside those the
// other organizations share.
var (
	rmsRecordDeleted    = vmsConst("RMS$_DEL")
	rmsInvalidKeyRef    = vmsConst("RMS$_KRF")
	rmsInvalidKeyBuffer = vmsConst("RMS$_KBF")
	rmsInvalidROP       = vmsConst("RMS$_ROP")
	rmsDuplicateKey     = vmsConst("RMS$_DUP")
	rmsOKDuplicate      = vmsConst("RMS$_OK_DUP")
	rmsOKLimit          = vmsConst("RMS$_OK_LIM")
	rmsKeyChanged       = vmsConst("RMS$_CHG")
	rmsOutOfSequence    = vmsConst("RMS$_SEQ")
)

// RAB$L_ROP's bits that only an indexed file reads.
var (
	ropKGE = vmsConst("RAB$M_KGE")
	ropKGT = vmsConst("RAB$M_KGT")
	ropREV = vmsConst("RAB$M_REV")
	ropLIM = vmsConst("RAB$M_LIM")
	ropCDK = vmsConst("RAB$M_CDK")
	ropLOA = vmsConst("RAB$M_LOA")
	ropFDL = vmsConst("RAB$M_FDL")

	rabKRF = rabOffset("KRF")
)

// indexedStream is a stream's context on an indexed file, beside the
// streamContext every stream has (whose fac it uses).
type indexedStream struct {
	// krf is the key of reference a sequential $GET or $FIND follows.
	krf int

	// place is the record the stream reached last, by krf's key: the next
	// record in sequence is the one after it. With hasPlace false the next
	// record is the key's first, and with atEnd ($CONNECT with RAB$V_EOF)
	// there is none.
	place    isam.Entry
	hasPlace bool
	atEnd    bool

	// queued, when set, is the next record itself, worked out before the
	// record at place was deleted: once a record has gone, isam can find
	// only what follows its key, which would pass over the rest of a set
	// of duplicates. queuedEnd says there's no next record.
	queued    *isam.Entry
	queuedEnd bool

	// current is the current record, if hasCurrent, and afterFind says the
	// last successful operation was a $FIND.
	current    isam.Entry
	hasCurrent bool
	afterFind  bool

	// lastPut is the primary key of the stream's last $PUT in sequence (or
	// the file's last, after $CONNECT with RAB$V_EOF): a sequential $PUT of
	// a lower key is RMS$_SEQ.
	lastPut []byte
}

// lost is the context after a failed operation, and after the operations
// that leave no current record.
func (s *indexedStream) lost() {
	s.hasCurrent, s.afterFind = false, false
}

// moveTo makes e the stream's place and key of reference.
func (s *indexedStream) moveTo(e isam.Entry) {
	s.krf, s.place, s.hasPlace, s.atEnd = e.Ref, e, true, false
	s.queued, s.queuedEnd = nil, false
}

// rewind puts the stream's place before key krf's first record.
func (s *indexedStream) rewind(krf int) {
	s.krf, s.hasPlace, s.atEnd, s.afterFind = krf, false, false, false
	s.queued, s.queuedEnd = nil, false
}

// rmsRFA is an isam RFA as RMS keeps one: the bucket's VBN, and the record's
// ID in RAB$W_RFA's word.
func rmsRFA(r isam.RFA) rfa { return rfa{vbn: r.VBN, offset: r.ID} }

// isamRFA is the other way round.
func isamRFA(r rfa) isam.RFA { return isam.RFA{VBN: r.vbn, ID: r.offset} }

// indexedFile opens h's file's prolog for one operation: for writing if the
// stream may change records, so a search can also take out a pointer a fast
// delete left (findLive).
func (h *FileHandle) indexedFile() (*isam.File, error) {
	return openIndexed(h.File, h.stream.fac&(facPut|facUpd|facDel) != 0)
}

// connectIndexed sets an indexed stream's context as $CONNECT leaves it: at
// RAB$B_KRF's first record, or with RAB$V_EOF past the last, so that only a
// $PUT of a higher primary key may follow in sequence. VMS 7.1 took EOF with
// key 1 too (case 137), where the manual says RMS$_ROP; and a key of
// reference past the file's is RMS$_KRF here, unconfirmed.
func (ctx *Context) connectIndexed(rabAddr uint32, h *FileHandle, rop uint32) (uint32, error) {
	krf, err := ctx.loadByte(rabAddr + rabKRF)
	if err != nil {
		return 0, err
	}

	f, err := h.indexedFile()
	if err != nil {
		return prologStatus(err), nil
	}

	if int(krf) >= len(f.Keys) {
		return rmsInvalidKeyRef, nil
	}

	h.idx = indexedStream{krf: int(krf)}

	if rop&ropEOF != 0 {
		h.idx.atEnd = true

		last, err := lastPrimaryKey(f)
		if err != nil {
			return rmsDeviceError, nil
		}

		h.idx.lastPut = last
	}

	return 0, nil
}

// lastPrimaryKey is the highest primary key in f, or nil for an empty file.
func lastPrimaryKey(f *isam.File) ([]byte, error) {
	var last []byte

	err := f.Walk(0, func(e isam.Entry) bool {
		last = e.Key

		return true
	})

	return last, err
}

// keyMatch is RAB$L_ROP's KGE, KGT, and REV as an isam search: RMS$_ROP for
// REV alone and for KGE with KGT, with or without REV (VMS 7.1: cases 120 and
// 121, the RMS manual's Table 7-4).
func keyMatch(rop uint32) (isam.Match, bool) {
	kge, kgt, rev := rop&ropKGE != 0, rop&ropKGT != 0, rop&ropREV != 0

	switch {
	case kge && kgt, rev && !kge && !kgt:
		return 0, false
	case rev && kge:
		return isam.EqualOrPrevious, true
	case rev:
		return isam.Previous, true
	case kge:
		return isam.EqualOrNext, true
	case kgt:
		return isam.Next, true
	}

	return isam.Equal, true
}

// searchValue reads the key a keyed operation gives, RAB$B_KSZ bytes at
// RAB$L_KBF, for key k: a string key's may be shorter than the key, a generic
// search, and any other key's must be the whole key, a KSZ of 0 meaning that.
// A string key with KSZ 0, or a size the key can't take, is RMS$_KSZ, and a
// buffer that can't be read RMS$_KBF (VMS 7.1: case 128).
func (ctx *Context) searchValue(rabAddr uint32, k *isam.Key) ([]byte, uint32, error) {
	ksz, err := ctx.loadByte(rabAddr + rabKSZ)
	if err != nil {
		return nil, 0, err
	}

	size := int(ksz)
	if size == 0 && k.Type.Kind() != isam.String {
		size = k.Size
	}

	switch {
	case size == 0 || size > k.Size:
		return nil, rmsInvalidKeySize, nil
	case k.Type.Kind() != isam.String && size != k.Size:
		return nil, rmsInvalidKeySize, nil
	}

	kbf, err := ctx.loadLongword(rabAddr + rabKBF)
	if err != nil {
		return nil, 0, err
	}

	value, err := ctx.loadBytes(kbf, size)
	if err != nil {
		return nil, rmsInvalidKeyBuffer, nil
	}

	return value, 0, nil
}

// findLive is a keyed search that passes over a pointer whose record has gone,
// which ROP FDL leaves in an alternate index: the pointer is taken out, as the
// Guide's 8.2.4 says RMS must (U134: the record was RMS$_RNF by each alternate
// afterwards), and the search made again. It returns the entry and its record.
func findLive(f *isam.File, ref int, value []byte, match isam.Match) (isam.Entry, []byte, error) {
	for {
		e, err := f.Find(ref, value, match)
		if err != nil {
			return isam.Entry{}, nil, err
		}

		record, err := f.Record(e.RFA)
		if !errors.Is(err, isam.ErrDeleted) || ref == 0 {
			return e, record, err
		}

		if err := f.DropPointer(e); err != nil {
			return isam.Entry{}, nil, err
		}
	}
}

// nextLive is the entry after at in its key's order whose record is still
// there, taking out the pointers it passes whose records have gone. ErrEnd
// says there's none.
func nextLive(f *isam.File, at isam.Entry, first bool) (isam.Entry, []byte, error) {
	var (
		e   isam.Entry
		err error
	)

	if first {
		e, err = f.First(at.Ref)
	} else {
		e, err = f.Following(at)
	}

	for err == nil {
		record, rerr := f.Record(e.RFA)
		if !errors.Is(rerr, isam.ErrDeleted) || e.Ref == 0 {
			return e, record, rerr
		}

		// The one after it first: once the pointer is out, Following could
		// find only what follows its key, passing over its duplicates.
		next, nerr := f.Following(e)

		if derr := f.DropPointer(e); derr != nil && !errors.Is(derr, isam.ErrReadOnly) {
			return isam.Entry{}, nil, derr
		}

		e, err = next, nerr
	}

	return isam.Entry{}, nil, err
}

// indexedLocate is $GET's and $FIND's work on an indexed file (find says
// which): the record by RAB$B_RAC (the next in sequence by the stream's key,
// one found by RAB$L_KBF's key, or the one RAB$W_RFA names), locked if the
// stream locks records, with the context moved on; a $GET returns it.
func (ctx *Context) indexedLocate(rabAddr uint32, h *FileHandle, find bool) (uint32, error) {
	s := &h.idx

	fail := func(sts uint32) (uint32, error) {
		s.lost()

		if !find {
			if err := ctx.storeWord(rabAddr+rabRSZ, 0); err != nil {
				return 0, err
			}
		}

		if err := ctx.storeRFA(rabAddr, rfa{}); err != nil {
			return 0, err
		}

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	if h.stream.fac&facReads == 0 {
		return fail(rmsFACNotAllowed)
	}

	rac, err := ctx.loadByte(rabAddr + rabRAC)
	if err != nil {
		return 0, err
	}

	rop, err := ctx.loadLongword(rabAddr + rabROP)
	if err != nil {
		return 0, err
	}

	f, err := h.indexedFile()
	if err != nil {
		return fail(prologStatus(err))
	}

	var (
		e      isam.Entry
		record []byte
	)

	switch rac {
	case racSeq:
		switch {
		case s.afterFind && s.hasCurrent && !find:
			e = s.current
			record, err = f.Record(e.RFA)
		case s.queuedEnd, s.atEnd:
			err = isam.ErrEnd
		case s.queued != nil:
			// The record that followed a deleted one, or the one after it if
			// it's gone too.
			e = *s.queued
			if record, err = f.Record(e.RFA); err != nil {
				e, record, err = nextLive(f, e, false)
			}
		default:
			e, record, err = nextLive(f, isam.Entry{Ref: s.krf, Key: s.place.Key, RFA: s.place.RFA}, !s.hasPlace)
		}

		switch {
		case errors.Is(err, isam.ErrEnd), errors.Is(err, isam.ErrDeleted):
			// At the end of the file a stream that unlocks automatically gives
			// up its record lock, as on the other organizations (Phase 47).
			if ctx.locksRecords(h) && rop&ropULK == 0 {
				ctx.unlockRecords(h, nil)
			}

			return fail(rmsEOF)
		case err != nil:
			return fail(rmsDeviceError)
		}
	case racKey:
		krf, err := ctx.loadByte(rabAddr + rabKRF)
		if err != nil {
			return 0, err
		}

		if int(krf) >= len(f.Keys) {
			return fail(rmsInvalidKeyRef)
		}

		match, ok := keyMatch(rop)
		if !ok {
			return fail(rmsInvalidROP)
		}

		value, sts, err := ctx.searchValue(rabAddr, f.Keys[krf])
		if err != nil {
			return 0, err
		} else if sts != 0 {
			return fail(sts)
		}

		e, record, err = findLive(f, int(krf), value, match)

		switch {
		case errors.Is(err, isam.ErrNotFound), errors.Is(err, isam.ErrDeleted):
			return fail(rmsRecordNotFound)
		case errors.Is(err, isam.ErrKeySize):
			return fail(rmsInvalidKeySize)
		case err != nil:
			return fail(rmsDeviceError)
		}
	case racRFA:
		r, err := ctx.loadRFA(rabAddr)
		if err != nil {
			return 0, err
		}

		if r.vbn == 0 {
			return fail(rmsInvalidRFA)
		}

		record, err = f.Record(isamRFA(r))

		var corrupt *isam.CorruptError

		switch {
		case errors.Is(err, isam.ErrDeleted):
			return fail(rmsRecordDeleted)
		case errors.As(err, &corrupt):
			// An RFA naming no bucket of the primary key's (unconfirmed).
			return fail(rmsInvalidRFA)
		case err != nil:
			return fail(rmsInvalidRFA)
		}

		// An RFA access makes the primary key the stream's (the RMS manual).
		value, _ := f.Keys[0].Value(record)
		e = isam.Entry{Ref: 0, Key: value, RFA: isamRFA(r)}
	default:
		return fail(rmsInvalidRAC)
	}

	result := rmsNormal

	if ctx.locksRecords(h) {
		lockSts, err := ctx.lockFound(rabAddr, h, rmsRFA(e.RFA), rop)
		if err != nil || lockSts == 0 {
			return 0, err
		}

		if lockSts&1 == 0 {
			return fail(lockSts)
		}

		result = lockSts

		// The record as it is now that it's locked.
		if result == rmsOKWaited {
			if record, err = f.Record(e.RFA); err != nil {
				return fail(rmsRecordNotFound)
			}
		}
	}

	if result == rmsNormal {
		sts, err := ctx.keyStatus(rabAddr, f, e, rac, rop)
		if err != nil {
			return 0, err
		}

		if sts != 0 {
			result = sts
		}
	}

	h.readBuffer.touch(ctx, int64(e.RFA.VBN-1)*isam.BlockSize)

	// The context moves on.
	s.moveTo(e)
	s.current, s.hasCurrent, s.afterFind = e, true, find

	if err := ctx.storeRFA(rabAddr, rmsRFA(e.RFA)); err != nil {
		return 0, err
	}

	if find {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, result)
	}

	return storeRecordStatus(ctx, rabAddr, record, result)
}

// keyStatus is the success status ROP LIM and CDK ask for, or 0 for neither:
// RMS$_OK_LIM for a record reached in sequence whose key, by its first
// RAB$B_KSZ bytes, isn't RAB$L_KBF's (the record is still returned, and the
// stream goes on past it: case 130's two records after the duplicate set), and
// RMS$_OK_DUP for a record whose next record by the same key has the same
// value (case 130: of a set of three, the first two).
func (ctx *Context) keyStatus(rabAddr uint32, f *isam.File, e isam.Entry, rac byte, rop uint32) (uint32, error) {
	if rac == racSeq && rop&ropLIM != 0 {
		ksz, err := ctx.loadByte(rabAddr + rabKSZ)
		if err != nil {
			return 0, err
		}

		size := min(int(ksz), len(e.Key))
		if size == 0 {
			size = len(e.Key)
		}

		kbf, err := ctx.loadLongword(rabAddr + rabKBF)
		if err != nil {
			return 0, err
		}

		limit, err := ctx.loadBytes(kbf, size)
		if err != nil {
			return rmsInvalidKeyBuffer, nil
		}

		if !bytes.Equal(limit, e.Key[:size]) {
			return rmsOKLimit, nil
		}
	}

	if rop&ropCDK != 0 {
		if next, _, err := nextLive(f, e, false); err == nil && bytes.Equal(next.Key, e.Key) {
			return rmsOKDuplicate, nil
		}
	}

	return 0, nil
}

// putStatus is the status for an isam error from Put or Update.
func putStatus(err error) uint32 {
	switch {
	case errors.Is(err, isam.ErrDuplicate):
		return rmsDuplicateKey
	case errors.Is(err, isam.ErrRecordSize):
		return rmsRecordTooBig
	case errors.Is(err, isam.ErrKeyChanged):
		return rmsKeyChanged
	case errors.Is(err, isam.ErrDeleted):
		return rmsNoCurrent
	}

	return rmsDeviceError
}

// recordFits reports whether a record of n bytes fits the file's records: no
// longer than MRS (if the file has one), and exactly MRS for fixed-length
// records.
func recordFits(a isam.Attributes, n int) bool {
	switch {
	case a.Format == isam.Fixed:
		return n == a.Size
	case a.Size != 0:
		return n <= a.Size
	}

	return true
}

// indexedPut is $PUT on an indexed file: by its keys, wherever they go, with
// RAB$B_RAC SEQ only in primary key order (RMS$_SEQ for a lower key than the
// stream's last; an equal one is RMS$_DUP if the key has no duplicates: case
// 133). RAB$V_UIF makes a record whose primary key is there already an update
// of it (case 142), and RAB$V_LOA fills buckets only to their fill sizes. The
// result is RMS$_OK_DUP when a key's duplicate was taken. A failure leaves
// RAB$W_RFA as it was.
func (ctx *Context) indexedPut(rabAddr uint32, h *FileHandle, record []byte) (uint32, error) {
	s := &h.idx

	fail := func(sts uint32) (uint32, error) {
		s.lost()

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	if h.stream.fac&facPut == 0 {
		return fail(rmsFACNotAllowed)
	}

	rac, err := ctx.loadByte(rabAddr + rabRAC)
	if err != nil {
		return 0, err
	}

	rop, err := ctx.loadLongword(rabAddr + rabROP)
	if err != nil {
		return 0, err
	}

	f, err := h.indexedFile()
	if err != nil {
		return fail(prologStatus(err))
	}

	k := f.Keys[0]

	value, ok := k.Value(record)
	if !ok || !recordFits(indexedAttributes(h.File), len(record)) {
		return fail(rmsRecordTooBig)
	}

	if rac == racSeq && s.lastPut != nil && k.Compare(value, s.lastPut) < 0 {
		return fail(rmsOutOfSequence)
	}

	if rop&ropUIF != 0 {
		if e, err := f.Find(0, value, isam.Equal); err == nil {
			return ctx.putOver(rabAddr, h, f, e.RFA, record)
		}
	}

	h.wrote(ctx, 0)

	var result isam.PutResult

	if rop&ropLOA != 0 {
		result, err = f.PutLoad(record)
	} else {
		result, err = f.Put(record)
	}

	if err != nil {
		return fail(putStatus(err))
	}

	s.lost()

	if rac == racSeq {
		s.lastPut = value
	}

	if err := ctx.storeRFA(rabAddr, rmsRFA(result.RFA)); err != nil {
		return 0, err
	}

	if result.Duplicate {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsOKDuplicate)
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// putOver is a $PUT with RAB$V_UIF of a record whose primary key is there: an
// update of that record, which must not be locked by another stream (RMS$_RLK,
// case 153) and needs update access (RMS$_FAC, the RMS manual). Its RFA goes
// in the RAB, and the status is the update's.
func (ctx *Context) putOver(rabAddr uint32, h *FileHandle, f *isam.File, at isam.RFA, record []byte) (uint32, error) {
	s := &h.idx

	fail := func(sts uint32) (uint32, error) {
		s.lost()

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	if h.stream.fac&facUpd == 0 {
		return fail(rmsFACNotAllowed)
	}

	r := rmsRFA(at)

	if ctx.locksRecords(h) {
		result, _, err := ctx.lockRecord(h, r, 0)
		if err != nil {
			return 0, err
		}

		switch result {
		case lockRefused:
			return fail(rmsRecordLocked)
		case lockQuota:
			return fail(rmsExEnqLm)
		case lockHeld:
			// Held only for the update.
			defer ctx.releaseRecord(h, r)
		}
	}

	h.wrote(ctx, 0)

	result, err := f.Update(at, record)
	if err != nil {
		return fail(putStatus(err))
	}

	s.lost()

	if err := ctx.storeRFA(rabAddr, r); err != nil {
		return 0, err
	}

	if result.Duplicate {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsOKDuplicate)
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// releaseRecord gives up the stream's lock on r.
func (ctx *Context) releaseRecord(h *FileHandle, r rfa) {
	if l := h.locks.records[r]; l != nil {
		events, _ := ctx.Locks.Dequeue(l.Owner, l.ID, lck.DequeueOptions{})
		lck.Deliver(events)
		delete(h.locks.records, r)
	}
}

// indexedUpdate is $UPDATE on an indexed file: the current record replaced by
// the one at RAB$L_RBF, its length free to change but not its primary key, an
// alternate key only with CHG (RMS$_CHG; a record too short for an alternate
// without CHG changes it too), and a new alternate value one its key has
// already RMS$_DUP if the key has no duplicates, RMS$_OK_DUP if it has (cases
// 138 to 141). An update that makes its bucket split leaves an STV of 1, as
// VMS's did (case 147; why isn't known). Afterwards there's no current record;
// a failure leaves RAB$W_RFA as it was.
func (ctx *Context) indexedUpdate(rabAddr uint32, h *FileHandle) (uint32, error) {
	s := &h.idx

	fail := func(sts uint32) (uint32, error) {
		s.lost()

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	at := rmsRFA(s.current.RFA)

	switch {
	case h.stream.fac&facUpd == 0:
		return fail(rmsFACNotAllowed)
	case !s.hasCurrent:
		return fail(rmsNoCurrent)
	case ctx.locksRecords(h) && h.locks.records[at] == nil:
		return fail(rmsRecordNotLocked)
	}

	rsz, err := ctx.loadWord(rabAddr + rabRSZ)
	if err != nil {
		return 0, err
	}

	rbf, err := ctx.loadLongword(rabAddr + rabRBF)
	if err != nil {
		return 0, err
	}

	record, err := ctx.loadBytes(rbf, int(rsz))
	if err != nil {
		return 0, err
	}

	f, err := h.indexedFile()
	if err != nil {
		return fail(prologStatus(err))
	}

	if !recordFits(indexedAttributes(h.File), len(record)) {
		return fail(rmsRecordTooBig)
	}

	h.wrote(ctx, 0)

	result, err := f.Update(s.current.RFA, record)
	if err != nil {
		return fail(putStatus(err))
	}

	s.lost()

	if err := ctx.storeRFA(rabAddr, at); err != nil {
		return 0, err
	}

	sts := rmsNormal
	if result.Duplicate {
		sts = rmsOKDuplicate
	}

	if _, err := storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts); err != nil {
		return 0, err
	}

	if result.Split {
		if err := ctx.storeLongword(rabAddr+rabSTV, 1); err != nil {
			return 0, err
		}
	}

	return sts, nil
}

// indexedDelete is $DELETE on an indexed file: the current record goes, with
// its pointers in the alternate indexes unless ROP FDL leaves them for a later
// search to take out (case 144). It needs FAB$V_DEL access and a current
// record (RMS$_CUR, case 143's second $DELETE). A success leaves RAB$W_RFA 0
// (cases 143 to 149), and a sequential $GET then reads the record after it by
// the stream's key (case 146).
func (ctx *Context) indexedDelete(rabAddr uint32, h *FileHandle) (uint32, error) {
	s := &h.idx

	fail := func(sts uint32) (uint32, error) {
		s.lost()

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	at := rmsRFA(s.current.RFA)

	switch {
	case h.stream.fac&facDel == 0:
		return fail(rmsFACNotAllowed)
	case !s.hasCurrent:
		return fail(rmsNoCurrent)
	case ctx.locksRecords(h) && h.locks.records[at] == nil:
		return fail(rmsRecordNotLocked)
	}

	rop, err := ctx.loadLongword(rabAddr + rabROP)
	if err != nil {
		return 0, err
	}

	f, err := h.indexedFile()
	if err != nil {
		return fail(prologStatus(err))
	}

	// The record after it, while it's there to go on from.
	var (
		next    *isam.Entry
		nextEnd bool
	)

	if s.hasPlace && s.place.RFA == s.current.RFA {
		switch e, _, err := nextLive(f, s.place, false); {
		case err == nil:
			next = &e
		case errors.Is(err, isam.ErrEnd):
			nextEnd = true
		}
	}

	h.wrote(ctx, 0)

	if err := f.Delete(s.current.RFA, rop&ropFDL != 0); err != nil {
		return fail(putStatus(err))
	}

	ctx.releaseRecord(h, at)
	s.lost()

	if next != nil || nextEnd {
		s.queued, s.queuedEnd = next, nextEnd
	}

	if err := ctx.storeRFA(rabAddr, rfa{}); err != nil {
		return 0, err
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// indexedRewind is $REWIND on an indexed file: the stream's next record is
// RAB$B_KRF's first, and that key the stream's (case 131); the current record
// stays. A key past the file's is RMS$_KRF (unconfirmed).
func (ctx *Context) indexedRewind(rabAddr uint32, h *FileHandle) (uint32, error) {
	krf, err := ctx.loadByte(rabAddr + rabKRF)
	if err != nil {
		return 0, err
	}

	f, err := h.indexedFile()
	if err != nil {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, prologStatus(err))
	}

	if int(krf) >= len(f.Keys) {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsInvalidKeyRef)
	}

	h.idx.rewind(int(krf))
	h.idx.lastPut = nil

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}
