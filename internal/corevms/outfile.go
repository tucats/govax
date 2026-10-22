package corevms

import (
	"github.com/tucats/govax/internal/rms"
)

// SYS$OUTPUT and SYS$ERROR as an image's own files (docs/PHASE-70 -
// traceback.md, subtask 8; docs/PHASE-71 - process-permanent files.md,
// subtask 8).
//
// A process's SYS$OUTPUT and SYS$ERROR are normally process-permanent
// files (ppfstart.go, and DCL's), which LIB$PUT_OUTPUT and $PUTMSG reach
// through their entries. A name the image's DCL defined for it instead
// (DEFINE/USER_MODE SYS$OUTPUT file, then RUN) leads to no entry: on VMS
// LIB$PUT_OUTPUT's $CREATE of SYS$OUTPUT then makes the file, which the
// image holds open until its rundown, and a condition's message, which
// $PUTMSG writes through an open of its own, can't go there (testdata/
// tb70's part S; msgout.go). govax makes the file at the first record
// written to it (variable-length records, carriage-return carriage
// control: rms.DCLAccess.Variable), keeps it open, and
// closes it at image rundown (closeImageOutput), so the next image makes
// a new version.

// outputFile is SYS$OUTPUT or SYS$ERROR as an image's file: the name it
// was opened for, and the open file.
type outputFile struct {
	spec string
	file *rms.DCLFile

	// held is set when the image wrote to the file itself (PutOutput): on
	// VMS it then has the file open, and $PUTMSG's own open of it fails.
	held bool
}

// processOutput is where one of the process's output names, SYS$OUTPUT
// or SYS$ERROR, sends records when it leads to no process-permanent file.
type processOutput struct {
	// name is the logical name, "SYS$OUTPUT" or "SYS$ERROR".
	name string

	// stream is the stream records go through when the name translates
	// to a mailbox or NL: (recdevice.go).
	stream *recordDevice

	// file is the file the image made, kept until its rundown.
	file *outputFile
}

// putRecordFile writes record to out's file when its name translates to
// a file, reporting whether it did. A file that can't be made or written
// is no file: the line goes to the terminal, as it would with no
// SYS$OUTPUT.
func (env *Environment) putRecordFile(out *processOutput, record string) bool {
	f := env.openRecordFile(out)
	if f == nil {
		return false
	}

	return f.file.Put([]byte(record))&1 != 0
}

// openRecordFile returns out's file, creating it (a new version) the
// first time, or when out's name has come to name another (the one before
// is closed); nil when the name isn't defined in the process table, names
// a device that holds no files, or the file can't be made; nil too when
// the name leads to a process-permanent file, open or closed, which is
// never the image's.
func (env *Environment) openRecordFile(out *processOutput) *outputFile {
	spec := env.processName(out.name)
	if spec == "" || env.Session == nil || isPermanentString(env.processValue(out.name)) {
		return nil
	}

	if _, ok := env.permanentEntry(out.name); ok {
		return nil
	}

	if f := out.file; f != nil && f.spec == spec {
		return f
	}

	if device, st := env.deviceName(out.name); st == 0 && !env.isFileDevice(device) {
		return nil // a terminal, mailbox, or NL:
	}

	loc, err := env.Session.Locate(spec, false)
	if err != nil {
		return nil
	}

	file, err := env.Session.OpenDCLFile(loc, rms.DCLAccess{Write: true, ShareRead: true, Variable: true})
	if err != nil {
		return nil
	}

	closeOutputFile(out)
	out.file = &outputFile{spec: spec, file: file}

	return out.file
}

// closeOutputFile closes out's file, if it has one.
func closeOutputFile(out *processOutput) {
	if out.file != nil {
		_ = out.file.file.Close()
		out.file = nil
	}
}

// closeImageOutput is image rundown's step for the files the image made
// (this file's opening comment).
func (env *Environment) closeImageOutput() {
	closeOutputFile(&env.output)
	closeOutputFile(&env.errorOutput)
}
