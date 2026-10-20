package rms

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/vmstime"
	"github.com/tucats/ods2/volume"
)

// This file is docs/PHASE-69 - backup save sets.md's subtask 8: making
// one file that a BACKUP save set holds (a backup.File) on a mounted
// volume or on the host. BACKUP's command (internal/dclcmd) decides which
// files to restore and what each is called; RestoreFile makes one of them.
//
// # A note for a reader new to VMS
//
// A save set keeps, for each file, its header's attributes (record
// format, end of file, owner, protection, dates, revision count, version
// limit, access control list, and the rest) and the blocks of data it
// uses, up to its end of file. VMS's BACKUP gives the new file all of
// them, so a restored file reads as the saved one: DIRECTORY/FULL of a
// restored tree shows the saved creation and revision dates, the saved
// revision count, and the saved allocation (testdata/backup69's r1.lst,
// beside the tree's own tree.lst). Only the file ID is new, and the
// owner, which BACKUP's /BY_OWNER may change (RestoreOptions.Owner).
//
// A directory file a save set holds (a [dir...] save keeps them, record
// flag DIRECTORY) is made as a new directory with the saved directory's
// default version limit, owner, protection, and allocation, but new dates.
// So are the directories above a file that don't exist yet, with what a
// new directory inherits from its parent. VMS's BACKUP gives every
// directory it makes a revision count of 1 (r1.lst), where CREATE/DIRECTORY
// gives 0.

// RestoreOptions are what RestoreFile can be told beyond the file and its
// name.
type RestoreOptions struct {
	// Owner is the restored file's owner UIC; nil keeps the saved owner.
	// It doesn't apply to the directories made above the file, which take
	// their parents' owners, as CREATE/DIRECTORY's do.
	Owner *ondisk.Uic

	// ParentOwner gives the restored file its directory's owner instead
	// (BACKUP's /BY_OWNER=PARENT); it wins over Owner.
	ParentOwner bool

	// Existing is what to do when the file's version is there already
	// (BACKUP's output file qualifiers). It applies to a volume target
	// with a version; a saved directory that exists is always left alone.
	Existing ExistingFile
}

// ExistingFile is what a restore does with a file that's already there:
// BACKUP's /NEW_VERSION, /OVERLAY, and /REPLACE (the System Management
// Utilities Reference Manual's entries for each).
type ExistingFile int

const (
	// ExistingRefused leaves the file there and restores nothing: an
	// *OutputExistsError (RMS$_FEX), as VMS's BACKUP reports OPENOUT.
	ExistingRefused ExistingFile = iota

	// ExistingNewVersion restores the file as the next version when its
	// version, or a higher one, exists (/NEW_VERSION: VMS made TEXT.TXT;2
	// beside ;1).
	ExistingNewVersion

	// ExistingOverlay writes the file over the existing version, which
	// keeps its file ID and its blocks, extended if the saved file needs
	// more (/OVERLAY).
	ExistingOverlay

	// ExistingReplace deletes the existing version and makes the file
	// anew with the same version (/REPLACE): a new file ID, as VMS's
	// r7.lst shows.
	ExistingReplace
)

// RestoreResult is what RestoreFile made.
type RestoreResult struct {
	// Name is the file made, "DUA1:[R1.B69]ACL.TXT;1" (the device shown as
	// the name it was reached through, as CreateDirectory shows it), or a
	// host path. For a saved directory it's the directory,
	// "DUA1:[R1.B69.SUB1]".
	Name string

	// Directories are the directories made for the file, top down: the
	// ones BACKUP's /LOG reports as CREDIR. For a saved directory made as
	// a new one, it's last.
	Directories []CreatedDirectory

	// Created is false only for a saved directory that already existed,
	// which is left as it is.
	Created bool
}

// ErrNoContiguousSpace is a contiguous file (FCH$V_CONTIG) whose saved
// allocation no single run of free clusters on the volume can hold. It
// wraps ErrACPDeviceFull. VMS's message for it is unconfirmed.
var ErrNoContiguousSpace = fmt.Errorf("%w: no free run of blocks large enough for a contiguous file", ErrACPDeviceFull)

