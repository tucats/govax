package dclcmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/ondisk"
)

// BACKUP's restore (docs/PHASE-69 - backup save sets.md, subtask 9): the
// files of an input save set that /SELECT and /EXCLUDE choose, each made
// by rms.Session.RestoreFile under the name the output specifier gives
// it, with /BY_OWNER's owner and the output file qualifiers' rule for a
// file that's there already, and BACKUP's messages as VMS V7.1's BACKUP
// V6.2 showed them (testdata/backup69's log, Part 4).
//
// # The output specifier's directory (the manual's 7.3.3)
//
//	omitted, or []   the default directory
//	[*...]           the directory each file was saved from
//	[dir]            dir, every file in it (a saved directory file too:
//	                 [B69]SUB1.DIR went to [R4]SUB1.DIR, so [R4.SUB1])
//	[dir...]         dir, with the part of each file's directory that
//	                 selection carries (backupSelection.selected) below it
//
// A name, type, or version the output specifier gives replaces the saved
// one ([R9]NEW.TXT); one it leaves out or makes "*" keeps it, and so a
// restored file keeps its saved version (MULTI.TXT;3, ;2, and ;1).
//
// A host output specifier (govax's; VMS has no host) names a directory,
// like [dir]; a last element of "..." ("out/...") is [dir...]'s form.
// Files are written there by COPY's rule (Decision 9) and named as COPY
// names a file it writes into a host directory, "NAME.TYPE;1".
//
// # What /LOG and the rest show
//
// With /LOG, CREATED names each file made, and CREDIR the directory made
// for it, once, naming the deepest level: VMS made [R1] and [R1.B69] for
// the first file of the R1 restore and said only "created directory
// DUA1:[R1.B69]". A saved directory file that's made is a CREDIR of the
// directory; one that's there already says nothing (unconfirmed). A file
// the save set has only part of is VBNMISSING, before its CREATED,
// whether or not /LOG is given. A file that can't be made is OPENOUT
// with the reason on the next line ("-RMS-E-FEX, file already exists,
// not superseded"), and the restore goes on with the next file. After
// the files come XORERRS for blocks rebuilt from their redundancy
// groups, and NOFILES ("no files selected from DUA1:[SETS]B69.BCK;") when
// no file was made, even when files were selected and failed. $STATUS is
// the most severe of the messages, the first one of that severity (VMS
// gave OPENOUT's %X10A38012 though NOFILES followed it, and VBNMISSING's
// though files were made after it), or %X10000001.
//
// Not done here: /LIST with a restore (the manual's "can also be used
// with any other operation"); the restore runs and nothing is listed.

// restoreSaveSet is a BACKUP restore: the input save set's files to the
// output specifier.
func (c *Context) restoreSaveSet(cmd BackupCommand) error {
	in, err := c.ReadSaveSet(cmd.Input, backup.Options{CheckCRC: cmd.CheckCRC})
	if err != nil {
		return backupOpenFailure(err)
	}

	target, err := c.restoreTarget(*cmd.Output)
	if err != nil {
		return FileFailure(err, cmd.Output.Text)
	}

	opts, err := c.restoreOptions(cmd)
	if err != nil {
		return err
	}

	r := &backupRestore{c: c, log: cmd.Log}
	selection := newBackupSelection(cmd.Select, cmd.Exclude)

	for _, f := range in.Set.Files {
		carried, ok := selection.selected(f.Name)
		if !ok {
			continue
		}

		r.restore(f, target.location(parseSavedName(f.Name), carried), opts)
	}

	if n := len(in.Set.Recovered); n > 0 {
		r.message(vmsdef.BackupXorErrs, n, in.Name)
	}

	if r.created == 0 {
		r.message(vmsdef.BackupNoFiles, in.Expanded)
	}

	if r.status == 0 {
		if c.SetStatus != nil {
			c.SetStatus(ssNormal | stsInhibitMsg)
		}

		return nil
	}

	// Each message has been shown; the command's status is the worst.
	return vmserrors.InhibitMessage(vmserrors.ConditionError{
		Status: r.status | stsInhibitMsg, Text: vmsdef.FormatMessage(r.status),
	})
}

