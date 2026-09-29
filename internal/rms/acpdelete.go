package rms

import (
	"errors"
	"fmt"

	"github.com/tucats/ods2/volume"
)

// Deleting files through the ACP (docs/PHASE-26.md subtask 44).
//
// # What deleting involves
//
// As creating a file makes two things, a header and (usually) a directory
// entry (see acpcreate.go), deleting undoes them separately: IO$_DELETE
// removes a directory entry, and, with IO$M_DELETE, deletes the file
// itself, freeing its header slot and its blocks. Removing only the entry
// (DCL's SET FILE/REMOVE) leaves the file, reachable by its file ID or any
// other entry naming it.
//
// # Files still accessed
//
// VMS doesn't pull a file out from under a program using it. A file
// accessed on some channel when it's deleted is only *marked for
// deletion*: its directory entry goes at once, but its header and blocks
// stay until the last channel deaccesses it, and meanwhile nothing new can
// access it. A file created with IO$M_DELETE (a temporary file) is marked
// the same way from the start.
//
// govax keeps this bookkeeping per mounted volume: how many times each
// file ID is accessed (mountedVolume.accessed), and which accessed files
// are marked (mountedVolume.doomed). ACPFile.Deaccess does the deletion
// when the count reaches zero.
//
// # What can't be deleted
//
// The volume's reserved files (INDEXF.SYS, BITMAP.SYS, the master file
// directory, and the rest of the files numbered up to the home block's
// reserved count) are refused (SS$_NOPRIV: on VMS, their protection
// denies it), and so is a directory that still has entries
// (SS$_DIRNOTEMPTY), checked before anything is removed.

// The delete errors.
var (
	ErrACPDirNotEmpty = errors.New("rms: directory is not empty")           // SS$_DIRNOTEMPTY
	ErrACPProtected   = errors.New("rms: a reserved file can't be deleted") // SS$_NOPRIV
)

// acpEntry is a directory entry: the directory's file ID, and the name
// and version in it.
type acpEntry struct {
	dir     FileID
	name    string
	version uint16
}

// ACPDeleteRequest is what IO$_DELETE asks for.
type ACPDeleteRequest struct {
	// Directory and Name are the entry to remove: Name ("NAME.TYP" for
	// the highest version, or "NAME.TYP;VER") in the directory whose file
	// ID is Directory. Directory zero means no entry: the file is named
	// by FID.
	Directory FileID
	Name      string
	FID       FileID

	// DeleteFile is IO$M_DELETE: delete the file too, not just the
	// entry.
	DeleteFile bool
}

// ACPDeleted is ACPDelete's result.
type ACPDeleted struct {
	// FID is the file's ID; Name is the entry removed, "NAME.TYP;VER" (""
	// for none).
	FID  FileID
	Name string

	// Deferred reports that the file is accessed, so it was only marked
	// for deletion: it goes at its last deaccess.
	Deferred bool
}

// ACPDelete removes a directory entry and, if asked, deletes the file (see
// this file's opening comment), on the volume mounted on device, which
// must be mounted writable.
func (t *MountTable) ACPDelete(device string, req ACPDeleteRequest) (ACPDeleted, error) {
	m, ok := t.mounts[normalizeDeviceName(device)]
	if !ok {
		return ACPDeleted{}, ErrACPNotMounted
	}

	if !m.Writable {
		return ACPDeleted{}, ErrACPWriteLocked
	}

	vol := m.Volume

	var (
		result ACPDeleted
		dir    *volume.Directory
		entry  *acpEntry
	)

	if req.Directory != (FileID{}) {
		base, version, err := splitACPName(req.Name)
		if err != nil {
			return result, err
		}

		if dir, err = vol.OpenDirectory(req.Directory.toOds2()); err != nil {
			return result, ErrACPBadDirectory
		}

		found, err := dir.Lookup(base, version)

		switch {
		case errors.Is(err, volume.ErrNotFound):
			return result, ErrACPNoSuchFile
		case err != nil:
			return result, err
		}

		result.FID = fileIDFrom(found.Fid)
		result.Name = fmt.Sprintf("%s;%d", base, found.Version)
		entry = &acpEntry{dir: req.Directory, name: base, version: found.Version}
	} else {
		result.FID = req.FID
	}

	if err := m.checkDeletable(result.FID, req.DeleteFile); err != nil {
		return result, err
	}

	bm, ib, err := deviceBitmaps(vol.Devices[0])
	if err != nil {
		return result, err
	}

	if entry != nil {
		if err := dir.Remove(entry.name, entry.version, bm, ib); err != nil {
			return result, err
		}
	}

	if req.DeleteFile {
		if m.accessed[result.FID] > 0 {
			m.markDoomed(result.FID, nil) // the entry, if any, is gone
			result.Deferred = true
		} else if err := deleteHeader(vol, result.FID, bm, ib); err != nil {
			return result, err
		}
	}

	return result, flushBitmaps(bm, ib)
}

