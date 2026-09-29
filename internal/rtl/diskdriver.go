package rtl

import (
	"errors"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// The disk driver (docs/PHASE-26.md subtask 41): $QIO on a disk channel,
// the file-level functions of the disk's ACP.
//
// # Accessing a file
//
// A program reads a file block by block, without RMS, like this:
//
//  1. $ASSIGN a channel to the disk (DUA0:).
//  2. IO$_ACCESS!IO$M_ACCESS, with a *file information block* (FIB,
//     $FIBDEF) describing the file: its file ID (FIB$W_FID), or a
//     directory's file ID (FIB$W_DID) and, in p2, a name to look up in
//     it. The FIB's access control word (FIB$L_ACCTL) asks for write
//     access with FIB$M_WRITE. The channel now has the file open.
//  3. IO$_READVBLK / IO$_WRITEVBLK move virtual blocks: p1 the buffer, p2
//     the byte count, p3 the first virtual block number (1 is the file's
//     first block). A transfer longer than a block covers consecutive
//     blocks.
//  4. IO$_DEACCESS closes the file; so does $DASSGN.
//
// IO$_ACCESS without IO$M_ACCESS only looks the name up, returning the
// file ID in the FIB. IO$_MODIFY with FIB$M_EXTEND in FIB$W_EXCTL adds
// FIB$L_EXSZ blocks to the file, returning how many and the first new
// virtual block in FIB$L_EXSZ and FIB$L_EXVBN.
//
// The function arguments of IO$_ACCESS, IO$_DEACCESS, and IO$_MODIFY:
//
//	p1  the FIB's descriptor          p2  the file name's descriptor
//	p3  a word for the result name's length
//	p4  the result name's descriptor ("NAME.TYP;VER")
//	p5  an attribute list: read by IO$_ACCESS, written by the others
//	    (diskattr.go)
//
// Errors in the request itself (a buffer or FIB the program can't
// access) are $QIO's R0 (SS$_ACCVIO); the rest are the I/O status block's.
//
// # How govax does it
//
// The operations on the file are internal/rms's (acp.go), built on the
// ods2 volume package, the one place govax reads and writes ODS-2 disks.
// This file decodes the $QIO arguments and the FIB, keeps the accessed
// file on the channel, and turns rms's errors into $SSDEF statuses. The
// device must be mounted (the console's MOUNT command). Every request
// completes during the call.

// The disk driver's function table.
var diskFunctions = map[uint32]ioFunc{
	ioCode("IO$_ACCESS"):    diskAccess,
	ioCode("IO$_CREATE"):    diskCreate,
	ioCode("IO$_DELETE"):    diskDelete,
	ioCode("IO$_DEACCESS"):  diskDeaccess,
	ioCode("IO$_MODIFY"):    diskModify,
	ioCode("IO$_READVBLK"):  diskReadVirtual,
	ioCode("IO$_WRITEVBLK"): diskWriteVirtual,
}

var (
	ioModAccess = ioCode("IO$M_ACCESS")
	ioModCreate = ioCode("IO$M_CREATE")
	ioModDelete = ioCode("IO$M_DELETE")
)

// FIB field offsets and bits ($FIBDEF, generated).
var (
	fibACCTL    = vmsdef.FIBConstants["FIB$L_ACCTL"]
	fibFID      = vmsdef.FIBConstants["FIB$W_FID"]
	fibDID      = vmsdef.FIBConstants["FIB$W_DID"]
	fibEXCTL    = vmsdef.FIBConstants["FIB$W_EXCTL"]
	fibEXSZ     = vmsdef.FIBConstants["FIB$L_EXSZ"]
	fibEXVBN    = vmsdef.FIBConstants["FIB$L_EXVBN"]
	fibNMCTL    = vmsdef.FIBConstants["FIB$W_NMCTL"]
	fibVERLIMIT = vmsdef.FIBConstants["FIB$W_VERLIMIT"]
	fibMWrite   = vmsdef.FIBConstants["FIB$M_WRITE"]
	fibMExtend  = vmsdef.FIBConstants["FIB$M_EXTEND"]
	// $FIBDEF's listings give no FIB$M_ masks for the name control bits,
	// only their bit numbers.
	fibMNewVer    = uint32(1) << vmsdef.FIBConstants["FIB$V_NEWVER"]
	fibMSupersede = uint32(1) << vmsdef.FIBConstants["FIB$V_SUPERSEDE"]
	fibMaxBytes   = vmsdef.FIBConstants["FIB$C_LENGTH"]
)

// acpStatuses maps internal/rms's ACP errors to $SSDEF statuses.
var acpStatuses = []struct {
	err    error
	status uint32
}{
	{rms.ErrACPNotMounted, vmsdef.SSConstants["SS$_DEVNOTMOUNT"]},
	{rms.ErrACPNoSuchFile, vmsdef.SSConstants["SS$_NOSUCHFILE"]},
	{rms.ErrACPBadDirectory, vmsdef.SSConstants["SS$_BADIRECTORY"]},
	{rms.ErrACPBadName, vmsdef.SSConstants["SS$_BADFILENAME"]},
	{rms.ErrACPBadVersion, vmsdef.SSConstants["SS$_BADFILEVER"]},
	{rms.ErrACPWriteLocked, vmsdef.SSConstants["SS$_WRITLCK"]},
	{rms.ErrACPReadOnly, vmsdef.SSConstants["SS$_NOPRIV"]},
	{rms.ErrACPEndOfFile, vmsdef.SSConstants["SS$_ENDOFFILE"]},
	{rms.ErrACPBadBlock, vmsdef.SSConstants["SS$_BADPARAM"]},
	{rms.ErrACPDeviceFull, vmsdef.SSConstants["SS$_DEVICEFULL"]},
	{rms.ErrACPDuplicate, vmsdef.SSConstants["SS$_DUPFILENAME"]},
	{rms.ErrACPDirNotEmpty, vmsdef.SSConstants["SS$_DIRNOTEMPTY"]},
	{rms.ErrACPProtected, vmsdef.SSConstants["SS$_NOPRIV"]},
}

// Other statuses the disk driver reports.
var (
	ssFilAlrAcc = vmsdef.SSConstants["SS$_FILALRACC"]
	ssFilNotAcc = vmsdef.SSConstants["SS$_FILNOTACC"]
	ssDrvErr    = vmsdef.SSConstants["SS$_DRVERR"]
	ssDevNotMnt = vmsdef.SSConstants["SS$_DEVNOTMOUNT"]
	ssCreated   = vmsdef.SSConstants["SS$_CREATED"]
)

// acpStatus is err's $SSDEF status: one of acpStatuses, or SS$_DRVERR
// for anything else (a failure reading or writing the disk image).
func acpStatus(err error) uint32 {
	for _, s := range acpStatuses {
		if errors.Is(err, s.err) {
			return s.status
		}
	}

	return ssDrvErr
}

// fib is a program's file information block, as read from its memory:
// only the fields govax uses. addr and size say where it is, for writing
// fields back.
type fib struct {
	addr, size uint32
	acctl      uint32
	fid, did   rms.FileID
	exctl      uint32
	exsz       uint32
	nmctl      uint32
	verlimit   uint16
}

// readFIB reads the FIB whose descriptor is at desc. A FIB may be shorter
// than the full structure (VMS allows that): fields past its end read as
// 0. It reports false if the descriptor or FIB can't be read, or is too
// short to hold a file ID.
func (env *Environment) readFIB(desc uint32) (*fib, bool) {
	if desc == 0 {
		return nil, false
	}

	size, err1 := env.mem.LoadWord(env.cpu, desc)
	addr, err2 := env.mem.LoadLongword(env.cpu, desc+4)

	if err1 != nil || err2 != nil || uint32(size) < fibDID {
		return nil, false
	}

	n := min(uint32(size), fibMaxBytes)

	raw, err := loadBytes(env, addr, int(n))
	if err != nil {
		return nil, false
	}

	b := []byte(raw)
	word := func(off uint32) uint16 {
		if off+2 > n {
			return 0
		}

		return uint16(b[off]) | uint16(b[off+1])<<8
	}
	long := func(off uint32) uint32 { return uint32(word(off)) | uint32(word(off+2))<<16 }
	id := func(off uint32) rms.FileID {
		rvn := word(off + 4)

		return rms.FileID{Num: word(off), Seq: word(off + 2), Rvn: uint8(rvn), Nmx: uint8(rvn >> 8)}
	}

	return &fib{
		addr: addr, size: n,
		acctl:    long(fibACCTL),
		fid:      id(fibFID),
		did:      id(fibDID),
		exctl:    uint32(word(fibEXCTL)),
		exsz:     long(fibEXSZ),
		nmctl:    uint32(word(fibNMCTL)),
		verlimit: word(fibVERLIMIT),
	}, true
}

// storeFID writes a file ID into the FIB at offset off, if the FIB is
// long enough to hold it.
func (env *Environment) storeFID(f *fib, off uint32, id rms.FileID) {
	if off+6 > f.size {
		return
	}

	_ = env.mem.StoreWord(env.cpu, f.addr+off, id.Num)
	_ = env.mem.StoreWord(env.cpu, f.addr+off+2, id.Seq)
	_ = env.mem.StoreWord(env.cpu, f.addr+off+4, uint16(id.Rvn)|uint16(id.Nmx)<<8)
}

// storeFIBLong writes a longword into the FIB at offset off, if it fits.
func (env *Environment) storeFIBLong(f *fib, off, v uint32) {
	if off+4 <= f.size {
		_ = env.mem.StoreLongword(env.cpu, f.addr+off, v)
	}
}

// diskDevice returns the mount table name of the request's disk.
func diskDevice(req *ioRequest) string {
	return req.channel.Device.Name
}

// diskAccess is IO$_ACCESS (see this file's opening comment). With a
// name (p2) and a directory ID in the FIB, the name is looked up there,
// the file ID stored in the FIB, and the full name returned through
// p3/p4. With IO$M_CREATE, a name the lookup doesn't find is created
// instead, as IO$_CREATE!IO$M_CREATE would (diskcreate.go), and the
// status is SS$_CREATED. With IO$M_ACCESS the file (by the FIB's file ID,
// looked up or given) is then opened on the channel, for writing if
// FIB$L_ACCTL has FIB$M_WRITE. Last, the attributes p5's list asks for
// are read from the file (accessed or not) into the program's buffers.
// Deleting (IO$M_DELETE) isn't an IO$_ACCESS modifier (SS$_ILLIOFUNC).
func diskAccess(env *Environment, req *ioRequest) (ioStatus, uint32) {
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

	attrs, st := env.readAttributeList(req.p[4], false)

	switch st {
	case ssAccVio:
		return ioStatus{}, ssAccVio
	case ssBadAttrib:
		return ioStatus{status: ssBadAttrib}, 0
	}

	c := req.channel
	open := req.modified(ioModAccess)

	if open && c.acp != nil {
		return ioStatus{status: ssFilAlrAcc}, 0
	}

	fid := f.fid
	status := uint32(ssNormal)

	name, ok := env.diskFileName(req)
	if !ok {
		return ioStatus{}, ssAccVio
	}

	if name != "" && f.did != (rms.FileID{}) {
		found, full, err := env.Mounts.ACPLookup(diskDevice(req), f.did, name)

		if errors.Is(err, rms.ErrACPNoSuchFile) && req.modified(ioModCreate) {
			// Create it: accessed below, like a file found.
			created, st := env.createFile(req, f, name, nil, false)
			if st != ssNormal && st != ssSupersede {
				return ioStatus{status: st}, 0
			}

			found, full, err, status = created.FID, created.Name, nil, ssCreated
		}

		if err != nil {
			return ioStatus{status: acpStatus(err)}, 0
		}

		fid = found
		env.storeFID(f, fibFID, fid)

		if !env.storeResultName(req, full) {
			return ioStatus{}, ssAccVio
		}
	}

	if fid == (rms.FileID{}) {
		return ioStatus{status: ssBadParam}, 0
	}

	if open {
		a, err := env.Mounts.ACPAccess(diskDevice(req), fid, f.acctl&fibMWrite != 0)
		if err != nil {
			return ioStatus{status: acpStatus(err)}, 0
		}

		c.acp = a
	}

	if len(attrs) > 0 {
		var (
			values rms.ACPAttributes
			err    error
		)

		if open {
			values = c.acp.Attributes()
		} else {
			values, err = env.Mounts.ACPReadAttributes(diskDevice(req), fid)
		}

		if err != nil {
			return ioStatus{status: acpStatus(err)}, 0
		}

		env.storeAttributes(attrs, &values)
	}

	return ioStatus{status: status}, 0
}

// writeAttributeList decodes p5's attribute list for writing, returning
// the change it makes (nil for none) and, if it can't be done, the
// request's result: SS$_ACCVIO in R0 or SS$_BADATTRIB in the IOSB.
func (env *Environment) writeAttributeList(req *ioRequest) (func(*rms.ACPAttributes), *ioResult) {
	attrs, st := env.readAttributeList(req.p[4], true)

	switch st {
	case ssAccVio:
		return nil, &ioResult{r0: ssAccVio}
	case ssBadAttrib:
		return nil, &ioResult{iosb: ioStatus{status: ssBadAttrib}}
	}

	change, ok := env.attributeChange(attrs)
	if !ok {
		return nil, &ioResult{r0: ssAccVio}
	}

	return change, nil
}

// ioResult is a driver function's two results, for helpers that can end a
// request early.
type ioResult struct {
	iosb ioStatus
	r0   uint32
}

// diskDeaccess is IO$_DEACCESS: the channel's file is closed, after
// which the attributes in p5's list are written to it (how RMS records
// the end of file). Writing attributes needs write access (SS$_NOPRIV);
// the file is closed even then.
func diskDeaccess(env *Environment, req *ioRequest) (ioStatus, uint32) {
	c := req.channel
	if c.acp == nil {
		return ioStatus{status: ssFilNotAcc}, 0
	}

	change, fail := env.writeAttributeList(req)
	if fail != nil {
		return fail.iosb, fail.r0
	}

	err := c.acp.DeaccessWithAttributes(change)
	c.acp = nil

	if err != nil {
		return ioStatus{status: acpStatus(err)}, 0
	}

	return ioStatus{status: ssNormal}, 0
}

// diskModify is IO$_MODIFY. With FIB$M_EXTEND in the FIB's FIB$W_EXCTL,
// it adds FIB$L_EXSZ blocks to the channel's file (accessed for writing),
// storing the number added in FIB$L_EXSZ and the first new virtual block
// in FIB$L_EXVBN. Then the attributes in p5's list are written: to the
// channel's file if one is accessed (for writing: SS$_NOPRIV otherwise),
// or else to the file the FIB's file ID names. The FIB (p1) may be left
// out when a file is accessed. Anything else it could modify (truncating)
// isn't supported and does nothing.
func diskModify(env *Environment, req *ioRequest) (ioStatus, uint32) {
	c := req.channel

	var f *fib

	if req.p[0] != 0 || c.acp == nil {
		var ok bool
		if f, ok = env.readFIB(req.p[0]); !ok {
			return ioStatus{}, ssAccVio
		}
	}

	change, fail := env.writeAttributeList(req)
	if fail != nil {
		return fail.iosb, fail.r0
	}

	if f != nil && f.exctl&fibMExtend != 0 {
		if c.acp == nil {
			return ioStatus{status: ssFilNotAcc}, 0
		}

		first, err := c.acp.Extend(f.exsz)
		if err != nil {
			return ioStatus{status: acpStatus(err)}, 0
		}

		env.storeFIBLong(f, fibEXSZ, f.exsz)
		env.storeFIBLong(f, fibEXVBN, first)
	}

	if change == nil {
		// No attributes: IO$_MODIFY is about the accessed file.
		if c.acp == nil {
			return ioStatus{status: ssFilNotAcc}, 0
		}

		return ioStatus{status: ssNormal}, 0
	}

	var err error

	switch {
	case c.acp != nil:
		err = c.acp.WriteAttributes(change)
	case env.Mounts == nil:
		return ioStatus{status: ssDevNotMnt}, 0
	case f.fid == (rms.FileID{}):
		return ioStatus{status: ssBadParam}, 0
	default:
		err = env.Mounts.ACPWriteAttributes(diskDevice(req), f.fid, change)
	}

	if err != nil {
		return ioStatus{status: acpStatus(err)}, 0
	}

	return ioStatus{status: ssNormal}, 0
}

// diskReadVirtual is IO$_READVBLK: p2 bytes (up to 65,535) from virtual
// block p3 of the channel's file into the buffer at p1. The IOSB's count
// is the bytes read; reading past the end of the file stops there with
// SS$_ENDOFFILE.
func diskReadVirtual(env *Environment, req *ioRequest) (ioStatus, uint32) {
	size := req.p[1] & 0xFFFF
	if !env.accessible(req.p[0], size, vm.AccessWrite) {
		return ioStatus{}, ssAccVio
	}

	c := req.channel
	if c.acp == nil {
		return ioStatus{status: ssFilNotAcc}, 0
	}

	data, err := c.acp.ReadVirtual(req.p[2], size)
	if len(data) > 0 {
		_ = env.mem.Store(env.cpu, req.p[0], data)
	}

	status := uint32(ssNormal)
	if err != nil {
		status = acpStatus(err)
	}

	return ioStatus{status: status, count: uint16(len(data))}, 0
}

// diskWriteVirtual is IO$_WRITEVBLK: p2 bytes from the buffer at p1 to
// the channel's file (accessed for writing) from virtual block p3, as
// whole blocks. Only allocated blocks can be written: a transfer that
// would pass the end of the allocation writes nothing and completes with
// SS$_ENDOFFILE (IO$_MODIFY extends the file). The IOSB's count is the
// bytes written.
func diskWriteVirtual(env *Environment, req *ioRequest) (ioStatus, uint32) {
	size := req.p[1] & 0xFFFF
	if !env.accessible(req.p[0], size, vm.AccessRead) {
		return ioStatus{}, ssAccVio
	}

	c := req.channel
	if c.acp == nil {
		return ioStatus{status: ssFilNotAcc}, 0
	}

	data, err := loadBytes(env, req.p[0], int(size))
	if err != nil {
		return ioStatus{}, ssAccVio
	}

	if err := c.acp.WriteVirtual(req.p[2], []byte(data)); err != nil {
		return ioStatus{status: acpStatus(err)}, 0
	}

	return ioStatus{status: ssNormal, count: uint16(size)}, 0
}

// deaccessChannel closes the file accessed on channel c, if any: $DASSGN
// and image rundown's deassignments do this.
func (env *Environment) deaccessChannel(c *channel) {
	if c.acp != nil {
		_ = c.acp.Deaccess()
		c.acp = nil
	}
}
