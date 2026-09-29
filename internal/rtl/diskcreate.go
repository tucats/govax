package rtl

import (
	"github.com/tucats/govax/internal/rms"
)

// Creating files with the disk's $QIO (docs/PHASE-26.md subtask 43).
//
// # IO$_CREATE
//
// A file on a Files-11 disk is a *header* (in the index file: the file
// ID's file number is its slot there) plus, usually, a *directory entry*
// naming it (see internal/rms/acpcreate.go). IO$_CREATE makes either or
// both, according to its modifiers and the FIB:
//
//	IO$_CREATE!IO$M_CREATE, FIB$W_DID set, p2 a name
//	    a new file, entered in that directory as the name
//	IO$_CREATE!IO$M_CREATE, FIB$W_DID zero
//	    a new file with no directory entry (only its file ID finds it)
//	IO$_CREATE (no IO$M_CREATE), FIB$W_DID and FIB$W_FID set, p2 a name
//	    a new directory entry for the existing file FIB$W_FID
//
// Its arguments are IO$_ACCESS's: p1 the FIB's descriptor, p2 the name's,
// p3/p4 where to return the entry made ("NAME.TYP;VER"), p5 an attribute
// list, which IO$_CREATE *writes* to the new file (its record format,
// say). Also from the FIB:
//
//   - FIB$W_NMCTL's FIB$M_NEWVER and FIB$M_SUPERSEDE: what to do when the
//     name's explicit version already exists (the next version instead,
//     or replace the existing file, SS$_SUPERSEDE); with neither,
//     SS$_DUPFILENAME.
//   - FIB$W_EXCTL's FIB$M_EXTEND with FIB$L_EXSZ: blocks to allocate at
//     once. FIB$L_EXSZ comes back as the number allocated (rounded up to
//     the volume's cluster size), and FIB$L_EXVBN as 1, the first.
//   - FIB$W_VERLIMIT: the new file's version limit (0 to inherit one).
//
// The new file ID is stored in FIB$W_FID. The owner is the process's UIC,
// unless the attribute list sets ATR$C_UIC. With IO$M_ACCESS the new file
// is then accessed on the channel, as IO$_ACCESS!IO$M_ACCESS would (for
// writing if FIB$L_ACCTL has FIB$M_WRITE); a file created with no blocks
// must be extended (IO$_MODIFY) before it can be written.
//
// IO$_ACCESS!IO$M_CREATE uses the same code, through createFile, for a
// name its lookup doesn't find.

// diskCreate is IO$_CREATE (see above). IO$M_DELETE (a temporary file)
// isn't supported (SS$_ILLIOFUNC).
func diskCreate(env *Environment, req *ioRequest) (ioStatus, uint32) {
	if req.modified(ioModDelete) {
		return ioStatus{}, ssIllIoFunc
	}

	if env.Mounts == nil {
		return ioStatus{status: ssDevNotMnt}, 0
	}

	f, ok := env.readFIB(req.p[0])
	if !ok {
		return ioStatus{}, ssAccVio
	}

	change, fail := env.writeAttributeList(req)
	if fail != nil {
		return fail.iosb, fail.r0
	}

	name, ok := env.diskFileName(req)
	if !ok {
		return ioStatus{}, ssAccVio
	}

	c := req.channel
	access := req.modified(ioModAccess)

	if access && c.acp != nil {
		return ioStatus{status: ssFilAlrAcc}, 0
	}

	if !req.modified(ioModCreate) {
		return env.enterFile(req, f, name, change, access)
	}

	created, st := env.createFile(req, f, name, change, access)
	if st != ssNormal && st != ssSupersede {
		return ioStatus{status: st}, 0
	}

	c.acp = created.File

	if !env.storeResultName(req, created.Name) {
		return ioStatus{}, ssAccVio
	}

	return ioStatus{status: st}, 0
}

