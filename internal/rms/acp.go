package rms

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// The disk ACP's file operations (docs/PHASE-26.md subtask 41): what a
// program's $QIO on a disk channel does to the files of a mounted volume.
//
// # The ACP and virtual blocks
//
// On VMS, RMS isn't the only way to a file. Underneath it, every file
// operation is a $QIO to the disk, handled by the disk's *ancillary
// control process* (ACP, or the XQP in later versions): IO$_ACCESS looks
// a file up in a directory and opens ("accesses") it on the channel,
// IO$_READVBLK and IO$_WRITEVBLK move its *virtual blocks* (512-byte
// blocks numbered from 1 within the file, wherever they physically are on
// the disk), IO$_MODIFY extends it, and IO$_DEACCESS closes it. RMS is
// built on exactly these calls, and programs that want raw block I/O
// (copying, backup, database engines) use them directly.
//
// Files are named, at this level, by *file IDs* (FIDs): three words, the
// file's number in the volume's index file, a sequence number bumped each
// time that slot is reused (so a stale FID is detected), and a relative
// volume number. A lookup names a file by its name in a directory, the
// directory itself named by its FID (the master file directory, [000000],
// is always FID (4,4,0)).
//
// # How govax does it
//
// This file is the part that touches the files: it's in internal/rms,
// the one package allowed to import the ods2 module, whose volume package
// does the real work (Directory.Lookup, Volume.OpenFID, File.ReadBlock,
// WriteBlock, Extend). internal/rtl's disk driver decodes the $QIO
// arguments, calls these, and turns their errors into $SSDEF statuses.

// FileID is a Files-11 file ID in the form a program's file information
// block (FIB) holds it: file number, sequence number, and relative volume
// number, with the file number's high byte (NMX) for very large volumes.
type FileID struct {
	Num, Seq uint16
	Rvn, Nmx uint8
}

func (id FileID) toOds2() ondisk.Fid {
	return ondisk.Fid{Num: id.Num, Seq: id.Seq, Rvn: id.Rvn, Nmx: id.Nmx}
}

func fileIDFrom(f ondisk.Fid) FileID {
	return FileID{Num: f.Num, Seq: f.Seq, Rvn: f.Rvn, Nmx: f.Nmx}
}

// The ACP functions' errors. internal/rtl maps each to its $SSDEF status
// (shown here); any other error is a device error.
var (
	ErrACPNotMounted   = errors.New("rms: device not mounted")           // SS$_DEVNOTMOUNT
	ErrACPNoSuchFile   = errors.New("rms: no such file")                 // SS$_NOSUCHFILE
	ErrACPBadDirectory = errors.New("rms: not a directory")              // SS$_BADIRECTORY
	ErrACPBadName      = errors.New("rms: bad file name")                // SS$_BADFILENAME
	ErrACPBadVersion   = errors.New("rms: bad file version")             // SS$_BADFILEVER
	ErrACPWriteLocked  = errors.New("rms: volume mounted read-only")     // SS$_WRITLCK
	ErrACPReadOnly     = errors.New("rms: file not accessed for write")  // SS$_NOPRIV
	ErrACPEndOfFile    = errors.New("rms: end of file")                  // SS$_ENDOFFILE
	ErrACPBadBlock     = errors.New("rms: virtual block number is zero") // SS$_BADPARAM
	ErrACPDeviceFull   = errors.New("rms: no room to extend the file")   // SS$_DEVICEFULL
)

// ACPLookup finds the file called name ("NAME.TYP;VER", or without a
// version for the highest) in the directory whose file ID is did, on the
// volume mounted on device, returning its file ID and its full name with
// version.
func (t *MountTable) ACPLookup(device string, did FileID, name string) (FileID, string, error) {
	vol, ok := t.Lookup(device)
	if !ok {
		return FileID{}, "", ErrACPNotMounted
	}

	base, version, err := splitACPName(name)
	if err != nil {
		return FileID{}, "", err
	}

	dir, err := vol.OpenDirectory(did.toOds2())
	if err != nil {
		// The ID is stale or unknown, or names a file that isn't a
		// directory: either way, not a directory to look in.
		return FileID{}, "", ErrACPBadDirectory
	}

	entry, err := dir.Lookup(base, version)

	switch {
	case errors.Is(err, volume.ErrNotFound):
		return FileID{}, "", ErrACPNoSuchFile
	case err != nil:
		return FileID{}, "", fmt.Errorf("rms: reading directory %v: %w", did, err)
	}

	return fileIDFrom(entry.Fid), fmt.Sprintf("%s;%d", strings.ToUpper(entry.Name), entry.Version), nil
}

// splitACPName splits "NAME.TYP;VER" (or ".VER", VMS's other version
// separator) into the name and type, as directories store them, and the
// version, 0 for none (the highest).
func splitACPName(name string) (string, uint16, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	base, ver := name, ""

	if i := strings.IndexByte(name, ';'); i >= 0 {
		base, ver = name[:i], name[i+1:]
	} else if dot := strings.IndexByte(name, '.'); dot >= 0 {
		if second := strings.IndexByte(name[dot+1:], '.'); second >= 0 {
			base, ver = name[:dot+1+second], name[dot+2+second:]
		}
	}

	if base == "" || strings.ContainsAny(base, "[]<>:*%") {
		return "", 0, ErrACPBadName
	}

	if !strings.Contains(base, ".") {
		base += "."
	}

	if ver == "" {
		return base, 0, nil
	}

	v, err := strconv.ParseUint(ver, 10, 16)
	if err != nil || v > 32767 {
		return "", 0, ErrACPBadVersion
	}

	return base, uint16(v), nil
}

