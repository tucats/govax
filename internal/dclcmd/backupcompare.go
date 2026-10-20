package dclcmd

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// BACKUP/COMPARE (docs/PHASE-69 - backup save sets.md, subtask 10): the
// files of an input save set that /SELECT and /EXCLUDE choose, each held
// up by rms.Session.CompareFile against the file the output specifier
// names for it, by the same rules as a restore's (restoreTarget): [*...]
// the directory each file was saved from, [dir] or [dir...] another, a
// name or type given replacing the saved one, and always the saved
// version. Both specifiers are input (the manual's /COMPARE), so nothing
// is made or changed.
//
// # What VMS V7.1's BACKUP showed (testdata/backup69's log, Part 5)
//
//	%BACKUP-E-VERIFYERR, verification error for block 1 of DUA1:[CMP]TEXT.TXT;1
//	%BACKUP-S-COMPARED, compared DUA1:[CMP]TEXT.TXT;1
//
// VERIFYERR for each block that differs, whether or not /LOG is given,
// and then, with /LOG, COMPARED for the file, even one that differed. A
// file that isn't there is OPENIN, at severity E, with RMS's reason on
// the next line ("-RMS-E-FNF, file not found"), and the compare goes on;
// a compare that found no file ends with NOFILES. $STATUS is the first of
// the most severe messages (VERIFYERR's %X10A38272; OPENIN's %X10A3800A,
// though NOFILES followed), else %X10000001, as a restore's is. A file
// the save set has only part of is VBNMISSING before its COMPARED, as a
// restore shows it (unconfirmed for a compare), and only its blocks the
// save set holds are compared.

// compareSaveSet is BACKUP/COMPARE: the input save set's files against
// the files the output specifier names.
func (c *Context) compareSaveSet(cmd BackupCommand) error {
	in, err := c.ReadSaveSet(cmd.Input, backup.Options{CheckCRC: cmd.CheckCRC})
	if err != nil {
		return backupOpenFailure(err)
	}

	target, err := c.restoreTarget(*cmd.Output)
	if err != nil {
		return FileFailure(err, cmd.Output.Text)
	}

	r := &backupRestore{c: c, log: cmd.Log}
	selection := newBackupSelection(cmd.Select, cmd.Exclude)

	for _, f := range in.Set.Files {
		carried, ok := selection.selected(f.Name)
		if !ok {
			continue
		}

		r.compare(f, target.location(parseSavedName(f.Name), carried))
	}

	return r.finish(in)
}

// compare holds f up against the file at loc, with BACKUP's messages.
// A saved directory file isn't compared, and nothing is said of it: VMS
// V7.1's compares of govax's save sets passed over SUB1.DIR and SUB2.DIR
// (round 2), whose blocks hold file IDs a restore makes anew.
func (r *backupRestore) compare(f *backup.File, loc rms.FileLocation) {
	if f.Directory() {
		return
	}

	result, err := r.c.ContainerSession.CompareFile(loc, f)
	if err != nil {
		r.openInFailure(loc, err)

		return
	}

	for _, vbn := range result.Differences {
		r.message(vmsdef.BackupVerifyErr, vbn, result.Name)
	}

	for _, m := range f.Missing() {
		r.message(vmsdef.BackupVBNMissing, result.Name, m.First, m.Last)
	}

	r.files++

	if r.log {
		r.message(vmsdef.BackupCompared, result.Name)
	}
}

// openInFailure is OPENIN, at severity E, for a file a compare couldn't
// open at loc, with the reason on the lines after it.
func (r *backupRestore) openInFailure(loc rms.FileLocation, err error) {
	name := loc.Name

	var reasons []string

	var fileErr *rms.DCLFileError
	if errors.As(err, &fileErr) {
		name = fileErr.Name
		if fileErr.Status != 0 {
			reasons = append(reasons, ConditionLine(fileErr.Status))
		}

		if fileErr.STV != 0 && fileErr.STV != fileErr.Status {
			reasons = append(reasons, ConditionLine(fileErr.STV))
		}
	} else {
		reasons = append(reasons, "-"+strings.TrimPrefix(FileFailure(err, name).Error(), "%"))
	}

	r.message(vmsdef.BackupOpenIn&^7|2, name)

	for _, reason := range reasons {
		r.c.Printf("%s\n", reason)
	}
}
