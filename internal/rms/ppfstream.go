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

// RecordStream is a RecordDevice made of Go functions: a stream DCL keeps
// as a process-permanent file that is neither a volume file nor a device
// of the system's (docs/PHASE-71 - process-permanent files.md, subtask 6):
// a command procedure's data records, which DCL's own reader gives out,
// or an /OUTPUT file on the host.
type RecordStream struct {
	// Next returns the next record, and false at the end of the data;
	// nil for a stream that can't be read.
	Next func() (record []byte, ok bool)

	// Write writes one record; nil for a stream that throws its records
	// away.
	Write func(record []byte) error

	// Done ends the stream when its entry is closed; nil for nothing to
	// do.
	Done func()

	// Taken is told of each record Get gives a reader; nil for nothing
	// to do. DCL's verification shows a procedure's data record then, not
	// when a $FIND locates it (VMS 7.1, testdata/ppf71's I5).
	Taken func(record []byte)

	// held is the record a $FIND located (Find), which the next Get
	// returns, as a sequential $GET after a $FIND gets the record found
	// on a file.
	held    []byte
	holding bool
}

// ssAbort is RecordStream.Put's status for a record Write couldn't write,
// which PutPermanent reports as RMS$_WER.
var ssAbort = vmsConst("SS$_ABORT")

// Characteristics is 0: AddPermanent gives the entry its own
// (PermanentOptions.Characteristics, or a record device's).
func (s *RecordStream) Characteristics() uint32 { return 0 }

// Put writes record through Write, and never waits.
func (s *RecordStream) Put(record []byte, _ bool) (uint32, bool, error) {
	if s.Write == nil {
		return ssNormal, false, nil
	}

	if err := s.Write(record); err != nil {
		return ssAbort, false, nil
	}

	return ssNormal, false, nil
}

// WaitPut has nothing to wait for.
func (s *RecordStream) WaitPut() (uint32, error) { return ssNormal, nil }

// Get reads the next record, the one Find located if it located one:
// SS$_ENDOFFILE at the end.
func (s *RecordStream) Get() ([]byte, uint32, error) {
	record, ok := s.held, s.holding
	s.held, s.holding = nil, false

	if !ok {
		if record, ok = s.next(); !ok {
			return nil, ssEndOfFile, nil
		}
	}

	if s.Taken != nil {
		s.Taken(record)
	}

	return record, ssNormal, nil
}

// Find locates the next record, which the next Get returns: a sequential
// $FIND. SS$_ENDOFFILE at the end.
func (s *RecordStream) Find() uint32 {
	record, ok := s.next()
	s.held, s.holding = record, ok

	if !ok {
		return ssEndOfFile
	}

	return ssNormal
}

// next is the stream's next record from Next, false at the end or for a
// stream that can't be read.
func (s *RecordStream) next() ([]byte, bool) {
	if s.Next == nil {
		return nil, false
	}

	return s.Next()
}

// Close ends the stream through Done.
func (s *RecordStream) Close() {
	if s.Done != nil {
		s.Done()
	}
}

// AddDCLFile makes f, a file DCL opened (OpenDCLFile), a process-permanent
// file of t's, with access fac (FAB$B_FAC: an /OUTPUT file's is PUT
// alone, so the device's characteristics have no input, as VMS 7.1's
// log's had none). A volume file's entry is its own stream, so an
// indirect accessor gets its attributes; a host file's is a RecordStream
// on it. Closing the entry closes f.
func (t *FileTable) AddDCLFile(f *DCLFile, fac byte, opts PermanentOptions) (*PermanentEntry, error) {
	if f.host != nil {
		stream := &RecordStream{
			Next: func() ([]byte, bool) {
				record, sts := f.host.get()

				return record, sts&1 != 0
			},
			Write: func(record []byte) error {
				if sts := f.host.put(record); sts&1 == 0 {
					return errors.New("rms: host file record not written")
				}

				return nil
			},
			Done: func() { _ = f.host.file.Close() },
		}

		if opts.Characteristics == 0 {
			opts.Characteristics = fileDevChar(fac)
		}

		return t.AddPermanent(&FileHandle{Device: stream, Access: fac}, opts)
	}

	// The file leaves the table OpenDCLFile kept it in: its entry closes
	// it now.
	f.ctx.Files.Release(f.ifi)
	f.handle.Access = fac

	return t.AddPermanent(f.handle, opts)
}

// Describe makes f, a file DCL opened, the file e's RecordStream stands
// for: an indirect open gets its attributes, and $DISPLAY its name and
// FID, while the records still come from the stream. A command
// procedure's entry describes the procedure running now. A host file, or
// f nil, describes nothing.
func (e *PermanentEntry) Describe(f *DCLFile) {
	if !e.handle.IsRecordDevice() {
		return
	}

	e.handle.File, e.handle.Found = nil, nil

	if f != nil && f.handle != nil {
		e.handle.File, e.handle.Found = f.handle.File, f.handle.Found
	}
}
