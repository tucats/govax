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
// the session, so an existing file is simply used again at its own size.
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

	// Created reports that the file didn't exist and was made at the
	// size asked for; otherwise it's used at its own size.
	Created bool

	// Contiguous reports that a volume file's blocks are one run on the
	// disk (a host file's are the host's business).
	Contiguous bool

	blocks uint32
	acp    *ACPFile
	host   *os.File
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

		return err
	}

	return nil
}

// OpenPageFile opens the page file spec names, creating it with size
// blocks if it doesn't exist. A spec in quotes, or one that Locate's
// rules say is a host path, is a host file; any other is a volume file,
// which must name one file with no wildcards. An existing file is used
// at its own size (one with no blocks at all is given size).
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

	blocks := uint32(min(info.Size()/ondisk.BlockSize, int64(^uint32(0))))

	if blocks == 0 {
		// New, or empty: made the size asked for. Truncate makes a sparse
		// file on hosts that have them, so the blocks cost nothing until
		// they're written.
		if err := f.Truncate(int64(size) * ondisk.BlockSize); err != nil {
			_ = f.Close()

			return nil, fmt.Errorf("rms: sizing page file %s: %w", path, err)
		}

		blocks = size
	}

	return &PageFileStore{Name: path, Host: true, Created: created, blocks: blocks, host: f}, nil
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
		if !s.Mounts.Writable(spec.Device) {
			return nil, fmt.Errorf("rms: creating page file %s: %w", text, ErrACPWriteLocked)
		}

		// A file the volume hasn't room for isn't made at all, rather
		// than left behind empty.
		if free, err := freeBlocks(vol); err != nil || free < size {
			return nil, fmt.Errorf("rms: creating page file %s: %w", text, ErrACPDeviceFull)
		}

		req := ACPCreateRequest{Directory: fileIDFrom(dir.Header.Fid), Name: name, Blocks: size}
		if version != 0 {
			req.Name = fmt.Sprintf("%s;%d", name, version)
		}

		created, err := s.Mounts.ACPCreate(spec.Device, req)
		if err != nil {
			return nil, fmt.Errorf("rms: creating page file %s: %w", text, err)
		}

		fid, store.Created = created.FID, true
		_, version, _ = splitACPName(created.Name)
	default:
		return nil, fmt.Errorf("rms: page file %s: %w", text, err)
	}

	a, err := s.Mounts.ACPAccessWith(spec.Device, fid, ACPAccessMode{Write: true, NoRead: true, NoWrite: true})
	if err != nil {
		return nil, fmt.Errorf("rms: accessing page file %s: %w", text, err)
	}

	if a.AllocatedBlocks() == 0 {
		if _, err := a.Extend(size); err != nil {
			_ = a.Deaccess()

			return nil, fmt.Errorf("rms: page file %s: %w", text, err)
		}

		store.Created = true
	}

	store.Contiguous = contiguous(a.file.Extents)

	if store.Created {
		if err := markPageFile(a, store.Contiguous); err != nil {
			_ = a.Deaccess()

			return nil, fmt.Errorf("rms: page file %s: %w", text, err)
		}
	}

	spec.Version = fmt.Sprint(version)
	store.Name, store.acp, store.blocks = spec.String(), a, a.AllocatedBlocks()

	return store, nil
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
		}
	})
}