// restoreOptions are the rms.RestoreOptions cmd's output file qualifiers
// ask for: /BY_OWNER's owner and the rule for an existing file.
// /BY_OWNER's options (the manual's output file qualifier): DEFAULT, the
// default, is the process's UIC (VMS gave OWNED.TXT, saved as [200,201],
// SYSTEM's [1,4]); ORIGINAL, the default with no option, the saved
// owner; PARENT the directory's owner; or a UIC. A bad UIC is
// SS$_IVIDENT (VMS's message unconfirmed).
func (c *Context) restoreOptions(cmd BackupCommand) (rms.RestoreOptions, error) {
	opts := rms.RestoreOptions{Existing: map[BackupExisting]rms.ExistingFile{
		BackupKeepExisting: rms.ExistingRefused,
		BackupNewVersion:   rms.ExistingNewVersion,
		BackupOverlay:      rms.ExistingOverlay,
		BackupReplace:      rms.ExistingReplace,
	}[cmd.Existing]}

	option := "DEFAULT"
	if cmd.OutputOwner != nil {
		option = strings.ToUpper(strings.TrimSpace(*cmd.OutputOwner))
	}

	switch option {
	case "", "ORIGINAL":
	case "DEFAULT":
		uic := savedUIC(c.processUIC())
		opts.Owner = &uic
	case "PARENT":
		opts.ParentOwner = true
	default:
		uic, err := ondisk.ParseUic(option)
		if err != nil {
			return opts, vmserrors.ConditionError{Status: ssIVIDENT, Text: strings.TrimPrefix(ConditionLine(ssIVIDENT), "-")}
		}

		opts.Owner = &uic
	}

	return opts, nil
}

// processUIC is the process's UIC, SYSTEM's when there's no process.
func (c *Context) processUIC() uint32 {
	if c.RTL != nil && c.RTL.Process != nil && c.RTL.Process.UIC != 0 {
		return c.RTL.Process.UIC
	}

	return systemUIC
}

// savedUIC is a UIC, group in the high word, as an ondisk.Uic.
func savedUIC(uic uint32) ondisk.Uic {
	return ondisk.Uic{Group: uint16(uic >> 16), Member: uint16(uic)}
}

// The forms of an output specifier's directory.
type restoreDirectoryForm int

const (
	restoreNamed restoreDirectoryForm = iota // [dir], [], or none
	restoreTree                              // [dir...]
	restoreSaved                             // [*...]
)

// backupTarget is a restore's output specifier, parsed.
type backupTarget struct {
	host bool
	form restoreDirectoryForm

	// device and dirs are a volume target's, as the default directory
	// and logical names expand them; path is a host target's directory.
	device string
	dirs   []string
	path   string

	// name, typ, and version replace the saved ones unless "" or wild.
	name, typ, version string
}

// restoreTarget parses a restore's output specifier, by govax's rules for
// host and volume names (rms.Session.Locate).
func (c *Context) restoreTarget(spec BackupSpecifier) (backupTarget, error) {
	s := c.ContainerSession

	loc, err := s.Locate(spec.Text, spec.Host)
	if err != nil {
		return backupTarget{}, err
	}

	if loc.Host {
		t := backupTarget{host: true, path: loc.Name}
		if filepath.Base(loc.Name) == "..." {
			t.form, t.path = restoreTree, filepath.Dir(loc.Name)
		}

		return t, nil
	}

	expanded, err := s.ExpandName(loc.Name)
	if err != nil {
		return backupTarget{}, err
	}

	var t backupTarget

	device, rest, _ := strings.Cut(expanded, ":")
	t.device = device

	dir, rest, _ := splitBackupDirectory(rest)
	t.dirs = directoryComponents(dir)

	switch n := len(t.dirs); {
	case n == 2 && t.dirs[0] == "*" && t.dirs[1] == ellipsisToken:
		t.form, t.dirs = restoreSaved, nil
	case n > 0 && t.dirs[n-1] == ellipsisToken:
		t.form, t.dirs = restoreTree, t.dirs[:n-1]
	}

	for _, d := range t.dirs {
		if d == ellipsisToken || strings.ContainsAny(d, "*%") {
			return backupTarget{}, fmt.Errorf("%s: an output directory may be [dir], [dir...], or [*...]", spec.Text)
		}
	}

	name, version, _ := strings.Cut(rest, ";")
	t.name, t.typ, _ = strings.Cut(name, ".")
	t.version = version

	return t, nil
}

