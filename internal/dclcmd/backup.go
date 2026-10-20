package dclcmd

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The BACKUP command (docs/PHASE-69 - backup save sets.md, subtask 6):
// the command as its qualifiers' places give it, the operation its
// specifiers name, the input save set opened and read, and BACKUP's
// messages for what goes wrong on the way. The operations themselves are
// the later subtasks': listing (7), restoring (8 and 9), comparing (10),
// and saving (11).
//
// # Where a qualifier stands
//
// VMS's BACKUP reads a qualifier by its place on the command line (the
// System Management Utilities Reference Manual, 7.2): one after the verb
// is a command qualifier (/LIST, /LOG, /COMPARE), one after the input
// specifier an input save-set qualifier (/SELECT, /CRC) or an input
// file-selection qualifier (/SINCE, /EXCLUDE), and one after the output
// specifier an output save-set qualifier (/BLOCK_SIZE, /COMMENT) or an
// output file qualifier (/NEW_VERSION). The grammar gives each specifier
// its own qualifiers (console.dcl's BACKUP), so the console's binding
// hands this file a BackupCommand already sorted.

// BackupDateKind is which of a file's dates /SINCE and /BEFORE test: the
// input file-selection qualifiers /BACKUP, /CREATED, /EXPIRED, and
// /MODIFIED, of which a command may give one. /MODIFIED, the revision
// date, is the default (the manual's /SINCE).
type BackupDateKind int

const (
	BackupModified BackupDateKind = iota
	BackupBackedUp
	BackupCreated
	BackupExpired
)

// BackupExisting is what a restore does with a file that's already there:
// the output file qualifiers /NEW_VERSION, /OVERLAY, and /REPLACE, of
// which a command may give one. With none, the file isn't restored
// (OPENOUT, "-RMS-E-FEX, file already exists, not superseded").
type BackupExisting int

const (
	BackupKeepExisting BackupExisting = iota
	BackupNewVersion
	BackupOverlay
	BackupReplace
)

// BackupCommand is one BACKUP command.
type BackupCommand struct {
	// Command is the command line, which a save records in its summary.
	Command string

	// Input is the input specifier, Inputs its list's elements (Input's
	// Text is the first); Output is the output specifier, nil when the
	// command has none (/LIST alone).
	Input  BackupSpecifier
	Inputs []string
	Output *BackupSpecifier

	// The command qualifiers. ListFile is /LIST's file, "" for
	// SYS$OUTPUT.
	List            bool
	ListFile        string
	Full, Brief     bool
	Log             bool
	Compare         bool
	IgnoreNoBackup  bool
	IgnoreInterlock bool

	// The input save-set qualifiers: /CRC (check each block's CRC),
	// /SELECT, and /EXCLUDE, which is also an input file-selection
	// qualifier.
	CheckCRC bool
	Select   []string
	Exclude  []string

	// The input file-selection qualifiers: /SINCE and /BEFORE as given
	// (TODAY, a VMS time), the date they test, and /BY_OWNER (nil when
	// not given; "" for the process's own UIC).
	Since, Before string
	DateKind      BackupDateKind
	InputOwner    *string

	// The output save-set qualifiers: /CRC, /BLOCK_SIZE and /GROUP_SIZE
	// (nil when not given), /COMMENT, and /BY_OWNER, which as an output
	// file qualifier is a restore's owner option (DEFAULT, ORIGINAL,
	// PARENT, or a UIC; "" for ORIGINAL).
	WriteCRC    bool
	BlockSize   *int
	GroupSize   *int
	Comment     string
	OutputOwner *string

	// Existing is the output file qualifiers' choice.
	Existing BackupExisting
}

