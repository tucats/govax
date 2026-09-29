package rms

import (
	"errors"

	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/ondisk"
)

// Logical block I/O on a mounted volume (docs/PHASE-26.md subtask 45).
//
// # Virtual, logical, and physical blocks
//
// A VMS disk can be read and written at three levels:
//
//   - *virtual* blocks are a file's, numbered from 1 within the file
//     wherever they lie on the disk (IO$_READVBLK, acp.go);
//   - *logical* blocks are the disk's, numbered from 0 across the whole
//     volume (LBNs), with no files in the picture at all: LBN 1 is the
//     home block, and so on (IO$_READLBLK);
//   - *physical* blocks are the drive's own addressing (cylinder, track,
//     sector on the oldest drives); on the MSCP disks govax emulates, a
//     physical block is a logical block (IO$_READPBLK).
//
// Logical I/O bypasses the file system entirely, so VMS requires a
// privilege for it (LOG_IO; PHY_IO for physical), checked by internal/rtl.
// Here it's just block reads and writes on the mounted disk image.
//
// A logical write goes straight to the image, under ods2's feet: ods2
// keeps the volume's storage and index file bitmaps, and the headers of
// open files, in memory, and writes them back later, over whatever a
// logical write put in those blocks. Writing a mounted volume's structure
// this way is as unwise here as on VMS.

// ErrACPIllegalBlock is a transfer that runs past the end of the volume
// (SS$_ILLBLKNUM).
var ErrACPIllegalBlock = errors.New("rms: logical block number past the end of the volume")

// logicalContainer returns the disk image mounted on device, checking
// that the n bytes from logical block lbn fit on it.
func (t *MountTable) logicalContainer(device string, lbn, n uint32) (diskimage.Container, error) {
	m, ok := t.mounts[normalizeDeviceName(device)]
	if !ok {
		return nil, ErrACPNotMounted
	}

	c := m.Volume.Devices[0].Container
	blocks := (uint64(n) + ondisk.BlockSize - 1) / ondisk.BlockSize

	if uint64(lbn)+blocks > uint64(c.Blocks()) || blocks == 0 && lbn >= c.Blocks() {
		return nil, ErrACPIllegalBlock
	}

	return c, nil
}

// ReadLogical reads n bytes from the volume mounted on device, starting at
// logical block lbn (0 is the first). The whole transfer must lie within
// the volume (ErrACPIllegalBlock, nothing read, otherwise).
func (t *MountTable) ReadLogical(device string, lbn, n uint32) ([]byte, error) {
	c, err := t.logicalContainer(device, lbn, n)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, n)
	buf := make([]byte, ondisk.BlockSize)

	for b := lbn; uint32(len(out)) < n; b++ {
		if err := c.ReadBlock(b, buf); err != nil {
			return out, err
		}

		out = append(out, buf[:min(ondisk.BlockSize, n-uint32(len(out)))]...)
	}

	return out, nil
}

// WriteLogical writes data to the volume mounted on device, from logical
// block lbn, as whole blocks (a short last block padded with zeros). The
// volume must be mounted writable (ErrACPWriteLocked), and the whole
// transfer must lie within it (ErrACPIllegalBlock, nothing written,
// otherwise).
func (t *MountTable) WriteLogical(device string, lbn uint32, data []byte) error {
	c, err := t.logicalContainer(device, lbn, uint32(len(data)))
	if err != nil {
		return err
	}

	w, ok := c.(diskimage.WritableContainer)
	if !ok || !t.Writable(device) {
		return ErrACPWriteLocked
	}

	for i := 0; i < len(data); i += ondisk.BlockSize {
		block := make([]byte, ondisk.BlockSize)
		copy(block, data[i:])

		if err := w.WriteBlock(lbn+uint32(i/ondisk.BlockSize), block); err != nil {
			return err
		}
	}

	return nil
}
