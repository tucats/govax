package rms

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// The page file's storage (docs/PHASE-64 - page file.md, subtask 2).
//
// With paging on, govax's page file holds pages that have no other home
// while their frames are reused (internal/corevms's pagefile.go). It's a
// file of the configuration's choosing (vax.sysgen.paging.file): on a
// mounted volume, as VMS's SYS$SYSTEM:PAGEFILE.SYS is, or on the host.
// Either way it's only a row of 512-byte blocks, block n holding one
// page: govax keeps its allocation map in Go, and nothing in it outlives
// the session, so an existing file is simply used again, made the size
// asked for first (vax.sysgen.paging.size, as SYSGEN CREATE resizes
// VMS's): a volume file is extended in place, or, when it's to be
// smaller, deleted and created again (ods2 has no way to give a file's
// last blocks back); a host file is truncated to the size either way.
//
// Two rules make it safe to read and write the file from inside a page
// fault, which may come in the middle of an RMS operation that is itself
// using ods2 (the phase doc's "The ods2 audit"):
//
//   - its block I/O goes straight to the file's blocks (ACPFile's
//     ReadPage and WritePage, or a host file's ReadAt and WriteAt), never
//     through RMS's records, streams, or IFI table;
//   - it never writes past its allocation, since extending a file
//     changes the volume's storage bitmap and headers, which an
//     interrupted RMS operation may be in the middle of changing.
//
// Creating the file does change them, so it must happen where no RMS
// operation is in progress: corevms opens the page file at an instruction
// boundary.

// PageFileStore is the page file's blocks: a volume file accessed for
// writing with no one else allowed to read or write it, or a host file.
type PageFileStore struct {
	// Name is the file's full name: a volume file's specification with
	// its version, or a host file's path.
	Name string

	// Host is true for a host file.
	Host bool

	// Created reports that the file didn't exist (or was made again,
	// smaller) at the size asked for.
	Created bool

	// Resized is the file's old size in blocks when an existing file was
	// made the size asked for, else 0. GrowFailed is the reason an
	// existing file couldn't grow to it, the volume having no room: it's
	// used at its old size.
	Resized    uint32
	GrowFailed error

	// Contiguous reports that a volume file's blocks are one run on the
	// disk (a host file's are the host's business).
	Contiguous bool

	blocks uint32
	acp    *ACPFile
	host   *os.File

	// volume is a volume file's mount, where it's counted among the
	// volume's page files while open (FilesOpenError).
	volume *mountedVolume
}

// Blocks is the file's size in blocks: how many pages it can hold.
func (p *PageFileStore) Blocks() uint32 { return p.blocks }

// ReadBlock reads block vbn (1 is the first) into buf, which must be at
// least a block long.
func (p *PageFileStore) ReadBlock(vbn uint32, buf []byte) error {
	if vbn == 0 || vbn > p.blocks {
		return fmt.Errorf("rms: page file block %d: %w", vbn, ErrACPEndOfFile)
	}

	if p.host != nil {
		_, err := p.host.ReadAt(buf[:ondisk.BlockSize], int64(vbn-1)*ondisk.BlockSize)

		return err
	}

	return p.acp.ReadPage(vbn, buf)
}

// WriteBlock writes data, one block, to block vbn. It never writes past
// the file's size, so the file never grows.
func (p *PageFileStore) WriteBlock(vbn uint32, data []byte) error {
	if vbn == 0 || vbn > p.blocks {
		return fmt.Errorf("rms: page file block %d: %w", vbn, ErrACPEndOfFile)
	}

	if len(data) != ondisk.BlockSize {
		return fmt.Errorf("rms: page file block %d: %d bytes, not a block", vbn, len(data))
	}

	if p.host != nil {
		_, err := p.host.WriteAt(data, int64(vbn-1)*ondisk.BlockSize)

		return err
	}

	return p.acp.WritePage(vbn, data)
}

// Close ends the file's use: a volume file is deaccessed, so its volume
// can be dismounted again, and a host file closed. Closing again does
// nothing.
func (p *PageFileStore) Close() error {
	switch {
	case p.host != nil:
		err := p.host.Close()
		p.host = nil

		return err
	case p.acp != nil:
		err := p.acp.Deaccess()
		p.acp = nil

		if p.volume != nil {
			p.volume.pageFiles--
			p.volume = nil
		}

		return err
	}

	return nil
}

