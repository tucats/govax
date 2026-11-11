package rms

import (
	"errors"
	"io"
)

// SysGet implements SYS$GET: given a RAB's VAX address (argv[0]), reads
// the record the stream's context and RAB$B_RAC choose (stream.go; the
// next one, in sequential access) from the file, terminal, or mailbox the
// RAB is connected to, and copies it into the caller's user buffer.
//
// Like every handler in this package it returns (uint32, error): the
// uint32 is R0 for the calling program, and a non-nil error is a failure
// at the emulator's level (an address the program can't reach) rather
// than an RMS status (status.go's storeStatus explains the split).
//
// # The buffers: RAB$L_UBF in, RAB$L_RBF out
//
// The record goes to the user buffer, RAB$L_UBF, of RAB$W_USZ bytes:
// "The Get service always requires the presence of a user record area"
// (the OpenVMS RMS Reference Manual's $GET). Without one, $GET is RMS$_UBF
// (no address) or RMS$_USZ (a size of 0) before any record is read, so
// no terminal line is consumed (missingBuffer). A record longer than
// the buffer is cut to it with RMS$_RTB, a warning whose RAB$L_STV is the
// whole record's size. RAB$W_RSZ is set to the length stored, and
// RAB$L_RBF to where the record is: the user buffer, in move mode (the
// manual's 7.16). RAB$L_RBF is an output here, as RAB$L_UBF is an input:
// a program that sets RBF and not UBF has a record buffer for $PUT, not
// one for $GET. Which of RMS$_UBF and RMS$_USZ VMS gives when both are
// missing is unconfirmed; govax checks the address first.
func SysGet(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	// A RAB only ever refers to a file through the IFI SYS$CONNECT already
	// copied into its RAB$W_ISI field (rab.go's rabISI) — SYS$GET, like
	// SYS$PUT, never touches the RAB's related FAB directly.
	ifi, err := ctx.loadWord(rabAddr + rabISI)
	if err != nil {
		return 0, err
	}

	handle, ok := ctx.Files.LookupStream(ifi)
	if !ok {
		// This RAB was never SYS$CONNECTed (or its $CONNECT failed): RMS$_ISI,
		// as VMS 7.1 gave a $PUT after a failed $CREATE (testdata/for81,
		// FOR81R's #15).
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsInvalidISI)
	}

	if sts, err := ctx.indirectAccess(rabAddr, facGet); err != nil || sts != 0 {
		return sts, err
	}

	if handle.IsConsole() || handle.IsRecordDevice() {
		rac, err := ctx.loadByte(rabAddr + rabRAC)
		if err != nil {
			return 0, err
		}

		if rac != racSeq {
			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsInvalidRAC)
		}

		if sts, err := ctx.missingBuffer(rabAddr, rac); err != nil || sts != 0 {
			if err != nil {
				return 0, err
			}

			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
		}

		if handle.IsConsole() {
			return getTerminal(ctx, rabAddr)
		}

		return getRecordDevice(ctx, rabAddr, handle)
	}

	if handle.indexedRecords() {
		return ctx.indexedLocate(rabAddr, handle, false)
	}

	if handle.relativeRecords() {
		return ctx.relativeLocate(rabAddr, handle, false)
	}

	// A file on a volume: the record the stream's context and RAB$B_RAC
	// choose (stream.go), locked if the stream locks records.
	found, r0, err := ctx.locate(rabAddr, handle, false)
	if err != nil || found.sts == 0 {
		return r0, err
	}

	return storeRecordStatus(ctx, rabAddr, found.record, found.sts)
}

// nextRecord reads handle's next record, or returns the status for why
// not: RMS$_EOF at the end of the file, RMS$_DEV for a corrupt record.
func nextRecord(handle *FileHandle) ([]byte, uint32) {
	record, err := handle.Reader.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			// Not a failure at all from the calling VAX program's point
			// of view — RMS$_EOF is how a real sequential-file reader
			// finds out it has read every record the file has, the same
			// role Go's own io.EOF plays for this package's underlying
			// odsrms.Reader.
			return nil, rmsEOF
		}

		// Anything else (odsrms.ErrCorruptRecord: a truncated or
		// self-inconsistent record's on-disk framing) is a genuine
		// problem with the file's own content, not a Go-level bug in
		// this package, so it's reported as a generic RMS device error
		// rather than propagated as a Go error — the same convention
		// this package already uses for every other ods2-layer failure
		// (for instance, create.go's createOnVolume on a failing
		// vol.CreateFile).
		return nil, rmsDeviceError
	}

	return record, 0
}

