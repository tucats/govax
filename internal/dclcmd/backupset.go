package dclcmd

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/rms"
)

// BACKUP's save sets as its specifiers name them (docs/PHASE-69 - backup
// save sets.md, subtask 5): which operation a command's input and output
// specifiers ask for, and the input save set opened and read.
//
// On a disk, a specifier is a file specification unless it has
// /SAVE_SET, which makes it a save set (the System Management Utilities
// Reference Manual, 7.3): BACKUP's operation follows from which of the
// two is one (Table 7-1). A save set as input is a restore, or with
// /COMPARE a compare; a save set as output is a save. /LIST with no
// output specifier lists the input, which must then be a save set
// (LISINPSAV, "/LIST requires save set as input", as VMS V7.1 said to
// "BACKUP/LIST [SETS]TEXT.BCK"). Both specifiers can't be save sets.

// BackupSpecifier is one of BACKUP's specifiers: its text, /HOST (a host
// file whatever the name looks like), and /SAVE_SET.
type BackupSpecifier struct {
	Text    string
	Host    bool
	SaveSet bool
}

// BackupOperation is what a BACKUP command does.
type BackupOperation int

const (
	// BackupList lists a save set: /LIST with no output specifier.
	BackupList BackupOperation = iota + 1

	// BackupRestore restores files from the input save set.
	BackupRestore

	// BackupCompare compares the input save set's files with files.
	BackupCompare

	// BackupSave saves files into the output save set.
	BackupSave
)

// The errors of the /SAVE_SET rule.
var (
	// ErrListNeedsSaveSet is LISINPSAV: /LIST with no output specifier,
	// and an input specifier that isn't a save set.
	ErrListNeedsSaveSet = errors.New("/LIST requires save set as input")

	// ErrBothSaveSets is both specifiers save sets, which the manual
	// says BACKUP refuses (its message unconfirmed).
	ErrBothSaveSets = errors.New("input and output cannot both be save sets")

	// ErrNoSaveSet is neither specifier a save set: a copy or compare of
	// files to files, which govax's BACKUP doesn't do (the Goal).
	ErrNoSaveSet = errors.New("neither specifier is a save set")

	// ErrCompareToSaveSet is /COMPARE with a save set as the output
	// specifier; Table 7-1's compares take a save set only as input.
	ErrCompareToSaveSet = errors.New("/COMPARE takes a save set only as input")
)

// BackupOperationFor is the operation a BACKUP command's specifiers ask
// for, with /LIST and /COMPARE as given. output is nil when the command
// has no output specifier, which only /LIST may leave out (the grammar
// requires it otherwise). With /LIST and an output specifier the
// operation is the one the specifiers name, and its save set is listed
// too (the manual's "can also be used with any other operation").
func BackupOperationFor(input BackupSpecifier, output *BackupSpecifier, list, compare bool) (BackupOperation, error) {
	if output == nil {
		if !list || !input.SaveSet {
			return 0, ErrListNeedsSaveSet
		}

		return BackupList, nil
	}

	switch {
	case input.SaveSet && output.SaveSet:
		return 0, ErrBothSaveSets
	case input.SaveSet && compare:
		return BackupCompare, nil
	case input.SaveSet:
		return BackupRestore, nil
	case output.SaveSet && compare:
		return 0, ErrCompareToSaveSet
	case output.SaveSet:
		return BackupSave, nil
	}

	return 0, ErrNoSaveSet
}

// SaveSetInput is an input save set, read.
type SaveSetInput struct {
	// Name is the save set file's full name with its version
	// ("DUA1:[SETS]B69.BCK;1"), or the host path. Expanded is the name
	// as given, expanded ("DUA1:[SETS]B69.BCK;"), as VMS names the save
	// set in NOFILES (XORERRS names it by Name); the host path for a host
	// file.
	Name     string
	Expanded string

	// Set is what it holds.
	Set *backup.SaveSet

	// RecordsMatch is false for a volume file whose records aren't the
	// fixed-length records of the save set's block size VMS writes (a
	// save set copied with COPY/BINARY, say). It's read anyway
	// (Decision 3); what VMS says of one is unconfirmed.
	RecordsMatch bool
}

// SaveSetOpenError is BACKUP's OPENIN for an input save set that can't
// be opened: Name is the name VMS shows ("DUA1:[SETS]NONE.BCK;"), and
// Cause the RMS error, a *rms.DCLFileError when it has an RMS status
// (RMS$_FNF: "-RMS-E-FNF, file not found").
type SaveSetOpenError struct {
	Name  string
	Cause error
}

func (e *SaveSetOpenError) Error() string {
	return fmt.Sprintf("error opening %s as input: %v", e.Name, e.Cause)
}

func (e *SaveSetOpenError) Unwrap() error { return e.Cause }

// NotSaveSetError is BACKUP's NOTSAVESET: the file opened, Name its full
// name with its version ("DUA1:[B69]TEXT.TXT;2"), isn't a save set.
type NotSaveSetError struct {
	Name string
}

func (e *NotSaveSetError) Error() string {
	return e.Name + " is not a BACKUP save set"
}

func (e *NotSaveSetError) Unwrap() error { return backup.ErrNotSaveSet }

// ReadSaveSet opens and reads the input save set spec names, by govax's
// rules for host and volume names (rms.Session.Locate; /HOST forces the
// host), with opts (/CRC). A file it can't open is a *SaveSetOpenError,
// and one that isn't a save set a *NotSaveSetError. The file is closed
// again before it returns: the save set is read whole.
func (c *Context) ReadSaveSet(spec BackupSpecifier, opts backup.Options) (*SaveSetInput, error) {
	s := c.ContainerSession

	loc, err := s.Locate(spec.Text, spec.Host)
	if err != nil {
		return nil, &SaveSetOpenError{Name: expandedSaveSetName(s, rms.FileLocation{Name: spec.Text}), Cause: err}
	}

	expanded := expandedSaveSetName(s, loc)

	f, err := s.OpenSaveSet(loc)
	if err != nil {
		var fileErr *rms.DCLFileError
		if errors.As(err, &fileErr) {
			expanded = fileErr.Name
		}

		return nil, &SaveSetOpenError{Name: expanded, Cause: err}
	}

	defer func() { _ = f.Close() }()

	set, err := backup.Read(f, f.Size, opts)
	if errors.Is(err, backup.ErrNotSaveSet) {
		return nil, &NotSaveSetError{Name: f.Name}
	}

	if err != nil {
		return nil, &SaveSetOpenError{Name: expanded, Cause: err}
	}

	return &SaveSetInput{
		Name:         f.Name,
		Expanded:     expanded,
		Set:          set,
		RecordsMatch: f.RecordsMatch(set.BlockSize),
	}, nil
}

// expandedSaveSetName is loc's name as VMS's BACKUP messages name a save
// set: a volume name expanded, with ";" for no version
// ("DUA1:[SETS]B69.BCK;"), a host path as it is.
func expandedSaveSetName(s *rms.Session, loc rms.FileLocation) string {
	if loc.Host {
		return loc.Name
	}

	name, err := s.ExpandName(loc.Name)
	if err != nil {
		return loc.Name
	}

	return name
}
