package corevms

import (
	"github.com/tucats/govax/internal/rms"
)

// SYS$OUTPUT and SYS$ERROR as files (docs/PHASE-48.md, subtask 2;
// docs/PHASE-70 - traceback.md, subtask 8).
//
// A process LIB$SPAWN or $CREPRC creates with an output file (a log, say,
// LIB$SPAWN's output-file) has SYS$OUTPUT translating to that file's name
// rather than to a device. On VMS the process's CLI creates the file when
// it starts and keeps it open as a process-permanent file, and every line
// written to SYS$OUTPUT is a record in it. govax creates the file at the
// first line written and writes each line through as it comes (the file
// is written again whole, keeping its version), so that whoever reads it
// later, the creator after the process ends, sees every line.
//
// A file an image's own writes or a condition's messages (msgout.go)
// made instead, under a SYS$OUTPUT or SYS$ERROR the image's DCL defined
// (DEFINE/USER_MODE SYS$OUTPUT file, then RUN), is the image's: image
// rundown forgets it (forgetImageOutput), so the next image makes a new
// version, as each image's own open does on VMS.

// outputFile is SYS$OUTPUT or SYS$ERROR as a file: the name it was opened
// for, where the file is, and its records so far.
type outputFile struct {
	spec    string
	loc     rms.FileLocation
	records [][]byte

	// held is set when the image wrote to the file itself (PutOutput): on
	// VMS it then has the file open, and a condition's message, which
	// $PUTMSG writes through an open of its own, can't go there
	// (testdata/tb70's part S; msgout.go).
	held bool
}

// processOutput is where one of the process's output names, SYS$OUTPUT
// or SYS$ERROR, sends records when it doesn't name the terminal.
type processOutput struct {
	// name is the logical name, "SYS$OUTPUT" or "SYS$ERROR".
	name string

	// stream is the stream records go through when the name translates
	// to a mailbox or NL: (recdevice.go).
	stream *recordDevice

	// permanent is the file the CLI made when the process started (a
	// SPAWN's log), kept for the process's life; file is one an image
	// made, kept until its rundown.
	permanent, file *outputFile
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

	return env.appendRecord(f, record)
}

// appendRecord adds record to f's file, reporting whether it could.
func (env *Environment) appendRecord(f *outputFile, record string) bool {
	f.records = append(f.records, []byte(record))

	loc, err := env.Session.RewriteRecordFile(f.loc, rms.TextRecords, f.records)
	if err != nil {
		return false
	}

	f.loc = loc

	return true
}

// openRecordFile returns out's file, creating it (a new version) the
// first time, or when out's name has come to name another; nil when the
// name isn't defined in the process table, names a device that holds no
// files, or the file can't be made.
func (env *Environment) openRecordFile(out *processOutput) *outputFile {
	spec := env.processName(out.name)
	if spec == "" || env.Session == nil {
		return nil
	}

	for _, f := range []*outputFile{out.permanent, out.file} {
		if f != nil && f.spec == spec {
			return f
		}
	}

	if device, st := env.deviceName(out.name); st == 0 && !env.isFileDevice(device) {
		return nil // a terminal, mailbox, or NL:
	}

	loc, err := env.Session.Locate(spec, false)
	if err != nil {
		return nil
	}

	found, err := env.Session.CreateRecordFile(loc, rms.TextRecords, nil)
	if err != nil {
		return nil
	}

	out.file = &outputFile{spec: spec, loc: found}

	return out.file
}

// openPermanentOutput makes the CLI's SYS$OUTPUT file, if SYS$OUTPUT
// names one, when a CLI process starts (startInterpreter), so its log
// exists, empty, even if nothing is written to it, as DCL's does; it's
// kept for the process's life.
func (env *Environment) openPermanentOutput() {
	if f := env.openRecordFile(&env.output); f != nil && f == env.output.file {
		env.output.permanent, env.output.file = f, nil
	}
}

// forgetImageOutput is image rundown's step for the files the image made
// (this file's opening comment).
func (env *Environment) forgetImageOutput() {
	env.output.file = nil
	env.errorOutput.file = nil
}