// location is where t puts a saved file, saved its name split and
// carried what selection carries of its directory.
func (t backupTarget) location(saved savedName, carried []string) rms.FileLocation {
	name := keepSaved(t.name, saved.name)
	typ := keepSaved(t.typ, saved.typ)
	version := keepSaved(t.version, saved.version)

	var dirs []string

	switch t.form {
	case restoreNamed:
		dirs = t.dirs
	case restoreTree:
		dirs = append(append([]string(nil), t.dirs...), carried...)
	case restoreSaved:
		dirs = saved.dirs
	}

	if t.host {
		file := name + "." + typ + ";" + version
		if typ == "DIR" {
			// A saved directory is a host directory of its name.
			file = name
		}

		return rms.FileLocation{Host: true, Name: filepath.Join(append(append([]string{t.path}, dirs...), file)...)}
	}

	dir := strings.Join(dirs, ".")
	if dir == "" {
		dir = "000000"
	}

	return rms.FileLocation{Name: fmt.Sprintf("%s:[%s]%s.%s;%s", t.device, dir, name, typ, version)}
}

// keepSaved is the output specifier's part given, or the saved one when
// it's left out or wild.
func keepSaved(given, saved string) string {
	if given == "" || strings.ContainsAny(given, "*%") {
		return saved
	}

	return given
}

// backupRestore is one restore's progress: what it has said and made.
type backupRestore struct {
	c   *Context
	log bool

	created int

	// status is the most severe message's code so far, 0 for none worse
	// than informational.
	status uint32
}

// restore makes f at loc, with BACKUP's messages.
func (r *backupRestore) restore(f *backup.File, loc rms.FileLocation, opts rms.RestoreOptions) {
	result, err := r.c.ContainerSession.RestoreFile(loc, f, opts)

	if r.log && len(result.Directories) > 0 {
		r.message(vmsdef.BackupCreDir, result.Directories[len(result.Directories)-1].Name)
	}

	if err != nil {
		r.openOutFailure(loc, err)

		return
	}

	if !result.Created || f.Directory() {
		return
	}

	for _, m := range f.Missing() {
		r.message(vmsdef.BackupVBNMissing, result.Name, m.First, m.Last)
	}

	r.created++

	if r.log {
		r.message(vmsdef.BackupCreated, result.Name)
	}
}

// openOutFailure is OPENOUT for a file a restore couldn't make at loc,
// with the reason on the line after it.
func (r *backupRestore) openOutFailure(loc rms.FileLocation, err error) {
	name := loc.Name

	var reason string

	var exists *rms.OutputExistsError

	switch {
	case errors.As(err, &exists):
		name, reason = exists.Spec, ConditionLine(rmsFEX)
	case errors.Is(err, rms.ErrACPDeviceFull):
		reason = ConditionLine(vmsdef.Symbols["SS$_DEVICEFULL"])
	case errors.Is(err, rms.ErrACPWriteLocked):
		reason = ConditionLine(vmsdef.Symbols["SS$_WRITLCK"])
	case errors.Is(err, rms.ErrACPAccessConflict):
		reason = ConditionLine(vmsdef.Symbols["SS$_ACCONFLICT"])
	default:
		reason = "-" + strings.TrimPrefix(FileFailure(err, name).Error(), "%")
	}

	r.message(vmsdef.BackupOpenOut, name)
	r.c.Printf("%s\n", reason)
}

// message shows one of BACKUP's messages, noting its severity for the
// command's status.
func (r *backupRestore) message(code uint32, args ...any) {
	r.c.Printf("%s\n", BackupLine(code, args...))

	if backupSeverityRank(code) > backupSeverityRank(r.status) {
		r.status = code
	}
}

// backupSeverityRank orders conditions by severity for a restore's
// status: fatal, error, warning, and then the rest (0 none at all).
func backupSeverityRank(code uint32) int {
	if code == 0 {
		return 0
	}

	switch code & 7 {
	case 4:
		return 4
	case 2:
		return 3
	case 0:
		return 2
	}

	return 0
}
