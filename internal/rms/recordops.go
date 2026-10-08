package rms

import (
	"fmt"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// The record operations past $GET and $PUT (docs/PHASE-49.md, subtasks
// 2 and 3): $FIND, $UPDATE, $TRUNCATE, $DELETE, and $REWIND, on the
// stream context stream.go keeps. Their rules are the RMS Reference
// Manual's and the Guide to OpenVMS File Applications' (sections 8.2
// and 8.6); the phase doc's survey lists them, and which of govax's
// choices VMS hasn't confirmed.

// rmsIOP is RMS$_IOP, an operation the file's organization or device
// doesn't allow: $DELETE on a sequential file, $UPDATE or $TRUNCATE on a
// terminal or mailbox.
var rmsIOP = rmsInvalidOperation

// SysFind implements SYS$FIND (RAB at argv[0]): it finds a record as
// $GET would, by RAB$B_RAC (sequential, or by RFA), and locks it, but
// doesn't return it: the record becomes the stream's current record, for
// a $GET, $UPDATE, or $TRUNCATE to follow, and its RFA goes in
// RAB$W_RFA. A sequential $FIND skips a record; a $FIND by RFA leaves the
// next record where it was. On a terminal or a mailbox it reads a record
// and drops it.
func SysFind(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	handle, sts, err := ctx.streamOf(rabAddr)
	if err != nil || sts != 0 {
		return sts, err
	}

	if handle.IsConsole() || handle.IsRecordDevice() {
		return findOnDevice(ctx, rabAddr, handle)
	}

	found, r0, err := ctx.locate(rabAddr, handle, true)
	if err != nil || found.sts == 0 {
		return r0, err
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, found.sts)
}

// findOnDevice is $FIND on the terminal or a record device: a record is
// read (the terminal prompting as for $GET) and thrown away.
func findOnDevice(ctx *Context, rabAddr uint32, h *FileHandle) (uint32, error) {
	if h.IsConsole() {
		_, status, err := terminalRecord(ctx, rabAddr, maxTerminalRecord)
		if err != nil {
			return 0, err
		}

		if status != 0 {
			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, status)
		}

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
	}

	if h.Access&facGet == 0 {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsFACNotAllowed)
	}

	_, status, err := h.Device.Get()
	if err != nil {
		return 0, err
	}

	switch status {
	case ssNormal:
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
	case ssEndOfFile:
		return rabStatus(ctx, rabAddr, rmsEOF, 0)
	}

	return rabStatus(ctx, rabAddr, rmsReadError, status)
}