// OpenPageFile opens the page file spec names, creating it with size
// blocks if it doesn't exist. A spec in quotes, or one that Locate's
// rules say is a host path, is a host file; any other is a volume file,
// which must name one file with no wildcards. An existing file is made
// size blocks too, when it can be (this file's opening comment).
func (s *Session) OpenPageFile(spec string, size uint32) (*PageFileStore, error) {
	text := strings.TrimSpace(spec)
	quoted := len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"'

	if quoted {
		text = text[1 : len(text)-1]
	}

	if text == "" {
		return nil, errors.New("rms: no page file named")
	}

	if size == 0 {
		return nil, errors.New("rms: a page file needs a size")
	}

	loc, err := s.Locate(text, quoted)
	if err != nil {
		return nil, err
	}

	if loc.Host {
		return openHostPageFile(loc.Name, size)
	}

	return s.openVolumePageFile(loc.Name, size)
}

// openHostPageFile opens or creates the host page file at path.
func openHostPageFile(path string, size uint32) (*PageFileStore, error) {
	_, statErr := os.Stat(path)
	created := errors.Is(statErr, fs.ErrNotExist)

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("rms: opening page file %s: %w", path, err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("rms: opening page file %s: %w", path, err)
	}

	store := &PageFileStore{Name: path, Host: true, Created: created, blocks: size, host: f}

	old := uint32(min(info.Size()/ondisk.BlockSize, int64(^uint32(0))))
	if old != size {
		// Made the size asked for. Truncate makes a sparse file on hosts
		// that have them, so the blocks cost nothing until they're
		// written.
		if err := f.Truncate(int64(size) * ondisk.BlockSize); err != nil {
			_ = f.Close()

			return nil, fmt.Errorf("rms: sizing page file %s: %w", path, err)
		}

		if !created && old != 0 {
			store.Resized = old
		}
	}

	return store, nil
}

// openVolumePageFile opens or creates the volume page file text names,
// and accesses it so that no one else may read or write it while govax
// pages to it.
func (s *Session) openVolumePageFile(text string, size uint32) (*PageFileStore, error) {
	vol, spec, err := s.resolveVolume(text)
	if err != nil {
		return nil, err
	}

	if spec.Name+spec.Type == "" || strings.ContainsAny(spec.Name+spec.Type+spec.Version, "*%") {
		return nil, fmt.Errorf("rms: page file %s: a page file needs one name, with no wildcards", text)
	}

	version, ok := parseOpenVersion(spec.Version)
	if !ok {
		return nil, fmt.Errorf("rms: page file %s: bad version", text)
	}

	dir, err := filespec.ResolveDirectory(vol, spec.Dirs)
	if err != nil {
		return nil, fmt.Errorf("rms: page file %s: %w", text, err)
	}

	name := spec.Name + "." + spec.Type
	store := &PageFileStore{}

	var fid FileID

	entry, err := dir.Lookup(name, version)

	switch {
	case err == nil:
		fid, version = fileIDFrom(entry.Fid), entry.Version
	case errors.Is(err, volume.ErrNotFound):
		if fid, version, err = s.createVolumePageFile(spec.Device, dir, vol, name, version, size, text); err != nil {
			return nil, err
		}

		store.Created = true
	default:
		return nil, fmt.Errorf("rms: page file %s: %w", text, err)
	}

	a, err := s.Mounts.ACPAccessWith(spec.Device, fid, ACPAccessMode{Write: true, NoRead: true, NoWrite: true})
	if err != nil {
		return nil, fmt.Errorf("rms: accessing page file %s: %w", text, err)
	}

	// A volume allocates whole clusters, so a file made for size blocks
	// may hold up to a cluster more; that's the size too.
	cluster := max(uint32(vol.Devices[0].Home.ClusterSize), 1)
	rounded := (size + cluster - 1) / cluster * cluster

	switch old := a.AllocatedBlocks(); {
	case old >= size && old <= rounded:

	case old == 0:
		// A file with no blocks at all is given them.
		if _, err := a.Extend(size); err != nil {
			_ = a.Deaccess()

			return nil, fmt.Errorf("rms: page file %s: %w", text, err)
		}

		store.Created = true

	case old < size:
		// Grown in place, as SYSGEN CREATE extends VMS's; without room,
		// used at its old size.
		if _, err := a.Extend(size - old); err != nil {
			store.GrowFailed = err
		} else {
			store.Resized = old
		}

	default:
		// Smaller: deleted, and made again at the size, with its version.
		// Nothing in it is kept, and its blocks are free for the new one.
		if err := a.Deaccess(); err != nil {
			return nil, fmt.Errorf("rms: page file %s: %w", text, err)
		}

		del := ACPDeleteRequest{Directory: fileIDFrom(dir.Header.Fid), Name: fmt.Sprintf("%s;%d", name, version), DeleteFile: true}
		if _, err := s.Mounts.ACPDelete(spec.Device, del); err != nil {
			return nil, fmt.Errorf("rms: resizing page file %s: %w", text, err)
		}

		if fid, version, err = s.createVolumePageFile(spec.Device, dir, vol, name, version, size, text); err != nil {
			return nil, err
		}

		if a, err = s.Mounts.ACPAccessWith(spec.Device, fid, ACPAccessMode{Write: true, NoRead: true, NoWrite: true}); err != nil {
			return nil, fmt.Errorf("rms: accessing page file %s: %w", text, err)
		}

		store.Created, store.Resized = true, old
	}

	store.Contiguous = contiguous(a.file.Extents)

	if store.Created || store.Resized != 0 {
		if err := markPageFile(a, store.Contiguous); err != nil {
			_ = a.Deaccess()

			return nil, fmt.Errorf("rms: page file %s: %w", text, err)
		}
	}

	spec.Version = fmt.Sprint(version)
	store.Name, store.acp, store.blocks = spec.String(), a, a.AllocatedBlocks()

	if entry, ok := s.Mounts.mounts[normalizeDeviceName(spec.Device)]; ok {
		store.volume = entry
		entry.pageFiles++
	}

	return store, nil
}