// getTerminal is SYS$GET for a RAB connected to the terminal: it reads a
// record as terminalRecord describes, of at most RAB$W_USZ characters,
// into the user buffer, as storeRecord does for a file's record.
func getTerminal(ctx *Context, rabAddr uint32) (uint32, error) {
	usz, err := ctx.loadWord(rabAddr + rabUSZ)
	if err != nil {
		return 0, err
	}

	record, status, err := terminalRecord(ctx, rabAddr, int(usz))
	if err != nil {
		return 0, err
	}

	ctx.terminalRead()

	ctx.countIO(1, 0)

	if status != 0 {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, status)
	}

	return storeRecord(ctx, rabAddr, record)
}

// missingBuffer returns the status for a $GET's RAB with no user buffer
// (SysGet's doc comment), RMS$_UBF or RMS$_USZ, or 0 if it has one. Each
// kind of stream asks once its own refusals (FAC, block I/O) are past and
// before it reads a record, and only for a RAC that names a mode: a RAC
// that names none is RMS$_RAC, whatever the buffer.
func (ctx *Context) missingBuffer(rabAddr uint32, rac byte) (uint32, error) {
	if rac != racSeq && rac != racKey && rac != racRFA {
		return 0, nil
	}

	ubf, err := ctx.loadLongword(rabAddr + rabUBF)
	if err != nil {
		return 0, err
	}

	usz, err := ctx.loadWord(rabAddr + rabUSZ)
	if err != nil {
		return 0, err
	}

	switch {
	case ubf == 0:
		return rmsInvalidUBF, nil
	case usz == 0:
		return rmsInvalidUSZ, nil
	}

	return 0, nil
}

// storeRecord copies a record SYS$GET has read into the caller's user
// buffer, sets RAB$W_RSZ to its length, and points RAB$L_RBF at it (see
// SysGet's doc comment).
func storeRecord(ctx *Context, rabAddr uint32, record []byte) (uint32, error) {
	return storeRecordStatus(ctx, rabAddr, record, rmsNormal)
}

// storeRecordStatus is storeRecord returning sts, a success status, if
// the record is stored. A record longer than the buffer (RAB$W_USZ) is
// RMS$_RTB: the part that fits is stored, RAB$W_RSZ is that part's
// length, and RAB$L_STV the whole record's size (what VMS 7.1's FORTRAN
// run-time library reported reading SYS$INPUT, testdata/for80's round 1).
func storeRecordStatus(ctx *Context, rabAddr uint32, record []byte, sts uint32) (uint32, error) {
	destAddr, err := ctx.loadLongword(rabAddr + rabUBF)
	if err != nil {
		return 0, err
	}

	usz, err := ctx.loadWord(rabAddr + rabUSZ)
	if err != nil {
		return 0, err
	}

	if len(record) > int(usz) {
		whole := len(record)
		record = record[:usz]

		if _, err := storeRecordStatus(ctx, rabAddr, record, rmsRecordTruncated); err != nil {
			return 0, err
		}

		if err := ctx.storeLongword(rabAddr+rabSTV, uint32(whole)); err != nil {
			return 0, err
		}

		return rmsRecordTruncated, nil
	}

	for i, b := range record {
		if err := ctx.storeByte(destAddr+uint32(i), b); err != nil {
			return 0, err
		}
	}

	if err := ctx.storeWord(rabAddr+rabRSZ, uint16(len(record))); err != nil {
		return 0, err
	}

	// RAB$L_RBF is the address of the record just read: in move mode,
	// the user buffer (RMS manual, section 7.16).
	if err := ctx.storeLongword(rabAddr+rabRBF, destAddr); err != nil {
		return 0, err
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
}
