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
// its version; without such an entry, the name its header records.
//
// err is ErrACPNotMounted or ErrACPNoSuchFile (a file ID that names no
// file: a header not in use, or a sequence number that isn't the
// header's). When the back links don't reach the master file directory,
// spec is the file's name alone ("NAME.TYP;3", no directory) and broken
// is ErrBrokenBacklink.
func (t *MountTable) ACPFileSpec(device string, fid FileID) (spec string, broken, err error) {
	vol, ok := t.Lookup(device)
	if !ok {
		return "", nil, ErrACPNotMounted
	}

	f, err := vol.OpenFID(fid.toOds2())
	if err != nil {
		return "", nil, ErrACPNoSuchFile
	}

	mfd := ondisk.MasterFileDirectoryFid

	// The master file directory is entered in itself, as 000000.DIR;1.
	if sameFileID(f.Header.Fid, mfd) {
		return "[000000]" + entryName(vol, f.Header), nil, nil
	}

	name := entryName(vol, f.Header)

	var dirs []string

	h := f.Header
	for range maxBacklinks {
		parent := h.Backlink
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
	}

	return name, ErrBrokenBacklink, nil
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
// its header (which, written by ods2 rather than VMS, may have no
// version).
func entryName(vol *volume.Volume, h ondisk.FileHeader) string {
	if !h.Backlink.IsZero() {
		if dir, err := vol.OpenDirectory(h.Backlink); err == nil {
			if entries, err := dir.List(); err == nil {
				for _, e := range entries {
					if sameFileID(e.Fid, h.Fid) {
						return fmt.Sprintf("%s;%d", strings.ToUpper(e.Name), e.Version)
					}
				}
			}
		}
	}

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
