package dclcmd

import (
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/security"
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

// CreateFileRequest is CREATE's command line for a file: its
// specification and qualifiers, as typed.
type CreateFileRequest struct {
	// File is the file's specification; Host an explicit /HOST on it.
	File string
	Host bool

	// Log is /LOG: report the file made.
	Log bool

	// OwnerUIC is /OWNER_UIC's UIC as typed, "" for none: the file's
	// owner, which only a process that may give it can name.
	OwnerUIC string

	// Protection is /PROTECTION's categories, "S:RWED" and so on, laid
	// over the protection the file would otherwise get; nil without it.
	Protection []string
}

// CreateFile implements CREATE file: it makes req.File, a new file
// holding the lines of SYS$INPUT. With /LOG, the file made is reported:
//
//	%CREATE-I-CREATED, DUA0:[WORK]NOTES.TXT;1 created
//
// Where the file goes is decided before any input is read
// (rms.Session.Locate), so a name that can't be used reads nothing; a
// file that can't be written is CREATE-E-OPENOUT, with its cause.
//
// /PROTECTION is merged into what the file would get without it (the
// previous version's, the directory's default protection ACE, or the
// process default), as VMS 7.1's was; a code giving no category any
// access is taken as none given, by the DCL Dictionary's note ("the
// system default RMS protection values"; unconfirmed). /OWNER_UIC names
// the owner: another's UIC, without the privilege to give it, is
// CREATE-E-OPENOUT over RMS$_PRV, nothing made (A.4, round 2).
func (c *Context) CreateFile(req CreateFileRequest) error {
	s := c.ContainerSession

	explicit, err := c.createFileProfile(req)
	if err != nil {
		return err
	}

	loc, err := s.LocateOutput(req.File, req.Host)
	if err != nil {
		return FileFailure(err, req.File)
	}

	// A file the process may not make is refused before its input is
	// read, as VMS 7.1's was (docs/planning/PHASE-93 - file
	// protection.md, A.4): CREATE-E-OPENOUT over RMS$_PRV.
	if err := s.CreateAllowedAs(loc, explicit); err != nil {
		return openFailure(FileFailure(err, req.File), "CREATE-E", false)
	}

	created, err := s.CreateRecordFileAs(loc, rms.TextRecords, c.readInputRecords(), explicit)
	if err != nil {
		if _, denied := rms.IsDenied(err); denied {
			return openFailure(FileFailure(err, req.File), "CREATE-E", false)
		}

		return objectFailureAs(vmserrors.CREATE_OPENOUT, err, req.File)
	}

	if req.Log {
		c.Printf("%%%s\n", vmserrors.New(vmserrors.CREATE_CREATED, created.Name))
	}

	return nil
}

// createFileProfile reads CREATE's /OWNER_UIC and /PROTECTION, each
// refused as CREATE/DIRECTORY refuses it: a UIC that doesn't resolve
// "%CREATE-F-SYNTAX, error parsing '[NOSUCH]'" over SS$_NOSUCHID or
// SS$_IVIDENT, a code's bad letters %CREATE-F-SYNTAX naming them, an
// unknown category DCL's IVKEYW.
func (c *Context) createFileProfile(req CreateFileRequest) (rms.ExplicitProfile, error) {
	var explicit rms.ExplicitProfile

	syntax := func(value string, secondary uint32) error {
		c.Printf("%%%s\n", vmserrors.New(vmserrors.CREATE_SYNTAX, value))

		if secondary != 0 {
			c.Printf("%s\n", ConditionLine(secondary))
		}

		return vmserrors.InhibitMessage(vmserrors.New(vmserrors.CREATE_SYNTAX, value))
	}

	if owner := strings.TrimSpace(req.OwnerUIC); owner != "" {
		uic, err := security.ParseUIC(owner, c.identifierLookup())
		if err != nil {
			return explicit, syntax(owner, uicStatus(err))
		}

		explicit.Owner = uic
	}

	code, err := qualifierProtection(req.Protection, func(part string) error { return syntax(part, 0) })
	if err != nil {
		return explicit, err
	}

	// Every bit set is no access for every category: taken as no code.
	if code != "" {
		if p, _ := security.ParseProtection(code, 0); p != 0xFFFF {
			explicit.Protection = rms.MergedProtection(code)
		}
	}

	return explicit, nil
}

// readInputRecords reads the lines of SYS$INPUT up to its end of file,
// as typeInput does, and returns them as records.
func (c *Context) readInputRecords() [][]byte {
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
