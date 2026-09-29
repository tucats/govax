package rms

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// Creating files through the ACP (docs/PHASE-26.md subtask 43).
//
// # What creating a file involves
//
// On a Files-11 volume a file is two separate things, which VMS's ACP can
// make separately:
//
//   - a *file header*, a block in the index file (INDEXF.SYS) that is the
//     file: its attributes and the map of where its data lives. The
//     header's slot number is the file ID's file number.
//   - a *directory entry*: a name and version in some directory, pointing
//     at the file ID. A file can have one entry, several (the same file
//     under two names), or none at all (a temporary work file only its
//     creator knows the ID of).
//
// So IO$_CREATE with IO$M_CREATE makes a header and, if the FIB names a
// directory and p2 a name, an entry for it; IO$_CREATE without IO$M_CREATE
// only makes an entry, for a file that already exists (DCL's SET
// FILE/ENTER). ACPCreate and ACPEnter, below, are those two.
//
// # Versions
//
// A name given without a version gets the next one: one more than the
// highest version the directory has of that name (1 if none). A name with
// an explicit version gets that version, unless the directory already has
// it: then the FIB's name control word decides. With FIB$M_NEWVER the file
// gets the next version instead; with FIB$M_SUPERSEDE the existing file
// is deleted and the new one takes its place (SS$_SUPERSEDE, a success);
// with neither, the create fails (SS$_DUPFILENAME). Versions stop at
// 32767 (SS$_BADFILEVER past it).

// ErrACPDuplicate is a name and version the directory already has
// (SS$_DUPFILENAME).
var ErrACPDuplicate = errors.New("rms: file already exists")

// maxVersion is the highest file version VMS allows.
const maxVersion = 32767

// ACPCreateRequest is what IO$_CREATE!IO$M_CREATE asks for.
type ACPCreateRequest struct {
	// Directory is the file ID of the directory to enter the new file
	// in; zero for none (a file with no directory entry).
	Directory FileID

	// Name is the file's name, "NAME.TYP" or "NAME.TYP;VER". With no
	// directory, it's only recorded in the header.
	Name string

	// NewVersion and Supersede are FIB$M_NEWVER and FIB$M_SUPERSEDE: what
	// to do when Name's explicit version already exists (see above).
	NewVersion, Supersede bool

	// Blocks is how many blocks to allocate at once (FIB$L_EXSZ, with
	// FIB$M_EXTEND); 0 for none.
	Blocks uint32

	// VersionLimit is the new file's version limit (FIB$W_VERLIMIT); 0
	// for the one it inherits (see ods2's CreateFile).
	VersionLimit uint16

	// Owner is the owner UIC to give the file (the creating process's);
	// 0 for the volume's default.
	Owner uint32

	// Attributes, if not nil, sets attributes of the new file (the
	// $QIO's attribute list); it runs after Owner is applied, so it can
	// override it.
	Attributes func(*ACPAttributes)

	// Access asks for the new file to be accessed (IO$M_ACCESS), for
	// writing too if Write.
	Access, Write bool
}

// ACPCreated is ACPCreate's result.
type ACPCreated struct {
	FID FileID

	// Name is the directory entry made, "NAME.TYP;VER"; "" for none.
	Name string

	// Blocks is how many blocks the file has allocated: Blocks rounded
	// up to the volume's cluster size.
	Blocks uint32

	// Superseded reports that an existing file was deleted to make way
	// for this one (SS$_SUPERSEDE).
	Superseded bool

	// File is the accessed file, if Access was asked for.
	File *ACPFile
}

// ACPCreate creates a file on the volume mounted on device (see this
// file's opening comment). The volume must be mounted writable.
func (t *MountTable) ACPCreate(device string, req ACPCreateRequest) (ACPCreated, error) {
	vol, ok := t.Lookup(device)
	if !ok {
		return ACPCreated{}, ErrACPNotMounted
	}

	if !t.Writable(device) {
		return ACPCreated{}, ErrACPWriteLocked
	}

	dev := vol.Devices[0]

	bm, ib, err := deviceBitmaps(dev)
	if err != nil {
		return ACPCreated{}, err
	}

	var (
		result ACPCreated
		f      *volume.File
	)

	if req.Directory == (FileID{}) {
		// A file with no directory entry: just the header.
		base := strings.ToUpper(strings.TrimSpace(req.Name))
		if i := strings.IndexByte(base, ';'); i >= 0 {
			base = base[:i]
		}

		f, err = volume.CreateHeader(dev, ib, volume.NewFileHeader{Name: base})
		if err == nil {
			err = f.OpenForWrite(bm, ib)
		}
	} else {
		f, result, err = createInDirectory(vol, req, bm, ib)
	}

	if err != nil {
		return ACPCreated{}, err
	}

	result.FID = fileIDFrom(f.Header.Fid)

	if err := finishCreate(f, req, bm, ib); err != nil {
		return ACPCreated{}, err
	}

	result.Blocks = f.Blocks()

	if req.Access {
		result.File = &ACPFile{file: f, fid: result.FID, writable: req.Write}
	}

	return result, nil
}