// Backup runs a BACKUP command.
func (c *Context) Backup(cmd BackupCommand) error {
	if cmd.Output == nil && !cmd.List {
		// VMS's DCL asks for "_To:" at a terminal, as govax's would for a
		// parameter with a prompt; from a procedure it's INSFPRM.
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	op, err := BackupOperationFor(cmd.Input, cmd.Output, cmd.List, cmd.Compare)
	if err != nil {
		return backupOperationFailure(err)
	}

	switch op {
	case BackupList:
		return c.listSaveSet(cmd)
	case BackupRestore:
		return c.restoreSaveSet(cmd)
	}

	if op != BackupSave {
		if _, err := c.ReadSaveSet(cmd.Input, backup.Options{CheckCRC: cmd.CheckCRC}); err != nil {
			return backupOpenFailure(err)
		}
	}

	return backupMessage(vmsdef.BackupNotYet, backupOperationNames[op])
}

// backupOperationNames name the operations for NOTYET, until each is in
// place.
var backupOperationNames = map[BackupOperation]string{
	BackupCompare: "BACKUP/COMPARE",
	BackupSave:    "a BACKUP save",
}

// backupOperationErrors are BACKUP's messages for the /SAVE_SET rule's
// errors (BackupOperationFor). LISINPSAV is VMS's; the others are
// govax's (vmsdef's BACKUP messages).
var backupOperationErrors = []struct {
	err  error
	code uint32
}{
	{ErrListNeedsSaveSet, vmsdef.BackupLisInpSav},
	{ErrBothSaveSets, vmsdef.BackupBothSaveSets},
	{ErrNoSaveSet, vmsdef.BackupNoSaveSet},
	{ErrCompareToSaveSet, vmsdef.BackupCmpInpSav},
}

// backupOperationFailure is BACKUP's message for err, an error of
// BackupOperationFor.
func backupOperationFailure(err error) error {
	for _, e := range backupOperationErrors {
		if errors.Is(err, e.err) {
			return backupMessage(e.code)
		}
	}

	return err
}

// backupOpenFailure is BACKUP's message for err, an error of
// ReadSaveSet: OPENIN, with RMS's status (and its secondary status) on
// the lines after it, as VMS shows a missing save set
// ("%BACKUP-F-OPENIN, error opening DUA1:[SETS]NONE.BCK; as input",
// "-RMS-E-FNF, file not found"), or NOTSAVESET.
func backupOpenFailure(err error) error {
	var notSaveSet *NotSaveSetError
	if errors.As(err, &notSaveSet) {
		return backupMessage(vmsdef.BackupNotSaveSet, notSaveSet.Name)
	}

	var openErr *SaveSetOpenError
	if !errors.As(err, &openErr) {
		return err
	}

	var fileErr *rms.DCLFileError
	if errors.As(openErr.Cause, &fileErr) {
		var causes []uint32
		if fileErr.Status != 0 {
			causes = append(causes, fileErr.Status)
		}

		if fileErr.STV != 0 && fileErr.STV != fileErr.Status {
			causes = append(causes, fileErr.STV)
		}

		return backupMessageWith(vmsdef.BackupOpenIn, causes, openErr.Name)
	}

	// An error without an RMS status (a device with nothing mounted, a
	// host file that can't be read) follows in the words FileFailure
	// gives it.
	failure := backupMessage(vmsdef.BackupOpenIn, openErr.Name).(vmserrors.ConditionError)
	failure.Text += "\n-" + strings.TrimPrefix(FileFailure(openErr.Cause, openErr.Name).Error(), "%")

	return failure
}

// backupMessage is BACKUP's message code, with its $FAO parameters
// args, as a command's failure: shown as $PUTMSG shows it, with code
// (its message shown) the command's status, as VMS's $STATUS showed
// BACKUP's (%X10A3800C after OPENIN).
func backupMessage(code uint32, args ...any) error {
	return backupMessageWith(code, nil, args...)
}

// backupMessageWith is backupMessage with the conditions causes on the
// lines after it ("-RMS-E-FNF, file not found").
func backupMessageWith(code uint32, causes []uint32, args ...any) error {
	text := vmsdef.FormatMessage(code, args...)
	for _, cause := range causes {
		text += "\n" + ConditionLine(cause)
	}

	return vmserrors.ConditionError{Status: code | stsInhibitMsg, Text: text}
}

// BackupLine is BACKUP's message code as a line of output, with its "%"
// ("%BACKUP-S-CREATED, created DUA1:[R1.B69]ACL.TXT;1"): /LOG's lines
// and the other messages that don't end a command.
func BackupLine(code uint32, args ...any) string {
	return "%" + vmsdef.FormatMessage(code, args...)
}
