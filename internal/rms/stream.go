package rms

import (
	"github.com/tucats/govax/internal/lck"
	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
)

// A record stream's context (docs/PHASE-49.md, subtasks 2 to 4): what a
// RAB connected to a sequential file on a volume remembers between
// record operations, as the Guide to OpenVMS File Applications (section
// 8.6, Table 8-3) describes it.
//
//   - The *current record* is the record the last successful $GET or
//     $FIND reached. $UPDATE and $TRUNCATE act on it; any other
//     successful operation, and any failed one, leaves the stream with
//     none (RMS$_CUR for $UPDATE or $TRUNCATE then).
//   - The *next record* is where a sequential $GET or $FIND reads: the
//     first record after $CONNECT (the end of the file with RAB$V_EOF),
//     the record after the current one after a $GET or a sequential
//     $FIND, and the end of the file after a $PUT or $TRUNCATE. A
//     failed operation leaves it alone, so a $GET refused a record lock
//     reads the same record again next time.
//   - A sequential $GET right after a $FIND reads the record the $FIND
//     found (the current one), not the next.
//   - A $FIND or $GET by RFA (RAB$B_RAC RAB$C_RFA) reads the record at
//     RAB$W_RFA; a $GET by RFA moves the next record to the one after
//     it, a $FIND by RFA leaves the next record alone.
//
// Positions are byte offsets in the file: a record's offset is where its
// framing starts (a Variable record's length word), the same offset its
// RFA names (recordlock.go's rfaAt).
//
// A $PUT on a sequential file goes at the end of the file only: when the
// next record is the end of the file, or the file ends at the next
// record (a $GET reached RMS$_EOF). Anywhere else it is RMS$_NEF, unless
// RAB$V_TPT (truncate on put) asks for the file to be cut off there.
//
// The context belongs to the FileHandle, so every RAB connected to one
// FAB shares it (govax keeps one stream per open file).

// streamContext is a stream's record context: see above.
type streamContext struct {
	// fac is the FAB$B_FAC access $CONNECT armed the stream with (the
	// file's own, if the FAB at $CONNECT asked for none).
	fac byte

	// current is the current record's offset, if hasCurrent.
	current    int64
	hasCurrent bool

	// afterFind is set when the last successful operation was a $FIND:
	// a sequential $GET then reads the current record.
	afterFind bool

	// next is the next record's offset. atEnd is set when the next
	// record is the end of the file, wherever that is now (after
	// $CONNECT with RAB$V_EOF, $PUT, or $TRUNCATE), so a $PUT appends
	// after records other streams have added since.
	next  int64
	atEnd bool

	// fresh is set when every read must come from the disk, not from the
	// Reader's read-ahead: the file has other writers, or this stream
	// changes records in place.
	fresh bool
}

// The FAB$B_FAC access bits the record operations need, beside facGet
// and facPut (fab.go): FAB$V_TRN for $TRUNCATE and RAB$V_TPT.
var facTrn = byte(vmsConst("FAB$M_TRN"))

// facReads is the access that lets a stream read records: GET, and
// UPD, DEL, and TRN, whose operations act on a record a $FIND or $GET
// has found. That UPD, DEL, and TRN imply reading is unconfirmed (the
// RMS manual says a service the open didn't ask for is rejected).
var facReads = facGet | facUpd | facDel | facTrn

// ropTPT is RAB$M_TPT, truncate on put.
var ropTPT = vmsConst("RAB$M_TPT")

// racRFA is RAB$C_RFA, record access by record file address.
var racRFA = byte(vmsConst("RAB$C_RFA"))

// endOfFile is the byte offset at which h's file ends now.
func (h *FileHandle) endOfFile() int64 {
	return odsrms.FileByteLength(h.File.Header.RecordAttributes)
}

// startStream sets h's context as $CONNECT leaves it, for access fac and
// the RAB's options rop.
func (h *FileHandle) startStream(fac byte, rop uint32) {
	h.stream = streamContext{fac: fac}

	if rop&ropEOF != 0 {
		h.stream.next, h.stream.atEnd = h.endOfFile(), true
	}

	mixed := fac&facPut != 0 && fac&facReads != 0 || fac&(facUpd|facTrn) != 0
	h.stream.fresh = mixed || h.Accessor != nil && sharedStream(h.Mode)
}

// atEndOfFile reports whether a sequential $PUT on h goes at the end of
// the file: see the comment at the top of this file.
func (h *FileHandle) atEndOfFile() bool {
	return h.stream.atEnd || h.stream.next >= h.endOfFile()
}

// lost is the context after a failed operation: no current record.
func (s *streamContext) lost() {
	s.hasCurrent, s.afterFind = false, false
}

// recordAt reads h's record at byte offset off, returning it and the
// offset just past it, or the status for why not: RMS$_EOF at (or past)
// the end of the file.
func (h *FileHandle) recordAt(off int64) ([]byte, int64, uint32) {
	if h.stream.fresh || h.Reader.Offset() != off {
		h.Reader.SeekTo(off)
	}

	record, sts := nextRecord(h)
	if sts != 0 {
		return nil, 0, sts
	}

	return record, h.Reader.Offset(), 0
}

