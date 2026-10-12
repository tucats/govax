package console

import (
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements CREATE with neither /DIRECTORY nor /NAME_TABLE:
// DCL's CREATE of a file (the DCL Dictionary). The new file's records are
// the lines of SYS$INPUT, read to its end of file: at the terminal, what
// is typed up to Ctrl/Z; in a command procedure, the data lines after the
// command, up to the next "$" line (dclinput.go). The file is made even
// when there are no lines.
//
// A file on a mounted volume is what VMS's CREATE makes: variable-length
// records with carriage-return carriage control (rms.TextRecords). A host
// file is plain text, each line ending as the host's text files' lines do
// (LF, or CR LF on Windows).

// CreateFile implements CREATE file: it makes spec, a new file holding
// the lines of SYS$INPUT. host is an explicit /HOST on spec; with log
// (/LOG), the file made is reported:
//
//	%CREATE-I-CREATED, DUA0:[WORK]NOTES.TXT;1 created
//
// Where the file goes is decided before any input is read
// (rms.Session.Locate), so a name that can't be used reads nothing; a
// file that can't be written is CREATE-E-OPENOUT, with its cause.
func (c *cmdContext) CreateFile(spec string, host, log bool) error {
	s := c.ContainerSession

	loc, err := s.Locate(spec, host)
	if err != nil {
		return fileFailure(err, spec)
	}

	created, err := s.CreateRecordFile(loc, rms.TextRecords, c.readInputRecords())
	if err != nil {
		return objectFailureAs(vmserrors.CREATE_OPENOUT, err, spec)
	}

	if log {
		c.Printf("%%%s\n", vmserrors.New(vmserrors.CREATE_CREATED, created.Name))
	}

	return nil
}

// readInputRecords reads the lines of SYS$INPUT up to its end of file,
// as typeInput does, and returns them as records.
func (c *cmdContext) readInputRecords() [][]byte {
	var records [][]byte

	if c.RTL == nil {
		return records
	}

	for c.Running() {
		line, ok := c.RTL.ReadLine(maxInputRecord)
		if !ok {
			break
		}

		records = append(records, []byte(line))
	}

	return records
}