// RestoreFile makes file, saved in a save set, at target: on a mounted
// volume, as the file target names, or on the host, as target's path.
//
// On a volume target must name one file with no wildcards. With no
// version it gets the next version; with one, that version, and if it
// exists already the error is an *OutputExistsError (VMS's RMS$_FEX, "file
// already exists, not superseded"). Every directory in its path that
// doesn't exist is made first. The file gets the saved record attributes
// (its end of file included), characteristics, protection, record
// protection, access level, journaling flags, dates, revision count,
// version limit, and access control list, its saved allocation (more if
// the data needs it, rounded up to the volume's cluster size), and the
// saved data. A contiguous file's allocation must fit in one run of free
// blocks (ErrNoContiguousSpace). A file saved without its data (marked
// NOBACKUP: backup.File.HeaderOnly) gets its attributes and allocation and
// no data: its blocks read as zeros, as its high-water mark is the start
// of the file. If the file can't be made whole, nothing of it is left.
//
// A saved directory file (backup.Attributes.Directory) is made as a
// directory: target is its file in the parent directory, "SUB1.DIR" or
// "SUB1.DIR;1". One that exists already is left as it is (Created false).
//
// On the host the file is written by COPY's rule (docs/PHASE-69's
// Decision 9): an object module's records in the host's object layout, a
// text file's records as lines, anything else its bytes up to its end of
// file. A saved directory is a host directory. The host has no versions,
// so a file that exists already is an *OutputExistsError.
func (s *Session) RestoreFile(target FileLocation, file *backup.File, opts RestoreOptions) (RestoreResult, error) {
	if target.Host {
		return restoreToHost(target.Name, file)
	}

	return s.restoreToVolume(target.Name, file, opts)
}

// restoreToVolume is RestoreFile for a volume target.
func (s *Session) restoreToVolume(text string, file *backup.File, opts RestoreOptions) (RestoreResult, error) {
	specs, err := expandSpec(s.Logicals, text, s.Default)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("restore: %w", err)
	}

	r := specs[0]

	vol, spec, err := s.mounted(r)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("restore: %w", err)
	}

	if spec.Recursive || spec.Name+spec.Type == "" || strings.ContainsAny(spec.Name+spec.Type+spec.Version+strings.Join(spec.Dirs, "."), "*%") {
		return RestoreResult{}, fmt.Errorf("restore: %s: a restored file needs one name, with no wildcards", text)
	}

	version, ok := parseOpenVersion(spec.Version)
	if !ok || version > MaxVersionLimit {
		return RestoreResult{}, fmt.Errorf("restore: %s: version %q: want 1 to %d", text, spec.Version, MaxVersionLimit)
	}

	if !s.Mounts.Writable(spec.Device) {
		return RestoreResult{}, fmt.Errorf("restore: %s: %w", spec.Device, ErrACPWriteLocked)
	}

	if len(vol.Devices) != 1 {
		return RestoreResult{}, fmt.Errorf("restore: %s is a %d-device volume set; only a single-device volume is supported", spec.Device, len(vol.Devices))
	}

	dev := vol.Devices[0]

	bm, err := dev.Bitmap()
	if err != nil {
		return RestoreResult{}, fmt.Errorf("restore: %w", err)
	}

	ib, err := dev.IndexBitmap()
	if err != nil {
		return RestoreResult{}, fmt.Errorf("restore: %w", err)
	}

	display := r.Display
	if display == "" {
		display = spec.Device
	}

	w := &volumeRestore{vol: vol, dev: dev, bm: bm, ib: ib, display: display, file: file}
	result, restoreErr := w.restore(spec, version, opts)

	// The bitmaps are written whatever happened: directories made before
	// a failure stay made, and a file taken back gave its space back.
	if err := bm.Flush(); err != nil && restoreErr == nil {
		restoreErr = err
	}

	if err := ib.Flush(); err != nil && restoreErr == nil {
		restoreErr = err
	}

	if restoreErr != nil {
		return result, fmt.Errorf("restore: %w", restoreErr)
	}

	return result, nil
}

// volumeRestore is one file being restored onto a volume: where it goes,
// and the volume's bitmaps.
type volumeRestore struct {
	vol     *volume.Volume
	dev     *volume.Device
	bm      *volume.Bitmap
	ib      *volume.IndexBitmap
	display string // the device as the user sees it
	file    *backup.File
}

