package rtl

import (
	"github.com/tucats/govax/internal/rms"
)

// Deleting files with the disk's $QIO (docs/PHASE-26.md subtask 44).
//
// # IO$_DELETE
//
// Its arguments are IO$_ACCESS's: p1 the FIB's descriptor, p2 a name's,
// p3/p4 where to return the entry removed ("NAME.TYP;VER"). What it does
// depends on the FIB and on IO$M_DELETE:
//
//	FIB$W_DID set, p2 a name ("NAME.TYP" for the highest version)
//	    the directory entry is removed, and the file's ID stored in the
//	    FIB; with IO$M_DELETE, the file is deleted too
//	FIB$W_DID zero, IO$M_DELETE
//	    the file FIB$W_FID is deleted (any directory entry naming it is
//	    the program's to remove)
//
// A file accessed on a channel (this one or another) when it's deleted is
// only marked for deletion: its entry goes at once, but the file itself
// stays, readable and writable on those channels, until the last of them
// deaccesses it; meanwhile it can't be accessed again (SS$_NOSUCHFILE).
// The volume's reserved files are SS$_NOPRIV, a directory with entries
// SS$_DIRNOTEMPTY. The work is internal/rms's (acpdelete.go).

// diskDelete is IO$_DELETE (see above).
func diskDelete(env *Environment, req *ioRequest) (ioStatus, uint32) {
	if env.Mounts == nil {
		return ioStatus{status: ssDevNotMnt}, 0
	}

	f, ok := env.readFIB(req.p[0])
	if !ok {
		return ioStatus{}, ssAccVio
	}

	name, ok := env.diskFileName(req)
	if !ok {
		return ioStatus{}, ssAccVio
	}

	del := rms.ACPDeleteRequest{FID: f.fid, DeleteFile: req.modified(ioModDelete)}

	switch {
	case f.did != (rms.FileID{}) && name != "":
		del.Directory, del.Name = f.did, name
	case !del.DeleteFile || f.fid == (rms.FileID{}):
		// No entry to remove, and no file to delete.
		return ioStatus{status: ssBadParam}, 0
	}

	deleted, err := env.Mounts.ACPDelete(diskDevice(req), del)
	if err != nil {
		return ioStatus{status: acpStatus(err)}, 0
	}

	env.storeFID(f, fibFID, deleted.FID)

	if !env.storeResultName(req, deleted.Name) {
		return ioStatus{}, ssAccVio
	}

	return ioStatus{status: ssNormal}, 0
}