// checkDeletable reports why the file with ID fid can't have its entry
// removed or, if deleteFile, be deleted: a reserved file, a file that
// isn't there, or a directory with entries. It changes nothing.
func (m *mountedVolume) checkDeletable(fid FileID, deleteFile bool) error {
	dev := m.Volume.Devices[0]

	if uint32(fid.Nmx)<<16|uint32(fid.Num) <= uint32(dev.Home.ReservedFiles) {
		return ErrACPProtected
	}

	f, err := m.Volume.OpenFID(fid.toOds2())
	if err != nil || m.doomed[fid] != nil {
		return ErrACPNoSuchFile
	}

	if deleteFile && f.Header.IsDirectory() {
		d, err := f.Directory()
		if err != nil {
			return err
		}

		entries, err := d.List()
		if err != nil {
			return err
		}

		if len(entries) > 0 {
			return ErrACPDirNotEmpty
		}
	}

	return nil
}

// deleteHeader frees the file with ID fid (ods2's DeleteHeader), mapping
// its refusals to the ACP errors.
func deleteHeader(vol *volume.Volume, fid FileID, bm *volume.Bitmap, ib *volume.IndexBitmap) error {
	err := volume.DeleteHeader(vol.Devices[0], fid.toOds2(), bm, ib)

	switch {
	case errors.Is(err, volume.ErrNotFound):
		return ErrACPNoSuchFile
	case errors.Is(err, volume.ErrReservedFile):
		return ErrACPProtected
	case errors.Is(err, volume.ErrDirectoryNotEmpty):
		return ErrACPDirNotEmpty
	}

	return err
}

// deleteNow deletes the file with ID fid and, if entry isn't nil, the
// directory entry naming it (a temporary file created with one), writing
// the bitmaps back.
func (m *mountedVolume) deleteNow(fid FileID, entry *acpEntry) error {
	bm, ib, err := deviceBitmaps(m.Volume.Devices[0])
	if err != nil {
		return err
	}

	if entry != nil {
		dir, err := m.Volume.OpenDirectory(entry.dir.toOds2())
		if err == nil {
			err = dir.Remove(entry.name, entry.version, bm, ib)
		}

		if err != nil {
			return err
		}
	}

	if err := deleteHeader(m.Volume, fid, bm, ib); err != nil {
		return err
	}

	return flushBitmaps(bm, ib)
}

// markDoomed marks the accessed file with ID fid for deletion at its last
// deaccess, along with entry, if not nil.
func (m *mountedVolume) markDoomed(fid FileID, entry *acpEntry) {
	if m.doomed == nil {
		m.doomed = map[FileID]*acpEntry{}
	}

	if entry == nil {
		entry = &acpEntry{} // no entry to remove, but marked
	}

	m.doomed[fid] = entry
}

// access records one more access of the file with ID fid.
func (m *mountedVolume) access(fid FileID) {
	if m.accessed == nil {
		m.accessed = map[FileID]int{}
	}

	m.accessed[fid]++
}

// release records the end of one access of the file with ID fid, and
// deletes the file if that was the last and it's marked for deletion.
func (m *mountedVolume) release(fid FileID) error {
	if m.accessed[fid]--; m.accessed[fid] > 0 {
		return nil
	}

	delete(m.accessed, fid)

	entry, doomed := m.doomed[fid]
	if !doomed {
		return nil
	}

	delete(m.doomed, fid)

	if entry.name == "" {
		entry = nil
	}

	return m.deleteNow(fid, entry)
}
