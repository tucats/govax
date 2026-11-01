package rms

import (
	"errors"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// FAB$L_FOP's bits for what becomes of a file (Phase 81; testdata/for81's
// FOR81R on VMS 7.1, cases "FOP").
//
//   - FAB$V_TMP, on $CREATE: the file has no directory entry, and a NAM's
//     result name no version. It stays on the volume, reachable by its file ID.
//   - FAB$V_TMD, on $CREATE: the same, and it goes when its last accessor does.
//   - FAB$V_DLT, on $CLOSE: the file goes.
//   - FAB$V_SPL and FAB$V_SCF, on $CLOSE: it's printed or run as a batch job,
//     and with DLT then deleted. govax has no queues, and neither did that VAX
//     have one that ran: it returned RMS$_NORMAL and the file stayed, even with
//     DLT.
var (
	fopTMP = vmsConst("FAB$M_TMP")
	fopTMD = vmsConst("FAB$M_TMD")
	fopDLT = vmsConst("FAB$M_DLT")
	fopSPL = vmsConst("FAB$M_SPL")
	fopSCF = vmsConst("FAB$M_SCF")
)

// makeTemporary takes a file just created by name out of its directory, as
// FAB$V_TMP and FAB$V_TMD ask, and for TMD marks it for delete.
func makeTemporary(dir *volume.Directory, name string, version uint16, f *volume.File, fop uint32) error {
	bm, ib, err := deviceBitmaps(dir.Device)
	if err != nil {
		return err
	}

	if err := dir.Remove(name, version, bm, ib); err != nil {
		return err
	}

	if fop&fopTMD != 0 {
		if err := volume.DeleteHeader(dir.Device, f.Header.Fid, bm, ib); err != nil {
			return err
		}
	}

	return flushBitmaps(bm, ib)
}

// deleteAtClose carries out FAB$V_DLT on $CLOSE: the file h has open goes, once
// its last accessor has. Its directory entry goes now, if it has one (a TMP
// file's doesn't), and one that's gone by another name is no matter.
func (ctx *Context) deleteAtClose(h *FileHandle, fop uint32) error {
	if fop&fopDLT == 0 || fop&(fopSPL|fopSCF) != 0 || h.File.MarkedForDelete() {
		return nil
	}

	if h.Found == nil || !ctx.Mounts.Writable(h.Found.Device) {
		return nil
	}

	vol, ok := ctx.Mounts.Lookup(h.Found.Device)
	if !ok {
		return nil
	}

	dir, err := filespec.ResolveDirectory(vol, h.Found.Dirs)
	if err != nil {
		return err
	}

	bm, ib, err := deviceBitmaps(dir.Device)
	if err != nil {
		return err
	}

	name := h.Found.Name + "." + h.Found.Type

	err = volume.DeleteFile(dir, name, h.Found.Version, bm, ib)
	if err != nil && h.Found.Version == 0 || errors.Is(err, volume.ErrNotFound) {
		err = volume.DeleteHeader(dir.Device, h.File.Header.Fid, bm, ib)
	}

	if err != nil {
		return err
	}

	return flushBitmaps(bm, ib)
}
