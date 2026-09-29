package rtl

import (
	"github.com/tucats/govax/internal/vm"
)

// Logical and physical block I/O on a disk (docs/PHASE-26.md subtask 45).
//
// IO$_READLBLK and IO$_WRITELBLK read and write the disk's *logical*
// blocks: numbered from 0 across the whole volume, with no files involved
// (LBN 1 is the home block, for instance). IO$_READPBLK and IO$_WRITEPBLK
// do the same with *physical* blocks, the drive's own addressing, which
// on the MSCP disks govax emulates is the same numbering. (Virtual blocks,
// a file's own, are IO$_READVBLK's: diskdriver.go.)
//
// The arguments are the virtual block functions':
//
//	p1  the buffer           p2  the byte count (up to 65,535)
//	p3  the first block number
//
// A transfer covers consecutive blocks; a write's short last block is
// padded with zeros. The whole transfer must lie on the volume
// (SS$_ILLBLKNUM, nothing moved, otherwise), and a write needs a writable
// mount (SS$_WRITLCK). The IOSB's count is the bytes moved.
//
// Going around the file system this way needs a privilege, checked before
// the request is queued (so it's $QIO's R0, SS$_NOPRIV): LOG_IO or PHY_IO
// for logical I/O, PHY_IO for physical. govax's process has both unless a
// program disables them ($SETPRV). The volume must be mounted: govax has
// a disk's blocks only through its mounted image. The work is
// internal/rms's (acplogical.go).

// The block I/O privileges.
var (
	privLOGIO = privilegeBit("LOG_IO")
	privPHYIO = privilegeBit("PHY_IO")
)

// diskReadLogical is IO$_READLBLK.
func diskReadLogical(env *Environment, req *ioRequest) (ioStatus, uint32) {
	return env.diskBlockRead(req, privLOGIO|privPHYIO)
}

// diskWriteLogical is IO$_WRITELBLK.
func diskWriteLogical(env *Environment, req *ioRequest) (ioStatus, uint32) {
	return env.diskBlockWrite(req, privLOGIO|privPHYIO)
}

// diskReadPhysical is IO$_READPBLK.
func diskReadPhysical(env *Environment, req *ioRequest) (ioStatus, uint32) {
	return env.diskBlockRead(req, privPHYIO)
}

// diskWritePhysical is IO$_WRITEPBLK.
func diskWritePhysical(env *Environment, req *ioRequest) (ioStatus, uint32) {
	return env.diskBlockWrite(req, privPHYIO)
}

// diskBlockRead reads p2 bytes from block p3 into the buffer at p1, for a
// process with at least one of the privileges in privs.
func (env *Environment) diskBlockRead(req *ioRequest, privs uint64) (ioStatus, uint32) {
	if !env.Process.hasAnyPrivilege(privs) {
		return ioStatus{}, ssNoPriv
	}

	size := req.p[1] & 0xFFFF
	if !env.accessible(req.p[0], size, vm.AccessWrite) {
		return ioStatus{}, ssAccVio
	}

	if env.Mounts == nil {
		return ioStatus{status: ssDevNotMnt}, 0
	}

	data, err := env.Mounts.ReadLogical(diskDevice(req), req.p[2], size)
	if len(data) > 0 {
		_ = env.mem.Store(env.cpu, req.p[0], data)
	}

	if err != nil {
		return ioStatus{status: acpStatus(err), count: uint16(len(data))}, 0
	}

	return ioStatus{status: ssNormal, count: uint16(len(data))}, 0
}

// diskBlockWrite writes p2 bytes from the buffer at p1 to block p3, for a
// process with at least one of the privileges in privs.
func (env *Environment) diskBlockWrite(req *ioRequest, privs uint64) (ioStatus, uint32) {
	if !env.Process.hasAnyPrivilege(privs) {
		return ioStatus{}, ssNoPriv
	}

	size := req.p[1] & 0xFFFF
	if !env.accessible(req.p[0], size, vm.AccessRead) {
		return ioStatus{}, ssAccVio
	}

	if env.Mounts == nil {
		return ioStatus{status: ssDevNotMnt}, 0
	}

	data, err := loadBytes(env, req.p[0], int(size))
	if err != nil {
		return ioStatus{}, ssAccVio
	}

	if err := env.Mounts.WriteLogical(diskDevice(req), req.p[2], []byte(data)); err != nil {
		return ioStatus{status: acpStatus(err)}, 0
	}

	return ioStatus{status: ssNormal, count: uint16(size)}, 0
}
