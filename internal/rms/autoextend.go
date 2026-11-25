package rms

import "github.com/tucats/ods2/volume"

// dclMultiblockCount is the multiblock count of DCL's own RAB, the one OPEN,
// READ, and WRITE use: a block at a time (VMS 7.1's REST.DAT in
// testdata/err113, round 2's FULL2, grew 2 blocks at a time).
const dclMultiblockCount = 1

// sequentialExtendSize is how many blocks RMS auto extends a sequential file
// written by records, by the Guide to OpenVMS File Applications's rule
// (3.2.1.3.1): the file's default extension quantity if it has one (then a
// process or system default, SET RMS_DEFAULT/EXTEND_QUANTITY, which govax
// doesn't have, each raised to the volume's default extension quantity);
// else RMS's minimal size, the smaller of twice the multiblock count and
// 256, which the guide doesn't raise to the volume's, and VMS 7.1 didn't:
// on a volume of 3-block clusters, whose default extension quantity was 5,
// a file DCL's OPEN made and wrote a line to had 3 blocks (testdata/dcl50's
// F1), and on one whose quantity was 1 its REST.DAT grew 2 at a time
// (testdata/err113's FULL2). The volume rounds the size up to a cluster, and
// the extension is all or nothing (volume.File.ExtendQuantity).
func sequentialExtendSize(f *volume.File, multiblock uint32) uint32 {
	if deq := uint32(f.Header.RecordAttributes.DefaultExtend); deq != 0 {
		return deq
	}

	return min(2*multiblock, 256)
}

// closeTruncate gives back the blocks of h's file past its end of file,
// rounded up to a cluster, when h was opened by DCL's OPEN to write: VMS
// 7.1's DCL closed a file it had written a line to at one cluster
// (testdata/dcl50's F1, 1/3 on a volume of 3-block clusters; testdata/lnm91's
// W.DAT and testdata/ppf71's, 1/1), though it grew by 2 blocks at a time
// (sequentialExtendSize).
func (h *FileHandle) closeTruncate() error {
	if !h.truncateAtClose || h.File == nil || h.File.MarkedForDelete() {
		return nil
	}

	bm, err := h.File.Device.Bitmap()
	if err != nil {
		return err
	}

	if err := volume.Truncate(h.File, bm, h.File.UsedBlocks()); err != nil {
		return err
	}

	return bm.Flush()
}