// ACPFile is a file accessed on a channel by IO$_ACCESS.
type ACPFile struct {
	file     *volume.File
	fid      FileID
	writable bool
}

// ACPAccess opens the file with ID fid on the volume mounted on device,
// for reading, or for writing too if write. A read-only mount refuses
// write access.
func (t *MountTable) ACPAccess(device string, fid FileID, write bool) (*ACPFile, error) {
	vol, ok := t.Lookup(device)
	if !ok {
		return nil, ErrACPNotMounted
	}

	if write && !t.Writable(device) {
		return nil, ErrACPWriteLocked
	}

	f, err := vol.OpenFID(fid.toOds2())
	if err != nil {
		return nil, ErrACPNoSuchFile
	}

	if write {
		bm, err := f.Device.Bitmap()
		if err != nil {
			return nil, err
		}

		ib, err := f.Device.IndexBitmap()
		if err != nil {
			return nil, err
		}

		if err := f.OpenForWrite(bm, ib); err != nil {
			return nil, err
		}
	}

	return &ACPFile{file: f, fid: fid, writable: write}, nil
}

// FileID is the accessed file's ID.
func (a *ACPFile) FileID() FileID { return a.fid }

// Writable reports whether the file was accessed for writing.
func (a *ACPFile) Writable() bool { return a.writable }

// AllocatedBlocks is how many virtual blocks the file has allocated, and
// EndOfFileBlock the last one holding data (0 for an empty file).
func (a *ACPFile) AllocatedBlocks() uint32 { return a.file.Blocks() }

// EndOfFileBlock is the last virtual block holding the file's data.
func (a *ACPFile) EndOfFileBlock() uint32 { return a.file.UsedBlocks() }

// ReadVirtual reads up to n bytes from the file, starting at virtual block
// vbn (1 is the first). Reading stops at the end of the file's data: the
// bytes before it are returned with ErrACPEndOfFile, which is also the
// error for a vbn past the end. The last block is returned whole, as the
// disk holds it.
func (a *ACPFile) ReadVirtual(vbn, n uint32) ([]byte, error) {
	if vbn == 0 {
		return nil, ErrACPBadBlock
	}

	eof := a.EndOfFileBlock()
	blocks := (n + ondisk.BlockSize - 1) / ondisk.BlockSize

	var out []byte

	buf := make([]byte, ondisk.BlockSize)

	for i := range blocks {
		b := vbn + i
		if b > eof {
			return out, ErrACPEndOfFile
		}

		if err := a.file.ReadBlock(b, buf); err != nil {
			return out, err
		}

		out = append(out, buf[:min(ondisk.BlockSize, n-uint32(len(out)))]...)
	}

	return out, nil
}

// WriteVirtual writes data to the file starting at virtual block vbn, as
// whole blocks (a short last block is padded with zeros). It needs write
// access (ErrACPReadOnly). Only the blocks allocated to the file can be
// written: a write that would run past them writes nothing and returns
// ErrACPEndOfFile, as VMS does; IO$_MODIFY (Extend) allocates more.
func (a *ACPFile) WriteVirtual(vbn uint32, data []byte) error {
	if !a.writable {
		return ErrACPReadOnly
	}

	if vbn == 0 {
		return ErrACPBadBlock
	}

	blocks := (uint32(len(data)) + ondisk.BlockSize - 1) / ondisk.BlockSize
	if blocks > 0 && vbn+blocks-1 > a.file.Blocks() {
		return ErrACPEndOfFile
	}

	for i := range blocks {
		block := make([]byte, ondisk.BlockSize)
		copy(block, data[i*ondisk.BlockSize:])

		if err := a.file.WriteBlock(vbn+i, block); err != nil {
			return err
		}
	}

	return nil
}

// Extend allocates blocks more virtual blocks at the end of the file,
// returning the first new block's number. It needs write access.
func (a *ACPFile) Extend(blocks uint32) (uint32, error) {
	if !a.writable {
		return 0, ErrACPReadOnly
	}

	first := a.file.Blocks() + 1

	if blocks == 0 {
		return first, nil
	}

	bm, err := a.file.Device.Bitmap()
	if err != nil {
		return 0, err
	}

	ib, err := a.file.Device.IndexBitmap()
	if err != nil {
		return 0, err
	}

	if err := volume.Extend(a.file, bm, ib, blocks); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrACPDeviceFull, err)
	}

	return first, nil
}

// Deaccess closes the file. For one accessed for writing, ods2 records
// the end of file just past the highest block written (if that's beyond
// the old end) and writes the storage bitmap back.
func (a *ACPFile) Deaccess() error {
	if !a.writable {
		return nil
	}

	if err := a.file.Close(); err != nil {
		return err
	}

	bm, err := a.file.Device.Bitmap()
	if err != nil {
		return err
	}

	return bm.Flush()
}