// restore makes w's file as spec names it, at version (0 for the next).
func (w *volumeRestore) restore(spec filespec.Spec, version uint16, opts RestoreOptions) (RestoreResult, error) {
	var result RestoreResult

	made, err := w.makeDirectories(spec.Dirs)
	result.Directories = made

	if err != nil {
		return result, err
	}

	parent, err := filespec.ResolveDirectory(w.vol, spec.Dirs)
	if err != nil {
		return result, err
	}

	if w.file.Directory() {
		return w.restoreDirectory(parent, spec, version, opts, result)
	}

	owner := restoredOwner(w.file, parent, opts)

	saved, err := ondisk.DecodeRecAttr(w.file.RecordAttributes[:])
	if err != nil {
		return result, err
	}

	fullName := spec.Name + "." + spec.Type
	out := filespec.Spec{Device: w.display, Dirs: spec.Dirs, Name: spec.Name, Type: spec.Type}

	version, overlay, err := w.existing(parent, fullName, version, opts.Existing)
	if err != nil {
		out.Version = fmt.Sprint(version)

		return result, fullOutputName(err, out)
	}

	if overlay != nil {
		if err := w.fill(overlay, saved, owner, false); err != nil {
			return result, fmt.Errorf("restoring %s;%d: %w", fullName, version, err)
		}

		out.Version = fmt.Sprint(version)
		result.Name = out.String()
		result.Created = true

		return result, nil
	}

	f, err := createVersion(w.vol, parent, fullName, version, copiedAttributes(saved), w.bm, w.ib)
	if err != nil {
		return result, fullOutputName(err, out)
	}

	made1, err := createdVersion(parent, fullName, version)
	if err != nil {
		return result, err
	}

	if err := w.fill(f, saved, owner, true); err != nil {
		// Nothing of a file that couldn't be made whole is left.
		_ = volume.DeleteFile(parent, fullName, made1, w.bm, w.ib)

		return result, fmt.Errorf("restoring %s;%d: %w", fullName, made1, err)
	}

	out.Version = fmt.Sprint(made1)
	result.Name = out.String()
	result.Created = true

	return result, nil
}

// existing applies rule to fullName;version in parent before a restore
// makes it: the version to create (0 for the next), or, for /OVERLAY of
// a version that's there, that file opened for writing (with version
// unchanged). /REPLACE deletes the version that's there. A version that's
// there and no rule that allows it is an *OutputExistsError. A file that
// some process has open can't be overlaid or replaced (SS$_ACCONFLICT;
// VMS's message for it unconfirmed).
func (w *volumeRestore) existing(parent *volume.Directory, fullName string, version uint16, rule ExistingFile) (uint16, *volume.File, error) {
	if version == 0 {
		return 0, nil, nil
	}

	highest, err := parent.Lookup(fullName, 0)
	if errors.Is(err, volume.ErrNotFound) {
		return version, nil, nil
	}

	if err != nil {
		return version, nil, err
	}

	if rule == ExistingNewVersion {
		// "a file with an identical directory name, file name, type, and
		// equal or higher version number": the next version.
		if highest.Version >= version {
			return 0, nil, nil
		}

		return version, nil, nil
	}

	entry, err := parent.Lookup(fullName, version)
	if errors.Is(err, volume.ErrNotFound) {
		return version, nil, nil
	}

	if err != nil {
		return version, nil, err
	}

	exists := &OutputExistsError{Spec: fmt.Sprintf("%s;%d", fullName, version), version: fmt.Sprint(version)}

	switch rule {
	case ExistingOverlay, ExistingReplace:
		if w.vol.Accessed(entry.Fid) {
			return version, nil, fmt.Errorf("%s;%d: %w", fullName, version, ErrACPAccessConflict)
		}
	default:
		return version, nil, exists
	}

	if rule == ExistingReplace {
		if err := volume.DeleteFile(parent, fullName, version, w.bm, w.ib); err != nil {
			return version, nil, err
		}

		return version, nil, nil
	}

	f, err := w.vol.OpenFID(entry.Fid)
	if err != nil {
		return version, nil, err
	}

	if err := f.OpenForWrite(w.bm, w.ib); err != nil {
		return version, nil, err
	}

	return version, f, nil
}