// SysUpdate implements SYS$UPDATE (RAB at argv[0]): the stream's current
// record (found by $GET or $FIND) is replaced by the record at RAB$L_RBF,
// RAB$W_RSZ bytes long, in place. A sequential file's record can't
// change length: a record of another length is RMS$_RSZ (the RMS
// manual's rule for sequential files; govax applies it to the stream
// formats too, unconfirmed). It needs FAB$V_UPD access (RMS$_FAC) and a
// current record (RMS$_CUR); in a stream that locks records, the record
// must be locked by it (RMS$_RNL, unconfirmed). The RFA goes in
// RAB$W_RFA; afterwards there's no current record, and the next record
// is unchanged.
func SysUpdate(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	handle, sts, err := ctx.streamOf(rabAddr)
	if err != nil || sts != 0 {
		return sts, err
	}

	if handle.IsConsole() || handle.IsRecordDevice() {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsIOP)
	}

	s := &handle.stream

	fail := func(sts uint32) (uint32, error) {
		s.lost()

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	switch {
	case s.fac&facUpd == 0 || handle.Reader == nil:
		return fail(rmsFACNotAllowed)
	case !s.hasCurrent:
		return fail(rmsNoCurrent)
	case ctx.locksRecords(handle) && handle.locks.records[rfaAt(s.current)] == nil:
		return fail(rmsRecordNotLocked)
	}

	old, _, sts := handle.recordAt(s.current)
	if sts != 0 {
		// The record went (the file was truncated by another stream).
		return fail(rmsNoCurrent)
	}

	rsz, err := ctx.loadWord(rabAddr + rabRSZ)
	if err != nil {
		return 0, err
	}

	if int(rsz) != len(old) {
		return fail(rmsRecordTooBig)
	}

	rbf, err := ctx.loadLongword(rabAddr + rabRBF)
	if err != nil {
		return 0, err
	}

	record, err := ctx.loadFixedString(rbf, int(rsz))
	if err != nil {
		return 0, err
	}

	at := s.current

	if err := writeAt(handle.File, at+recordFraming(handle.File), []byte(record)); err != nil {
		return fail(rmsDeviceError)
	}

	if err := ctx.storeRFA(rabAddr, rfaAt(at)); err != nil {
		return 0, err
	}

	s.lost()

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// recordFraming is how many bytes come before a record's data in f: a
// Variable or VFC record's length word, nothing for the others (a stream
// record's delimiter follows its data).
func recordFraming(f *volume.File) int64 {
	switch f.Header.RecordAttributes.Format {
	case ondisk.RecordFormatVariable, ondisk.RecordFormatVFC:
		return 2
	}

	return 0
}

// writeAt writes data into f at byte offset off, over what is there:
// each block it touches is read, changed, and written back.
func writeAt(f *volume.File, off int64, data []byte) error {
	block := make([]byte, ondisk.BlockSize)

	for len(data) > 0 {
		vbn := uint32(off/ondisk.BlockSize) + 1

		if err := f.ReadBlock(vbn, block); err != nil {
			return fmt.Errorf("rms: reading virtual block %d: %w", vbn, err)
		}

		n := copy(block[off%ondisk.BlockSize:], data)

		if err := f.WriteBlock(vbn, block); err != nil {
			return fmt.Errorf("rms: writing virtual block %d: %w", vbn, err)
		}

		data, off = data[n:], off+int64(n)
	}

	return nil
}

// truncateAt makes h's file end at byte offset off: the records from
// there on are gone, for every stream. The blocks stay allocated, and
// their bytes aren't erased (RMS manual, $TRUNCATE). The stream's next
// record becomes the end of the file.
func (h *FileHandle) truncateAt(off int64) {
	h.File.SetEndOfFile(uint32(off/ondisk.BlockSize)+1, uint16(off%ondisk.BlockSize))
	h.stream.lost()
	h.stream.next, h.stream.atEnd = off, true
}

// SysTruncate implements SYS$TRUNCATE (RAB at argv[0]): the file is cut
// off at its current record (the one RAB$W_RFA names, with RAB$B_RAC
// RAB$C_RFA), which goes with every record after it; $PUT then appends
// there. It needs FAB$V_TRN access (RMS$_FAC) and, in sequential access,
// a current record (RMS$_CUR).
func SysTruncate(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	handle, sts, err := ctx.streamOf(rabAddr)
	if err != nil || sts != 0 {
		return sts, err
	}

	if handle.IsConsole() || handle.IsRecordDevice() {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsIOP)
	}

	s := &handle.stream

	fail := func(sts uint32) (uint32, error) {
		s.lost()

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	if s.fac&facTrn == 0 {
		return fail(rmsFACNotAllowed)
	}

	rac, err := ctx.loadByte(rabAddr + rabRAC)
	if err != nil {
		return 0, err
	}

	var at int64

	switch rac {
	case racSeq:
		if !s.hasCurrent {
			return fail(rmsNoCurrent)
		}

		at = s.current
	case racRFA:
		r, err := ctx.loadRFA(rabAddr)
		if err != nil {
			return 0, err
		}

		off, ok := handle.validRFA(r)
		if !ok {
			return fail(rmsInvalidRFA)
		}

		at = off
	default:
		return fail(rmsInvalidRAC)
	}

	handle.truncateAt(at)

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// SysDelete implements SYS$DELETE (RAB at argv[0]), the record $DELETE:
// it removes records from relative and indexed files only ("You cannot
// use this service when processing sequential files", RMS manual), and
// govax has sequential files only, so a connected stream's $DELETE is
// RMS$_IOP.
func SysDelete(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	handle, sts, err := ctx.streamOf(rabAddr)
	if err != nil || sts != 0 {
		return sts, err
	}

	handle.stream.lost()

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsIOP)
}

// SysRewind implements SYS$REWIND (RAB at argv[0]): the stream's next
// record becomes the file's first; the current record is unchanged (the
// guide, Table 8-3). On a terminal or a mailbox it does nothing
// (unconfirmed).
func SysRewind(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	handle, sts, err := ctx.streamOf(rabAddr)
	if err != nil || sts != 0 {
		return sts, err
	}

	if !handle.IsConsole() && !handle.IsRecordDevice() {
		s := &handle.stream
		s.next, s.atEnd, s.afterFind = 0, false, false
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}