// createFile creates the file f and name describe on the request's disk
// (see this file's opening comment), with the attributes change sets
// (nil for none), accessing it if access. It stores the file ID, and the
// allocation if one was asked for, in the FIB, and returns what rms made
// and the status: SS$_NORMAL, SS$_SUPERSEDE, or an error.
func (env *Environment) createFile(req *ioRequest, f *fib, name string, change func(*rms.ACPAttributes), access bool) (rms.ACPCreated, uint32) {
	inDirectory := f.did != (rms.FileID{})
	if inDirectory && name == "" {
		return rms.ACPCreated{}, ssBadParam
	}

	var blocks uint32
	if f.exctl&fibMExtend != 0 {
		blocks = f.exsz
	}

	created, err := env.Mounts.ACPCreate(diskDevice(req), rms.ACPCreateRequest{
		Directory:    f.did,
		Name:         name,
		NewVersion:   f.nmctl&fibMNewVer != 0,
		Supersede:    f.nmctl&fibMSupersede != 0,
		Blocks:       blocks,
		VersionLimit: f.verlimit,
		Owner:        env.Process.UIC,
		Attributes:   change,
		Access:       access,
		Write:        f.acctl&fibMWrite != 0,
	})
	if err != nil {
		return rms.ACPCreated{}, acpStatus(err)
	}

	env.storeFID(f, fibFID, created.FID)

	if blocks > 0 {
		env.storeFIBLong(f, fibEXSZ, created.Blocks)
		env.storeFIBLong(f, fibEXVBN, 1)
	}

	if created.Superseded {
		return created, ssSupersede
	}

	return created, ssNormal
}

// enterFile is IO$_CREATE without IO$M_CREATE: the existing file
// FIB$W_FID is entered in directory FIB$W_DID as name, then written the
// attributes change sets and, if access, accessed.
func (env *Environment) enterFile(req *ioRequest, f *fib, name string, change func(*rms.ACPAttributes), access bool) (ioStatus, uint32) {
	if f.did == (rms.FileID{}) || f.fid == (rms.FileID{}) || name == "" {
		return ioStatus{status: ssBadParam}, 0
	}

	dev := diskDevice(req)

	full, err := env.Mounts.ACPEnter(dev, f.did, name, f.fid, f.nmctl&fibMNewVer != 0)
	if err != nil {
		return ioStatus{status: acpStatus(err)}, 0
	}

	if !env.storeResultName(req, full) {
		return ioStatus{}, ssAccVio
	}

	if change != nil {
		if err := env.Mounts.ACPWriteAttributes(dev, f.fid, change); err != nil {
			return ioStatus{status: acpStatus(err)}, 0
		}
	}

	if access {
		a, err := env.Mounts.ACPAccess(dev, f.fid, f.acctl&fibMWrite != 0)
		if err != nil {
			return ioStatus{status: acpStatus(err)}, 0
		}

		req.channel.acp = a
	}

	return ioStatus{status: ssNormal}, 0
}

// diskFileName returns the file name whose descriptor is the request's
// p2, or "" for none (p2 0). It reports false if the descriptor or name
// can't be read.
func (env *Environment) diskFileName(req *ioRequest) (string, bool) {
	if req.p[1] == 0 {
		return "", true
	}

	name, ok, err := strGet(env, req.p[1], maxFAOOutput)

	return name, ok && err == nil
}

// storeResultName returns name ("NAME.TYP;VER", or "" for none) through
// the request's p4 (a descriptor) and p3 (a word for its length), where
// given. It reports false if they can't be written.
func (env *Environment) storeResultName(req *ioRequest, name string) bool {
	if req.p[3] == 0 || name == "" {
		return true
	}

	n, _, err := storeDescriptor(env, req.p[3], name)
	if err != nil {
		return false
	}

	if req.p[2] != 0 {
		_ = env.mem.StoreWord(env.cpu, req.p[2], n)
	}

	return true
}
