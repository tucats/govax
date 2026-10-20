package rms

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// A BACKUP save set opened for reading (docs/PHASE-69 - backup save
// sets.md, subtask 5). VMS keeps a save set on a disk as a file of
// fixed-length records, each one of the save set's blocks; govax reads
// it as bytes, its blocks found by internal/backup from the first
// block's header, so a save set whose record attributes were lost on the
// way (copied with COPY/BINARY, or by FTP as fixed 512-byte records)
// reads as well. A save set may also be a host file holding the same
// bytes.

// SaveSetFile is a save set's file, open for reading as an io.ReaderAt
// of its bytes.
type SaveSetFile struct {
	// Name is the file's full name, with its version
	// ("DUA1:[B69]TEXT.TXT;2"), or the host path: what BACKUP's
	// NOTSAVESET names.
	Name string

	// Host is true for a host file.
	Host bool

	// Size is the file's length in bytes: a volume file's up to its end
	// of file, a host file's whole.
	Size int64

	// Format and RecordSize are a volume file's record format (FAT$B_RTYPE's
	// low half, an ondisk.RecordFormat) and record size (FAT$W_RSIZE): a
	// save set VMS wrote has fixed-length records of its block size
	// (RecordsMatch). A host file has neither.
	Format     uint8
	RecordSize uint16

	// file is the accessed volume file; host the open host file.
	file *ACPFile
	host *os.File
}

// ReadAt reads len(p) bytes from offset off, as io.ReaderAt says: fewer
// only at the file's end, with io.EOF.
func (f *SaveSetFile) ReadAt(p []byte, off int64) (int, error) {
	if f.host != nil {
		return f.host.ReadAt(p, off)
	}

	if off < 0 {
		return 0, fmt.Errorf("rms: reading %s: negative offset", f.Name)
	}

	n := 0
	block := make([]byte, ondisk.BlockSize)

	// Each virtual block the range touches, read whole and copied from.
	for n < len(p) && off+int64(n) < f.Size {
		at := off + int64(n)
		vbn := uint32(at/ondisk.BlockSize) + 1

		if err := f.file.file.ReadBlock(vbn, block); err != nil {
			return n, err
		}

		start := int(at % ondisk.BlockSize)
		end := min(ondisk.BlockSize, start+len(p)-n, start+int(f.Size-at))
		n += copy(p[n:], block[start:end])
	}

	if n < len(p) {
		return n, io.EOF
	}

	return n, nil
}

// RecordsMatch reports whether the file's records are what VMS's BACKUP
// writes for a save set of blockSize: fixed-length records of that size.
// A host file has no records, and matches.
func (f *SaveSetFile) RecordsMatch(blockSize int) bool {
	return f.Host || (f.Format == uint8(ondisk.RecordFormatFixed) && int(f.RecordSize) == blockSize)
}

// Close lets the file go.
func (f *SaveSetFile) Close() error {
	switch {
	case f.host != nil:
		return f.host.Close()
	case f.file != nil:
		return f.file.Deaccess()
	}

	return nil
}

// OpenSaveSet opens the save set file at loc for reading. A volume file
// is accessed for reading, others allowed to read it but not to write it
// while it's open (govax's choice: VMS's sharing for a save set read
// isn't known). A name with wildcards is a *DCLFileError with RMS$_WLD (a
// save set is one file), and one that finds no file NotFound's, or for a
// host file a *DCLFileError with RMS$_FNF naming the path.
func (s *Session) OpenSaveSet(loc FileLocation) (*SaveSetFile, error) {
	if loc.Host {
		return openHostSaveSet(loc.Name)
	}

	if strings.ContainsAny(loc.Name, "*%") || strings.Contains(loc.Name, "...") {
		name, err := s.ExpandName(loc.Name)
		if err != nil {
			name = strings.ToUpper(loc.Name)
		}

		return nil, newDCLFileError(name, rmsWildcardError)
	}

	var set *SaveSetFile

	err := s.firstSpec(loc.Name, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := filespec.Glob(vol, r.Spec)
		if err != nil || len(matches) == 0 {
			return &NotFoundError{Spec: loc.Name}
		}

		m := matches[0]

		a, err := s.Mounts.ACPAccessWith(r.Spec.Device, fileIDFrom(m.Fid), ACPAccessMode{NoWrite: true})
		if err != nil {
			return err
		}

		ra := a.file.Header.RecordAttributes
		set = &SaveSetFile{
			Name:       fullSpec(r.Display, m),
			Size:       odsrms.FileByteLength(ra),
			Format:     uint8(ra.Format) & 0x0F,
			RecordSize: ra.RecordSize,
			file:       a,
		}

		return nil
	})

	var notFound *NotFoundError

	switch {
	case set != nil:
		return set, nil
	case err == nil, errors.As(err, &notFound), errors.Is(err, ErrACPNoSuchFile):
		return nil, s.NotFound(loc.Name)
	case errors.Is(err, ErrACPAccessConflict):
		return nil, newDCLFileError(fullName(s, loc.Name), rmsFileLocked)
	}

	return nil, fmt.Errorf("rms: opening %s: %w", loc.Name, err)
}

// fullName is spec as ExpandName expands it, or as given, upper case, if
// it can't be.
func fullName(s *Session, spec string) string {
	name, err := s.ExpandName(spec)
	if err != nil {
		return strings.ToUpper(spec)
	}

	return name
}

// openHostSaveSet opens the host file path as a save set.
func openHostSaveSet(path string) (*SaveSetFile, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, newDCLFileError(path, rmsFileNotFound)
	}

	if err != nil {
		return nil, fmt.Errorf("rms: opening %s: %w", path, err)
	}

	info, err := f.Stat()
	if err == nil && info.IsDir() {
		err = fmt.Errorf("%s: is a directory, not a file", path)
	}

	if err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("rms: opening %s: %w", path, err)
	}

	return &SaveSetFile{Name: path, Host: true, Size: info.Size(), host: f}, nil
}