// restoredOwner is the owner a restore gives file in parent: the saved
// owner, opts.Owner, or with opts.ParentOwner the directory's.
func restoredOwner(file *backup.File, parent *volume.Directory, opts RestoreOptions) ondisk.Uic {
	switch {
	case opts.ParentOwner:
		return parent.File.Header.Owner
	case opts.Owner != nil:
		return *opts.Owner
	}

	return savedOwner(file.Owner)
}

// fill gives f w's file's allocation, data, and header: f just created
// (fresh), or an existing file being overlaid, whose blocks stay where
// they are, extended only if the saved file needs more.
func (w *volumeRestore) fill(f *volume.File, saved ondisk.RecAttr, owner ondisk.Uic, fresh bool) error {
	file := w.file
	data := file.Data()
	blocks := uint32(len(data) / ondisk.BlockSize)

	if allocation := max(file.Allocated, blocks); allocation > f.Blocks() {
		if fresh && file.Characteristics&ondisk.FchContig != 0 && !w.contiguousRoom(allocation) {
			return ErrNoContiguousSpace
		}

		if err := volume.Extend(f, w.bm, w.ib, allocation-f.Blocks()); err != nil {
			return fmt.Errorf("%w: %v", ErrACPDeviceFull, err)
		}
	}

	for vbn := uint32(1); vbn <= blocks; vbn++ {
		start := int(vbn-1) * ondisk.BlockSize
		if err := f.WriteBlock(vbn, data[start:start+ondisk.BlockSize]); err != nil {
			return err
		}
	}

	// Close records how far the data was written (the high-water mark);
	// the saved end of file replaces its own below.
	if err := f.Close(); err != nil {
		return err
	}

	err := volume.UpdateHeader(f, func(h *ondisk.FileHeader, id *ondisk.Ident) {
		h.RecordAttributes = saved
		h.RecordAttributes.VersionLimit = fileVersionLimit(file.VersionLimit)

		// The characteristics are the saved file's, but for the two that
		// describe this header's state rather than the file: a file
		// marked for deletion, or a directory (which restoreDirectory
		// makes).
		h.FileCharacteristics = file.Characteristics &^ (ondisk.FchMarkDel | ondisk.FchDirectory)
		h.Owner = owner
		h.FileProtection = file.Protection
		h.RecordProtection = file.RecordProt
		h.AccessMode = file.AccessLevel
		h.Journaling = file.Journal
		h.RecoveryUnitActive = file.RUActive

		// The high-water mark is the restore's own, the block after the
		// last one written (1 when nothing was): blocks past it read as
		// zeros, not as whatever the disk held there. The save set's
		// HIGHWATER isn't used, since the blocks between the end of file
		// and it weren't saved; what VMS's restore records is
		// unconfirmed. An overlaid file's own mark may lie past the
		// saved data, so it's set from the data.
		if h.HighWaterMark == 0 || !fresh {
			h.HighWaterMark = blocks + 1
		}

		id.Revision = file.Revision
		id.CreationDate = vmstime.VMSTime(file.Created)
		id.RevisionDate = vmstime.VMSTime(file.Revised)
		id.ExpirationDate = vmstime.VMSTime(file.Expires)
		id.BackupDate = vmstime.VMSTime(file.Backup)
	})
	if err != nil {
		return err
	}

	// An overlaid file's own ACL goes, as nil removes it.
	if len(file.ACL) > 0 || !fresh {
		return volume.SetACL(f, file.ACL)
	}

	return nil
}

// contiguousRoom reports whether one run of free clusters can hold blocks
// blocks: how a contiguous file's single extent is found (volume.Extend
// takes the longest run there is, so a run that's long enough gives one
// extent).
func (w *volumeRestore) contiguousRoom(blocks uint32) bool {
	cluster := uint32(max(w.dev.Home.ClusterSize, 1))

	return (blocks+cluster-1)/cluster <= w.bm.LargestFreeRun()
}