// createInDirectory makes the header and directory entry for a create
// with a directory: the version rules in this file's opening comment,
// then ods2's CreateFileVersion.
func createInDirectory(vol *volume.Volume, req ACPCreateRequest, bm *volume.Bitmap, ib *volume.IndexBitmap) (*volume.File, ACPCreated, error) {
	var result ACPCreated

	base, version, err := splitACPName(req.Name)
	if err != nil {
		return nil, result, err
	}

	dir, err := vol.OpenDirectory(req.Directory.toOds2())
	if err != nil {
		return nil, result, ErrACPBadDirectory
	}

	if version != 0 {
		_, err := dir.Lookup(base, version)

		switch {
		case errors.Is(err, volume.ErrNotFound):
			// Free: the file gets the version asked for.
		case err != nil:
			return nil, result, err
		case req.Supersede:
			if err := volume.DeleteFile(dir, base, version, bm, ib); err != nil {
				return nil, result, fmt.Errorf("rms: superseding %s;%d: %w", base, version, err)
			}

			result.Superseded = true
		case req.NewVersion:
			version = 0
		default:
			return nil, result, ErrACPDuplicate
		}
	}

	if version == 0 {
		next, err := dir.NextVersion(base)
		if err != nil {
			return nil, result, err
		}

		if next > maxVersion {
			return nil, result, ErrACPBadVersion
		}

		version = next
	}

	f, err := vol.CreateFileVersion(dir, base, version, ondisk.RecAttr{}, bm, ib)
	if errors.Is(err, volume.ErrExists) {
		return nil, result, ErrACPDuplicate
	}

	if err != nil {
		return nil, result, err
	}

	result.Name = fmt.Sprintf("%s;%d", base, version)

	return f, result, nil
}

// finishCreate applies the rest of a create request to the new header f:
// its version limit, its first allocation, and its owner and attributes;
// then, unless it's to be accessed for writing, disarms it, and writes the
// bitmaps back.
func finishCreate(f *volume.File, req ACPCreateRequest, bm *volume.Bitmap, ib *volume.IndexBitmap) error {
	if req.VersionLimit != 0 {
		if err := volume.SetVersionLimit(f, req.VersionLimit); err != nil {
			return err
		}
	}

	if req.Blocks > 0 {
		if err := volume.Extend(f, bm, ib, req.Blocks); err != nil {
			return fmt.Errorf("%w: %v", ErrACPDeviceFull, err)
		}
	}

	// Close records the end of file from the blocks written (none), so
	// it comes before the attributes, which may set one.
	if !req.Access || !req.Write {
		if err := f.Close(); err != nil {
			return err
		}
	}

	if req.Owner != 0 || req.Attributes != nil {
		err := updateAttributes(f, func(a *ACPAttributes) {
			if req.Owner != 0 {
				a.Owner = req.Owner
			}

			if req.Attributes != nil {
				req.Attributes(a)
			}
		})
		if err != nil {
			return err
		}
	}

	return flushBitmaps(bm, ib)
}

// ACPEnter enters the existing file with ID fid in the directory whose ID
// is did, as name (IO$_CREATE without IO$M_CREATE), returning the entry
// made, "NAME.TYP;VER". The version rules are ACPCreate's, except that an
// explicit version already there can't be superseded (only newVersion
// applies).
func (t *MountTable) ACPEnter(device string, did FileID, name string, fid FileID, newVersion bool) (string, error) {
	vol, ok := t.Lookup(device)
	if !ok {
		return "", ErrACPNotMounted
	}

	if !t.Writable(device) {
		return "", ErrACPWriteLocked
	}

	base, version, err := splitACPName(name)
	if err != nil {
		return "", err
	}

	dir, err := vol.OpenDirectory(did.toOds2())
	if err != nil {
		return "", ErrACPBadDirectory
	}

	if _, err := vol.OpenFID(fid.toOds2()); err != nil {
		return "", ErrACPNoSuchFile
	}

	if version != 0 {
		if _, err := dir.Lookup(base, version); err == nil {
			if !newVersion {
				return "", ErrACPDuplicate
			}

			version = 0
		}
	}

	if version == 0 {
		if version, err = dir.NextVersion(base); err != nil {
			return "", err
		}

		if version > maxVersion {
			return "", ErrACPBadVersion
		}
	}

	bm, ib, err := deviceBitmaps(dir.Device)
	if err != nil {
		return "", err
	}

	if err := dir.Insert(base, version, fid.toOds2(), bm, ib); err != nil {
		if errors.Is(err, volume.ErrExists) {
			return "", ErrACPDuplicate
		}

		return "", err
	}

	return fmt.Sprintf("%s;%d", base, version), flushBitmaps(bm, ib)
}

// deviceBitmaps returns dev's storage bitmap and index file bitmap caches.
func deviceBitmaps(dev *volume.Device) (*volume.Bitmap, *volume.IndexBitmap, error) {
	bm, err := dev.Bitmap()
	if err != nil {
		return nil, nil, err
	}

	ib, err := dev.IndexBitmap()
	if err != nil {
		return nil, nil, err
	}

	return bm, ib, nil
}

// flushBitmaps writes the two bitmap caches back to the volume, so what a
// create or delete allocated or freed is on the disk at once (as it is on
// VMS) rather than only at DISMOUNT.
func flushBitmaps(bm *volume.Bitmap, ib *volume.IndexBitmap) error {
	if err := bm.Flush(); err != nil {
		return err
	}

	return ib.Flush()
}
