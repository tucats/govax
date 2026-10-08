package corevms

import (
	"github.com/tucats/govax/internal/rms"
)

// SYS$OUTPUT as a file (docs/PHASE-48.md, subtask 2).
//
// A process LIB$SPAWN or $CREPRC creates with an output file (a log, say,
// LIB$SPAWN's output-file) has SYS$OUTPUT translating to that file's name
// rather than to a device. On VMS the process's CLI creates the file when
// it starts and keeps it open as a process-permanent file, and every line
// written to SYS$OUTPUT is a record in it. govax creates the file at the
// first line written and writes each line through as it comes (the file
// is written again whole, keeping its version), so that whoever reads it
// later, the creator after the process ends, sees every line.

// outputFile is SYS$OUTPUT as a file: the name it was opened for, where
// the file is, and its records so far.
type outputFile struct {
	spec    string
	loc     rms.FileLocation
	records [][]byte
}

// putOutputFile writes record to SYS$OUTPUT when SYS$OUTPUT is a file,
// reporting whether it was one. A file that can't be made or written is
// no file: the line goes to the terminal, as it would with no SYS$OUTPUT.
func (env *Environment) putOutputFile(record string) bool {
	f := env.openOutputFile()
	if f == nil {
		return false
	}

	f.records = append(f.records, []byte(record))

	loc, err := env.Session.RewriteRecordFile(f.loc, rms.TextRecords, f.records)
	if err != nil {
		return false
	}

	f.loc = loc

	return true
}

// openOutputFile returns SYS$OUTPUT's file, creating it (a new version)
// the first time, or when SYS$OUTPUT has come to name another; nil when
// SYS$OUTPUT isn't defined in the process table or the file can't be
// made. A CLI process calls it when it starts (startInterpreter), so its
// log exists, empty, even if nothing is written to it, as DCL's does.
func (env *Environment) openOutputFile() *outputFile {
	spec := env.processName("SYS$OUTPUT")
	if spec == "" || env.Session == nil {
		return nil
	}

	if f := env.outputFile; f != nil && f.spec == spec {
		return f
	}

	if device, st := env.deviceName("SYS$OUTPUT"); st == 0 && !env.isFileDevice(device) {
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

	env.outputFile = &outputFile{spec: spec, loc: found}

	return env.outputFile
}