// restoreDirectory is restore for a saved directory file: the directory
// spec's name names, in parent.
func (w *volumeRestore) restoreDirectory(parent *volume.Directory, spec filespec.Spec, version uint16, opts RestoreOptions, result RestoreResult) (RestoreResult, error) {
	if !strings.EqualFold(spec.Type, "DIR") || version > 1 {
		return result, fmt.Errorf("%s.%s;%d: a directory is restored as NAME.DIR;1", spec.Name, spec.Type, version)
	}

	dirs := append(append([]string(nil), spec.Dirs...), strings.ToUpper(spec.Name))
	result.Name = filespec.Spec{Device: w.display, Dirs: dirs}.String()

	fullName := spec.Name + ".DIR"
	if _, err := parent.Lookup(fullName, 0); err == nil {
		return result, nil
	} else if !errors.Is(err, volume.ErrNotFound) {
		return result, err
	}

	saved, err := ondisk.DecodeRecAttr(w.file.RecordAttributes[:])
	if err != nil {
		return result, err
	}

	owner := restoredOwner(w.file, parent, opts)
	protection := w.file.Protection

	d, err := w.vol.CreateDirectory(parent, fullName, volume.DirectoryOptions{
		VersionLimit: saved.VersionLimit,
		Owner:        &owner,
		Protection:   &protection,
		Allocation:   w.file.Allocated,
	}, w.bm, w.ib)
	if err != nil {
		return result, err
	}

	if err := markBackupDirectory(d); err != nil {
		return result, err
	}

	result.Directories = append(result.Directories, CreatedDirectory{Name: result.Name, Created: true})
	result.Created = true

	return result, nil
}

// makeDirectories makes each directory of dirs that doesn't exist, top
// down, with what a new directory inherits from its parent, and returns
// the ones it made.
func (w *volumeRestore) makeDirectories(dirs []string) ([]CreatedDirectory, error) {
	levels, err := filespec.CreateDirectoryPath(w.vol, dirs, volume.InheritedDirectoryOptions, w.bm, w.ib)

	var made []CreatedDirectory

	for _, l := range levels {
		if !l.Created {
			continue
		}

		if markErr := markBackupDirectory(l.Directory); markErr != nil && err == nil {
			err = markErr
		}

		made = append(made, CreatedDirectory{Name: w.display + ":" + l.String(), Created: true})
	}

	return made, err
}

// markBackupDirectory gives a directory a restore made the revision count
// VMS's BACKUP gives one: 1 (r1.lst).
func markBackupDirectory(d *volume.Directory) error {
	return volume.UpdateHeader(d.File, func(_ *ondisk.FileHeader, id *ondisk.Ident) {
		id.Revision = 1
	})
}

// savedOwner is a save set's UIC, group in the high word and member in
// the low, as an ondisk.Uic.
func savedOwner(uic uint32) ondisk.Uic {
	return ondisk.Uic{Group: uint16(uic >> 16), Member: uint16(uic)}
}

// fileVersionLimit is a saved file's version limit (its VERLIMIT
// attribute) as a header holds it: a save set writes 32767 for a file
// without one, which ods2 keeps as 0.
func fileVersionLimit(limit uint16) uint16 {
	if limit == ondisk.NoVersionLimit {
		return 0
	}

	return limit
}

