package rms

import (
	"errors"

	"github.com/tucats/ods2/volume"
)

// File sharing (docs/PHASE-47.md): how an RMS open's FAB$B_FAC and
// FAB$B_SHR decide whether it may share a file with its other openers.
//
// The rules are the Guide to OpenVMS File Applications' (section 7.1.2,
// Tables 7-3 and 7-4), per record operation: GET, PUT, UPDATE, and
// DELETE. An opener's access is the operations its FAB$B_FAC asks for,
// GET included whenever it asks for any (write access implies read
// access); its sharing is the operations its FAB$B_SHR lets others do,
// SHRGET included whenever it lets them do any (write sharing implies
// read sharing), nothing with FAB$V_NIL (which takes precedence), and
// everything with FAB$V_UPI (user interlocking: the openers synchronize
// themselves; unconfirmed that it shares every operation). A new opener
// may join if every operation it asks for is in each current opener's
// sharing, and every operation each current opener has is in its own
// sharing; otherwise RMS$_FLK, "file currently locked by another user".
// With FAB$B_SHR 0, an opener asking only to read shares reading and one
// asking to write shares nothing (the RMS manual's FAB$B_SHR defaults; the
// guide's "SHARING GET" for a reader and "SHARING NONE" for a creator).
//
// Underneath, every RMS open is an access in the file system (ods2's
// volume.Access): whether it writes, and whether it lets others read or
// write at all. That coarser test, which RMS's rule implies, is what
// arbitrates RMS opens against $QIO IO$_ACCESS ones, and while a file is
// open every accessor uses one copy of its header (the shared
// volume.File), so one process's appends and extensions are seen by the
// others, and a file deleted while open is deleted at its last close.

// opSet is a set of record operations, for an opener's access or
// sharing.
type opSet uint8

const (
	opGet opSet = 1 << iota
	opPut
	opUpd
	opDel

	opWrites = opPut | opUpd | opDel
	opAll    = opGet | opWrites
)

// accessOps is the operations an open with access fac asks for.
func accessOps(fac byte) opSet {
	var ops opSet

	if fac&facPut != 0 {
		ops |= opPut
	}

	if fac&facUpd != 0 {
		ops |= opUpd
	}

	if fac&facDel != 0 {
		ops |= opDel
	}

	if fac&facGet != 0 || fac&facWrite != 0 {
		ops |= opGet
	}

	return ops
}

// sharingOps is the operations an open with access fac and sharing shr
// lets others do.
func sharingOps(fac, shr byte) opSet {
	shr = effectiveSharing(fac, shr)

	switch {
	case shr&shrNil != 0:
		return 0
	case shr&shrUPI != 0:
		return opAll
	}

	var ops opSet

	if shr&shrPut != 0 {
		ops |= opPut
	}

	if shr&shrUpd != 0 {
		ops |= opUpd
	}

	if shr&shrDel != 0 {
		ops |= opDel
	}

	if shr&shrGet != 0 || ops != 0 {
		ops |= opGet
	}

	return ops
}

// The FAB$B_SHR bits.
var (
	shrPut = byte(vmsConst("FAB$M_SHRPUT"))
	shrGet = byte(vmsConst("FAB$M_SHRGET"))
	shrDel = byte(vmsConst("FAB$M_SHRDEL"))
	shrUpd = byte(vmsConst("FAB$M_SHRUPD"))
	shrNil = byte(vmsConst("FAB$M_NIL"))
	shrUPI = byte(vmsConst("FAB$M_UPI"))

	// facDel is FAB$M_DEL, and facWrite every FAB$B_FAC bit that is write
	// access.
	facDel   = byte(vmsConst("FAB$M_DEL"))
	facWrite = facPut | facUpd | facDel | byte(vmsConst("FAB$M_TRN"))
)

// fabSHR is FAB$B_SHR, the file sharing field.
var fabSHR = fabOffset("SHR")

// effectiveSharing is the FAB$B_SHR an open with access fac and sharing
// shr has, after the manual's defaults: none given means SHRGET for a
// reader and NIL for a writer.
func effectiveSharing(fac, shr byte) byte {
	if shr&(shrPut|shrGet|shrDel|shrUpd|shrNil|shrUPI) != 0 {
		return shr
	}

	if fac&facWrite != 0 {
		return shrNil
	}

	return shrGet
}

// accessMode is the file system access an RMS open with access fac
// (already defaulted: facAccess) and sharing shr asks for: writing if it
// asks for any write access; others kept from reading if it shares
// nothing, and from writing if it shares no write operation.
func accessMode(fac, shr byte) volume.AccessMode {
	share := sharingOps(fac, shr)

	return volume.AccessMode{
		Write:   fac&facWrite != 0,
		NoRead:  share == 0,
		NoWrite: share&opWrites == 0,
	}
}

// opener is one RMS open of a file, for the arbitration: its access and
// sharing.
type opener struct {
	access, share opSet
}

// compatible reports whether openers a and b may have one file open
// together (the guide's Tables 7-3 and 7-4, each way).
func (a opener) compatible(b opener) bool {
	return a.access&^b.share == 0 && b.access&^a.share == 0
}

// openerClaim is an RMS open's place in its file's list of openers, kept
// by the volume's mountedVolume; release takes it out.
type openerClaim struct {
	m   *mountedVolume
	fid FileID
	o   *opener
}

// claimOpen adds an opener with access fac and sharing shr to the file
// with ID fid on the volume mounted on device, if it's compatible with
// the file's other RMS openers; a nil claim means it isn't (RMS$_FLK).
func (t *MountTable) claimOpen(device string, fid FileID, fac, shr byte) *openerClaim {
	m, ok := t.mounts[normalizeDeviceName(device)]
	if !ok {
		return &openerClaim{}
	}

	o := &opener{access: accessOps(fac), share: sharingOps(fac, shr)}

	for _, other := range m.openers[fid] {
		if !o.compatible(*other) {
			return nil
		}
	}

	if m.openers == nil {
		m.openers = map[FileID][]*opener{}
	}

	m.openers[fid] = append(m.openers[fid], o)

	return &openerClaim{m: m, fid: fid, o: o}
}

// release takes the opener out of its file's list. Releasing again does
// nothing.
func (c *openerClaim) release() {
	if c == nil || c.m == nil || c.o == nil {
		return
	}

	list := c.m.openers[c.fid]
	for i, o := range list {
		if o == c.o {
			list = append(list[:i], list[i+1:]...)

			break
		}
	}

	if len(list) == 0 {
		delete(c.m.openers, c.fid)
	} else {
		c.m.openers[c.fid] = list
	}

	c.o = nil
}

// accessStatus is the RMS status for a failed volume.Access: RMS$_FLK for
// a conflict, RMS$_FNF for a file marked for delete (or one that isn't
// there).
func accessStatus(err error) uint32 {
	if errors.Is(err, volume.ErrAccessConflict) {
		return rmsFileLocked
	}

	return rmsFileNotFound
}

// sharedStream reports whether a stream on a file opened with mode has to
// keep the file current for others: whether anyone else may read or
// write the file. A writer of such a file writes each record through,
// at the file's end of file as it is then (ods2's Writer.SetShared).
func sharedStream(mode volume.AccessMode) bool {
	return !mode.NoRead || !mode.NoWrite
}