// createVolumePageFile creates the page file name (NAME.TYP) in dir on
// the volume mounted on device, of size blocks, with version if it isn't
// 0, returning its file ID and version. A volume without room for it
// gets no file at all, rather than one left behind empty. text is the
// name as given, for errors.
func (s *Session) createVolumePageFile(device string, dir *volume.Directory, vol *volume.Volume,
	name string, version uint16, size uint32, text string,
) (FileID, uint16, error) {
	if !s.Mounts.Writable(device) {
		return FileID{}, 0, fmt.Errorf("rms: creating page file %s: %w", text, ErrACPWriteLocked)
	}

	if free, err := freeBlocks(vol); err != nil || free < size {
		return FileID{}, 0, fmt.Errorf("rms: creating page file %s: %w", text, ErrACPDeviceFull)
	}

	req := ACPCreateRequest{Directory: fileIDFrom(dir.Header.Fid), Name: name, Blocks: size}
	if version != 0 {
		req.Name = fmt.Sprintf("%s;%d", name, version)
	}

	created, err := s.Mounts.ACPCreate(device, req)
	if err != nil {
		return FileID{}, 0, fmt.Errorf("rms: creating page file %s: %w", text, err)
	}

	_, version, _ = splitACPName(created.Name)

	return created.FID, version, nil
}

// freeBlocks is how many blocks are free on vol's first device.
func freeBlocks(vol *volume.Volume) (uint32, error) {
	dev := vol.Devices[0]

	bm, err := dev.Bitmap()
	if err != nil {
		return 0, err
	}

	return bm.FreeClusters() * uint32(dev.Home.ClusterSize), nil
}

// contiguous reports whether extents are one run of logical blocks.
func contiguous(extents []volume.ExtentLocation) bool {
	for i := 1; i < len(extents); i++ {
		prev := extents[i-1]
		if extents[i].Rvn != prev.Rvn || extents[i].StartLBN != prev.StartLBN+prev.Count {
			return false
		}
	}

	return true
}

// markPageFile gives a new page file its attributes: fixed-length
// 512-byte records, every block in use, and contiguous-best-try (SYSGEN
// CREATE's default), contiguous too if its blocks are one run. (The
// record attributes, owner, and protection VMS gives its page file are
// unconfirmed until the phase's probe.)
func markPageFile(a *ACPFile, isContiguous bool) error {
	blocks := a.AllocatedBlocks()

	return updateAttributes(a.file, func(attr *ACPAttributes) {
		ra, err := ondisk.DecodeRecAttr(attr.RecordAttributes[:])
		if err != nil {
			ra = ondisk.RecAttr{}
		}

		ra.Format = ondisk.RecordFormatFixed
		ra.RecordSize = ondisk.BlockSize
		ra.EndOfFileBlock = blocks + 1
		ra.FirstFreeByte = 0
		copy(attr.RecordAttributes[:], ondisk.EncodeRecAttr(ra))

		attr.Characteristics |= ondisk.FchContigB
		if isContiguous {
			attr.Characteristics |= ondisk.FchContig
		} else {
			attr.Characteristics &^= ondisk.FchContig // grown past its run
		}
	})
}
