package rms

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// A file's name from its file ID (docs/PHASE-58, subtask 7), for
// LIB$FID_TO_NAME.
//
// A file header records the file ID of the directory the file was
// entered in, its *back link* (ondisk.FileHeader.Backlink). Following
// back links from a file, directory by directory, reaches the master
// file directory, [000000], whose own back link is itself; the names met
// on the way, read in reverse, are the file's directory path. VMS's ACP
// answers this as the file specification attribute (ATR$C_FILE_SPEC);
// govax's ACP doesn't have that attribute, so LIB$FID_TO_NAME asks here.

// ErrBrokenBacklink is a file whose back links don't lead to the master
// file directory: one has no back link, or names a file that is gone or
// isn't a directory, or the chain runs on past any real depth.
var ErrBrokenBacklink = errors.New("rms: file's back links don't reach the master file directory")

// maxBacklinks bounds the walk, so a volume whose back links form a
// cycle can't hang it. VAX RMS reaches eight levels (and seven more in a
// root); this is well past that.
const maxBacklinks = 64

// ACPFileSpec returns the name of the file with ID fid on the volume
// mounted on device, found through its back links: "[DIR.SUB]NAME.TYP;3"
// ("[000000]NAME.TYP;3" for a file in the master file directory). A
// directory's name, and the file's, are those of its entry in the
// directory its back link names, so a renamed file has its new name and
// its version. As VMS 7.1 names them (testdata/lib58):
//
//   - a file its back link's directory has no entry for is "[]" and the
//     name its header records ("[]ORPHAN.TXT;1"), with no error;
//   - did, when it isn't nil, is the directory to look for the file in,
//     in place of the back link (even a file ID of 0); a file it has no
//     entry for is ErrACPNoSuchFile;
//   - a file ID of file number 0, past the index file, or whose header
//     holds another file (a stale sequence number) is ErrACPNoSuchFile;
//     a header slot that was never used (not an ODS-2 header) is
//     ErrACPBadHeader (SS$_FILESTRUCT).
//
// err is ErrACPNotMounted or one of the above. When the back links
// don't reach the master file directory (a back link of 0, or to a file
// that's gone or isn't a directory), spec is the file's name alone
// ("NAME.TYP;3", no directory) and broken is ErrBrokenBacklink.
func (t *MountTable) ACPFileSpec(device string, fid FileID, did *FileID) (spec string, broken, err error) {
	vol, ok := t.Lookup(device)
	if !ok {
		return "", nil, ErrACPNotMounted
	}

	if fid.Num == 0 && fid.Nmx == 0 {
		return "", nil, ErrACPNoSuchFile
	}

	f, err := vol.OpenFID(fid.toOds2())
	if err != nil {
		return "", nil, headerError(vol, fid)
	}

	mfd := ondisk.MasterFileDirectoryFid

	// The master file directory is entered in itself, as 000000.DIR;1.
	if sameFileID(f.Header.Fid, mfd) && did == nil {
		return "[000000]" + entryName(vol, f.Header), nil, nil
	}

	h := f.Header
	parent := h.Backlink

	if did != nil {
		parent = did.toOds2()
	}

	name, entered := entryIn(vol, parent, h)

	switch {
	case !entered && did != nil:
		return "", nil, ErrACPNoSuchFile
	case !entered && !parent.IsZero():
		if _, err := vol.OpenDirectory(parent); err == nil {
			return "[]" + name, nil, nil
		}
	}

	var dirs []string

	for range maxBacklinks {
		if parent.IsZero() {
			return name, ErrBrokenBacklink, nil
		}

		if sameFileID(parent, mfd) {
			return "[" + topDirectory(dirs) + "]" + name, nil, nil
		}

		d, err := vol.OpenFID(parent)
		if err != nil || !d.Header.IsDirectory() || sameFileID(parent, h.Fid) {
			return name, ErrBrokenBacklink, nil
		}

		// A directory's entry is "NAME.DIR;1": the path has NAME.
		dir, _, _ := strings.Cut(entryName(vol, d.Header), ".")
		dirs = append(dirs, dir)
		h = d.Header
		parent = h.Backlink
	}

	return name, ErrBrokenBacklink, nil
}

// headerError is why the header of the file fid can't be opened: past
// the index file's allocation, or holding another file, ErrACPNoSuchFile; a
// slot never used (not an ODS-2 header: its structure level isn't 2),
// ErrACPBadHeader.
func headerError(vol *volume.Volume, fid FileID) error {
	rvn := fid.Rvn
	if rvn == 0 {
		rvn = 1
	}

	for _, dev := range vol.Devices {
		if dev.Rvn != rvn || dev.IndexFile == nil {
			continue
		}

		n := uint32(fid.Num) | uint32(fid.Nmx)<<16
		vbn := n - 1 + uint32(dev.Home.IndexBitmapVBN) + uint32(dev.Home.IndexBitmapSize)

		if vbn > dev.IndexFile.Blocks() {
			return ErrACPNoSuchFile
		}

		buf := make([]byte, ondisk.BlockSize)
		if err := dev.IndexFile.ReadBlock(vbn, buf); err != nil {
			return ErrACPNoSuchFile
		}

		// FH2$W_STRUCLEV: ODS-2's has 2 in its high byte.
		if buf[7] != 2 {
			return ErrACPBadHeader
		}
	}

	return ErrACPNoSuchFile
}

// entryIn is the name of the file whose header is h in the directory
// parent, "NAME.TYP;VER", and whether parent has an entry for it; when
// it hasn't, the name in h.
func entryIn(vol *volume.Volume, parent ondisk.Fid, h ondisk.FileHeader) (string, bool) {
	if !parent.IsZero() {
		if dir, err := vol.OpenDirectory(parent); err == nil {
			if entries, err := dir.List(); err == nil {
				for _, e := range entries {
					if sameFileID(e.Fid, h.Fid) {
						return fmt.Sprintf("%s;%d", strings.ToUpper(e.Name), e.Version), true
					}
				}
			}
		}
	}

	return headerName(h), false
}

// topDirectory is the path dirs make, read from the top: dirs runs from
// the file's own directory up, so it is reversed. No directories is the
// master file directory's.
func topDirectory(dirs []string) string {
	if len(dirs) == 0 {
		return "000000"
	}

	var b strings.Builder

	for i := len(dirs) - 1; i >= 0; i-- {
		b.WriteString(dirs[i])

		if i > 0 {
			b.WriteByte('.')
		}
	}

	return b.String()
}

// entryName is the name of the file whose header is h, "NAME.TYP;VER":
// its entry's in the directory its back link names, or else the name in
// its header.
func entryName(vol *volume.Volume, h ondisk.FileHeader) string {
	name, _ := entryIn(vol, h.Backlink, h)

	return name
}

// headerName is the name a file header records (which, written by ods2
// rather than VMS, may have no version).
func headerName(h ondisk.FileHeader) string {
	if id, err := h.Ident(); err == nil {
		return strings.ToUpper(strings.TrimRight(id.Filename+id.FilenameExtension, " "))
	}

	return ""
}

// sameFileID reports whether a and b name the same file: the same file
// number and sequence number (a relative volume number of 0 means this
// volume, so it isn't compared).
func sameFileID(a, b ondisk.Fid) bool {
	return a.Number() == b.Number() && a.Seq == b.Seq
}
