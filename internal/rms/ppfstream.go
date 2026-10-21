package rms

import "errors"

// Record I/O on a process-permanent file from Go (docs/PHASE-71 -
// process-permanent files.md, subtask 5): what LIB$PUT_OUTPUT,
// LIB$GET_INPUT, and $PUTMSG do on VMS by an indirect open of SYS$OUTPUT,
// SYS$INPUT, or SYS$ERROR, a $CONNECT, and a $PUT or $GET (ppfaccess.go).
// A record goes through the entry's one stream, as an indirect accessor's
// does, checked against the access of the open that made the entry; to a
// VFC print file it takes DCL's carriage control, as an accessor whose FAB
// asks for CR does.

// ErrTerminalEntry is GetPermanent's error for the terminal, whose lines
// the caller reads itself, with the terminal's rules.
var ErrTerminalEntry = errors.New("rms: the terminal's lines are read by the terminal's rules")

// IsTerminal reports whether the entry's stream is the terminal.
func (e *PermanentEntry) IsTerminal() bool { return e.handle.IsConsole() }

// PutPermanent writes record to e's stream as one record and returns
// RMS$_NORMAL, or the status for why not: RMS$_FAC when the open that made
// the entry can't write; for a file, putSequential's; for a mailbox,
// RMS$_RSZ for a record too big, RMS$_WER for another failure. On the
// terminal a record is a line. A non-nil error is a mailbox's wait for
// room (the caller is called again) or a failure of the machine.
func (ctx *Context) PutPermanent(e *PermanentEntry, record []byte) (uint32, error) {
	h := e.handle

	if h.Access&facPut == 0 {
		return rmsFACNotAllowed, nil
	}

	switch {
	case h.IsConsole():
		if _, err := h.Console.Write(append(record, '\n')); err != nil {
			return rmsDeviceError, nil
		}

		ctx.countIO(1, 0)

		return rmsNormal, nil
	case h.IsRecordDevice():
		status, _, err := h.Device.Put(record, false)
		if err != nil {
			return 0, err
		}

		switch status {
		case ssNormal:
			return rmsNormal, nil
		case ssMbTooSml:
			return rmsRecordTooBig, nil
		}

		return rmsWriteError, nil
	}

	return h.putSequential(ctx, record), nil
}

// GetPermanent reads the next record from e's stream and returns it with
// RMS$_NORMAL, or nil and the status for why not: RMS$_FAC when the open
// that made the entry can't read, RMS$_EOF at the end of the data, for a
// mailbox RMS$_RER for another failure. The terminal is
// ErrTerminalEntry. A non-nil error is otherwise a mailbox's wait for a
// message (the caller is called again) or a failure of the machine.
func (ctx *Context) GetPermanent(e *PermanentEntry) ([]byte, uint32, error) {
	h := e.handle

	switch {
	case h.IsConsole():
		return nil, 0, ErrTerminalEntry
	case h.Access&facGet == 0:
		return nil, rmsFACNotAllowed, nil
	case h.IsRecordDevice():
		record, status, err := h.Device.Get()
		if err != nil {
			return nil, 0, err
		}

		switch status {
		case ssNormal:
			return record, rmsNormal, nil
		case ssEndOfFile:
			return nil, rmsEOF, nil
		}

		return nil, rmsReadError, nil
	}

	record, sts := h.getSequential(ctx)

	return record, sts, nil
}