// restoreToHost is RestoreFile for a host target, path.
func restoreToHost(path string, file *backup.File) (RestoreResult, error) {
	if file.Directory() {
		made, err := makeHostDirectories(path)
		if err != nil {
			return RestoreResult{Directories: made}, fmt.Errorf("restore: %w", err)
		}

		return RestoreResult{Name: path, Directories: made, Created: len(made) > 0}, nil
	}

	if _, err := os.Lstat(path); err == nil {
		return RestoreResult{}, fmt.Errorf("restore: %w", &OutputExistsError{Spec: path})
	}

	made, err := makeHostDirectories(filepath.Dir(path))
	if err != nil {
		return RestoreResult{Directories: made}, fmt.Errorf("restore: %w", err)
	}

	result := RestoreResult{Name: path, Directories: made, Created: true}

	saved, err := ondisk.DecodeRecAttr(file.RecordAttributes[:])
	if err != nil {
		return RestoreResult{}, fmt.Errorf("restore: %w", err)
	}

	content, err := hostContent(path, file, saved)
	if err != nil {
		return RestoreResult{Directories: made}, fmt.Errorf("restore: %s: %w", path, err)
	}

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return RestoreResult{Directories: made}, fmt.Errorf("restore: %w", &OutputExistsError{Spec: path})
	}

	if err != nil {
		return RestoreResult{Directories: made}, fmt.Errorf("restore: %w", err)
	}

	_, err = out.Write(content)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		_ = os.Remove(path)

		return RestoreResult{Directories: made}, fmt.Errorf("restore: %w", err)
	}

	// The host file's time is the saved revision time, as COPY/TIME gives
	// a copy its source's. Not being able to set it isn't a failure.
	if file.Revised != 0 {
		t := vmstime.VMSTime(file.Revised).Time()
		_ = os.Chtimes(path, t, t)
	}

	return result, nil
}

// hostContent is what a restore to the host writes for file, whose
// record attributes are saved, at path: by COPY's rule, an object
// module's records (by path's type, hostRecordTypes) in the host's object
// layout, a text file's records as lines, anything else its bytes up to
// its end of file. A file saved without its data is empty.
func hostContent(path string, file *backup.File, saved ondisk.RecAttr) ([]byte, error) {
	data := file.Data()

	length := min(odsrms.FileByteLength(saved), int64(len(data)))
	data = data[:length]

	_, typ := hostBaseNameType(path)
	if i := strings.IndexByte(typ, ';'); i >= 0 {
		typ = typ[:i]
	}

	_, objectType := hostRecordTypes[typ]

	switch {
	case len(data) == 0:
		return nil, nil

	case objectType && saved.Format == ondisk.RecordFormatVariable:
		r, err := odsrms.NewReaderFrom(odsrms.BytesSource(data), saved)
		if err != nil {
			return nil, err
		}

		var records [][]byte

		for {
			rec, err := r.Next()
			if err != nil {
				if errors.Is(err, odsrms.ErrCorruptRecord) {
					return nil, err
				}

				break
			}

			records = append(records, rec)
		}

		var b bytes.Buffer
		if err := obj.WriteRecords(&b, records); err != nil {
			return nil, err
		}

		return b.Bytes(), nil

	case restoredAsText(saved):
		r, err := odsrms.NewReaderFrom(odsrms.BytesSource(data), saved)
		if err != nil {
			return nil, err
		}

		var b bytes.Buffer
		if err := writeRecordsFrom(&b, r, saved, lfLineEnding); err != nil {
			return nil, err
		}

		return b.Bytes(), nil
	}

	return data, nil
}

// restoredAsText reports whether a file with record attributes attr is a
// text file for a restore to the host: a sequential file of records with
// carriage control (implied, print file, or FORTRAN), whose records
// become lines. An image (fixed 512-byte records with none), an indexed
// file, or a file of undefined format is bytes. COPY treats every file
// but an object module as text; BACKUP's restore to the host, which VMS
// doesn't have, draws the line here (govax's choice).
func restoredAsText(attr ondisk.RecAttr) bool {
	switch attr.Format {
	case ondisk.RecordFormatFixed, ondisk.RecordFormatVariable, ondisk.RecordFormatVFC,
		ondisk.RecordFormatStreamCRLF, ondisk.RecordFormatStreamLF, ondisk.RecordFormatStreamCR:
	default:
		return false
	}

	return attr.Attributes&(ondisk.AttrFortranCC|ondisk.AttrImpliedCC|ondisk.AttrPrintCC) != 0
}

// makeHostDirectories makes the host directory path and each missing one
// above it, and returns the ones it made, top down.
func makeHostDirectories(path string) ([]CreatedDirectory, error) {
	var missing []string

	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		if _, err := os.Stat(p); err == nil {
			break
		}

		missing = append(missing, p)

		if filepath.Dir(p) == p {
			break
		}
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}

	made := make([]CreatedDirectory, 0, len(missing))
	for i := len(missing) - 1; i >= 0; i-- {
		made = append(made, CreatedDirectory{Name: missing[i], Created: true})
	}

	return made, nil
}
