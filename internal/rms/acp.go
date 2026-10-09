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

	// ErrACPAccessConflict is an access that conflicts with the file's
	// other accessors' (sharing.go).
	ErrACPAccessConflict = errors.New("rms: file accessed in a conflicting way") // SS$_ACCONFLICT
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

	// access is the file system's access of the file (ods2's
	// volume.Access), shared with the file's other accessors, RMS's
	// among them.
	access *volume.Access

	// usedAtAccess is how many blocks held data when the file was
	// accessed, and maxWritten the highest block WriteVirtual has written
	// since: Deaccess moves the end of file only if a write went past it.
	usedAtAccess, maxWritten uint32

	// mount is the volume's record, where this access is counted (see
	// acpdelete.go), and released reports that Deaccess has ended it.
	mount    *mountedVolume
	released bool

	// claim is the RMS open's place in the file's list of openers, for
	// a file opened by RMS for the user (FAB$V_UFO, ufo.go): the access
	// keeps the open's sharing until it ends. nil for IO$_ACCESS.
	claim *openerClaim
}

// ACPAccessMode is how an IO$_ACCESS uses the file: FIB$L_ACCTL's
// FIB$M_WRITE (Write), FIB$M_NOREAD (no one else may read it), and
// FIB$M_NOWRITE (no one else may write it).
type ACPAccessMode struct {
	Write, NoRead, NoWrite bool
}

// ACPAccess opens the file with ID fid on the volume mounted on device,
// for reading, or for writing too if write, sharing it with anyone.
func (t *MountTable) ACPAccess(device string, fid FileID, write bool) (*ACPFile, error) {
	return t.ACPAccessWith(device, fid, ACPAccessMode{Write: write})
}

// ACPAccessWith opens the file with ID fid on the volume mounted on
// device for an accessor using mode. A read-only mount refuses write
// access; an access that conflicts with the file's other accessors (an
// RMS open's too) is ErrACPAccessConflict.
//
// A file marked for deletion (see acpdelete.go) can't be accessed
// (ErrACPNoSuchFile).
func (t *MountTable) ACPAccessWith(device string, fid FileID, mode ACPAccessMode) (*ACPFile, error) {
	m, ok := t.mounts[normalizeDeviceName(device)]
	if !ok {
		return nil, ErrACPNotMounted
	}

	vol := m.Volume

	if _, doomed := m.doomed[fid]; doomed {
		return nil, ErrACPNoSuchFile
	}

	if mode.Write && !t.Writable(device) {
		return nil, ErrACPWriteLocked
	}

	a, err := vol.Access(fid.toOds2(), volume.AccessMode{Write: mode.Write, NoRead: mode.NoRead, NoWrite: mode.NoWrite})
	if errors.Is(err, volume.ErrAccessConflict) {
		return nil, ErrACPAccessConflict
	}

	if err != nil {
		return nil, ErrACPNoSuchFile
	}

	f := a.File

	return &ACPFile{file: f, fid: fid, writable: mode.Write, access: a, usedAtAccess: f.UsedBlocks(), mount: m}, nil
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
//
// Blocks this access has written count as data even past the recorded end
// of file, which only moves when the file is deaccessed (see Deaccess):
// a program can read back what it just wrote.
func (a *ACPFile) ReadVirtual(vbn, n uint32) ([]byte, error) {
	if vbn == 0 {
		return nil, ErrACPBadBlock
	}

	eof := max(a.EndOfFileBlock(), a.maxWritten)
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

		a.maxWritten = max(a.maxWritten, vbn+i)
	}

	return nil
}

// ReadPage reads virtual block vbn into buf (at least a block long) for
// a section of the file (the pager's page I/O, not a transfer): any block
// the file has allocated can be read, its end of file notwithstanding (a
// block never written reads as zeros). Past the allocation it's
// ErrACPEndOfFile.
func (a *ACPFile) ReadPage(vbn uint32, buf []byte) error {
	if vbn == 0 || vbn > a.file.Blocks() {
		return ErrACPEndOfFile
	}

	return a.file.ReadBlock(vbn, buf)
}

// WritePage writes data, one block, to virtual block vbn: a section's
// modified page going back to its file. Like ReadPage it may write any
// allocated block and leaves the end of file alone (a section's pages
// aren't records). It needs write access (ErrACPReadOnly).
func (a *ACPFile) WritePage(vbn uint32, data []byte) error {
	if !a.writable {
		return ErrACPReadOnly
	}

	if vbn == 0 || vbn > a.file.Blocks() {
		return ErrACPEndOfFile
	}

	return a.file.WriteBlock(vbn, data)
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

// Deaccess closes the file. For one accessed for writing, if a write went
// past the old end of file, ods2 records the end of file just past the
// highest block written; either way the storage bitmap is written back.
func (a *ACPFile) Deaccess() error {
	return a.DeaccessWithAttributes(nil)
}

// DeaccessWithAttributes closes the file, as Deaccess does, and then, if
// change isn't nil, writes the attributes it sets (IO$_DEACCESS with an
// attribute list: how RMS records a file's exact end of file when it
// closes it). Writing attributes needs write access (ErrACPReadOnly); the
// file is closed even then.
//
// The end of file is left alone when no write went past it. (ods2's own
// Close always records the end of file as a whole number of blocks, which
// would round up a file whose last block is partly used, even one only
// read, or overwritten in place.)
//
// If this was the file's last access and it's marked for deletion, it's
// deleted now (see acpdelete.go). Deaccessing again does nothing.
func (a *ACPFile) DeaccessWithAttributes(change func(*ACPAttributes)) error {
	if a.released {
		return nil
	}

	err := a.close(change)
	a.released = true

	// The file system's deaccess: the header written back, and the file
	// deleted if it was deleted while accessed, freeing blocks the
	// bitmaps then need written for.
	if a.access != nil {
		if derr := a.access.Deaccess(); err == nil {
			err = derr
		}

		if bm, ib, berr := deviceBitmaps(a.file.Device); berr != nil {
			if err == nil {
				err = berr
			}
		} else if ferr := flushBitmaps(bm, ib); err == nil {
			err = ferr
		}
	}

	a.claim.release()

	if a.mount != nil {
		if rerr := a.mount.release(a.fid); err == nil {
			err = rerr
		}
	}

	return err
}

// close is DeaccessWithAttributes's work on the file itself.
func (a *ACPFile) close(change func(*ACPAttributes)) error {
	if !a.writable {
		if change != nil {
			return ErrACPReadOnly
		}

		return nil
	}

	// The end of file moves to just past the highest block written, if
	// that's past where it is (another accessor may have moved it
	// further).
	if a.maxWritten > a.usedAtAccess && a.maxWritten > a.file.UsedBlocks() {
		a.file.SetEndOfFile(a.maxWritten+1, 0)
	}

	bm, err := a.file.Device.Bitmap()
	if err != nil {
		return err
	}

	if err := bm.Flush(); err != nil {
		return err
	}

	if change != nil {
		return updateAttributes(a.file, change)
	}

	return nil
}