// validRFA reports whether r can name a record of h's file: in the file,
// and, for formats whose records start at known boundaries, on one (a
// Fixed record's multiple of its size, a Variable record's even byte).
// It returns the record's offset.
func (h *FileHandle) validRFA(r rfa) (int64, bool) {
	if r.vbn == 0 || r.offset >= ondisk.BlockSize {
		return 0, false
	}

	off := int64(r.vbn-1)*ondisk.BlockSize + int64(r.offset)
	if off >= h.endOfFile() {
		return 0, false
	}

	attr := h.File.Header.RecordAttributes

	switch attr.Format {
	case ondisk.RecordFormatFixed, ondisk.RecordFormatUndefined:
		if attr.MaxRecordSize == 0 || off%int64(attr.MaxRecordSize) != 0 {
			return 0, false
		}
	case ondisk.RecordFormatVariable, ondisk.RecordFormatVFC:
		if off%2 != 0 {
			return 0, false
		}
	}

	return off, true
}

// located is a record $GET or $FIND reached: its bytes, offset, and the
// success status to return.
type located struct {
	record []byte
	at     int64
	sts    uint32
}

// locate is $GET's and $FIND's common work on a volume file (find says
// which): it chooses the record by RAB$B_RAC and the stream's context,
// reads it, locks it if the stream locks records, and moves the context
// on. A zero located.sts means the operation failed with the status
// returned (stored in the RAB already) or is waiting (err).
func (ctx *Context) locate(rabAddr uint32, h *FileHandle, find bool) (located, uint32, error) {
	fail := func(sts uint32) (located, uint32, error) {
		h.stream.lost()
		r0, err := storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)

		return located{}, r0, err
	}

	if h.Reader == nil || h.stream.fac&facReads == 0 {
		return fail(rmsFACNotAllowed)
	}

	rac, err := ctx.loadByte(rabAddr + rabRAC)
	if err != nil {
		return located{}, 0, err
	}

	rop, err := ctx.loadLongword(rabAddr + rabROP)
	if err != nil {
		return located{}, 0, err
	}

	s := &h.stream

	// Which record: the next one, the current one for a $GET after a
	// $FIND, or the one RAB$W_RFA names.
	var target int64

	switch rac {
	case racSeq:
		target = s.next
		if s.atEnd {
			target = h.endOfFile()
		}

		if s.afterFind && !find && s.hasCurrent {
			target = s.current
		}
	case racRFA:
		r, err := ctx.loadRFA(rabAddr)
		if err != nil {
			return located{}, 0, err
		}

		off, ok := h.validRFA(r)
		if !ok {
			return fail(rmsInvalidRFA)
		}

		target = off
	default:
		return fail(rmsInvalidRAC)
	}

	record, end, sts := h.recordAt(target)
	if sts != 0 {
		// At the end of the file, a stream that unlocks automatically
		// gives up its record lock (Phase 47).
		if sts == rmsEOF && ctx.locksRecords(h) && rop&ropULK == 0 {
			ctx.unlockRecords(h, nil)
		}

		if sts == rmsEOF {
			// The next record stays at the end of the file, so a $PUT
			// may follow.
			s.next = target
		}

		return fail(sts)
	}

	result := rmsNormal

	if ctx.locksRecords(h) {
		lockSts, err := ctx.lockFound(h, rfaAt(target), rop)
		if err != nil || lockSts == 0 {
			return located{}, 0, err
		}

		if lockSts&1 == 0 {
			return fail(lockSts)
		}

		result = lockSts

		// The record as it is now that it's locked: another stream may
		// have updated it while this one waited.
		if result == rmsOKWaited {
			if record, end, sts = h.recordAt(target); sts != 0 {
				return fail(sts)
			}
		}
	}

	// The context moves on (Table 8-3).
	s.current, s.hasCurrent, s.afterFind = target, true, find
	if rac == racSeq || !find {
		s.next, s.atEnd = end, false
	}

	if err := ctx.storeRFA(rabAddr, rfaAt(target)); err != nil {
		return located{}, 0, err
	}

	return located{record: record, at: target, sts: result}, 0, nil
}

// lockFound locks the record at r that $GET or $FIND found, as RAB$L_ROP
// says, and drops the stream's other record locks unless RAB$V_ULK keeps
// them. It returns the success status for the record (RMS$_NORMAL,
// RMS$_OK_RLK, RMS$_OK_ALK, RMS$_OK_WAT), an error status (RMS$_RLK), or
// 0 with err the process's wait for the lock.
func (ctx *Context) lockFound(h *FileHandle, r rfa, rop uint32) (uint32, error) {
	ls := &h.locks
	sts := rmsNormal

	switch {
	case ls.waiting != nil && ls.pendingAt == r:
		// A lock waited for: granted now, or still not.
		if ls.waiting.State != lck.Granted {
			return 0, ctx.awaitLock(ls.waiting)
		}

		ctx.holdRecord(h, r, ls.waiting)
		ls.waiting = nil
		sts = rmsOKWaited
	default:
		if ls.waiting != nil {
			// The RAB now names another record: the old wait is given up.
			ctx.dropWaiting(h)
		}

		result, held, err := ctx.lockRecord(h, r, rop)
		if err != nil {
			return 0, err
		}

		switch result {
		case lockRefused:
			// An error unlocks the stream's record, unless it unlocks
			// manually (the guide, 7.2.1 and 7.2.4.1).
			if rop&ropULK == 0 {
				ctx.unlockRecords(h, nil)
			}

			return rmsRecordLocked, nil
		case lockWait:
			ls.pendingAt = r

			return 0, ctx.awaitLock(ls.waiting)
		case lockAlready:
			sts = rmsOKAlreadyLocked
		}

		if held {
			sts = rmsOKRecordLocked
		}
	}

	if rop&ropULK == 0 {
		ctx.unlockRecords(h, &r)
	}

	return sts, nil
}
