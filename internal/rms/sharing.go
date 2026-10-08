package rms

import (
	"errors"

	"github.com/tucats/ods2/volume"
)

// File sharing (docs/PHASE-47.md): how an RMS open's FAB$B_FAC and
// FAB$B_SHR become the file system's access arbitration.
//
// When RMS opens or creates a file, it accesses it through the file
// system (ods2's volume.Access), saying whether it will write the file and
// whether others may read or write it while it has it open. The file
// system refuses an access that conflicts with the file's other accessors,
// and RMS reports that as RMS$_FLK, "file currently locked by another
// user". The RMS manual's FAB$B_FAC description gives the rule this
// implements: a read (GET) is read access, and PUT, UPD, DEL, and TRN are
// write access; FAB$B_SHR says which of these the opener lets others do.
// With FAB$B_SHR 0, an opener asking only to read shares reading
// (FAB$V_SHRGET) and one asking to write shares nothing (FAB$V_NIL), so
// write sharing must always be asked for; FAB$V_NIL takes precedence over
// any other bit.
//
// While a file is open, every accessor uses one copy of its header (the
// shared volume.File), so one process's appends and extensions are seen
// by the others, and a file deleted while open is deleted at its last
// close.

// The FAB$B_SHR bits.
var (
	shrPut = byte(vmsConst("FAB$M_SHRPUT"))
	shrGet = byte(vmsConst("FAB$M_SHRGET"))
	shrDel = byte(vmsConst("FAB$M_SHRDEL"))
	shrUpd = byte(vmsConst("FAB$M_SHRUPD"))
	shrNil = byte(vmsConst("FAB$M_NIL"))
	shrUPI = byte(vmsConst("FAB$M_UPI"))

	// shrWrite is every bit that lets others write. FAB$V_UPI counts: the
	// manual has it allow "one or more users write access" (to a file
	// they interlock themselves). It lets others read too (unconfirmed:
	// govax's reading of an option meant to be given alone).
	shrWrite = shrPut | shrUpd | shrDel | shrUPI
	shrRead  = shrGet | shrUPI

	// facWrite is every FAB$B_FAC bit that is write access.
	facWrite = facPut | facUpd | byte(vmsConst("FAB$M_DEL")) | byte(vmsConst("FAB$M_TRN"))
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
// (already defaulted: facAccess) and sharing shr asks for.
func accessMode(fac, shr byte) volume.AccessMode {
	shr = effectiveSharing(fac, shr)
	nothing := shr&shrNil != 0

	return volume.AccessMode{
		Write:   fac&facWrite != 0,
		NoRead:  nothing || shr&shrRead == 0,
		NoWrite: nothing || shr&shrWrite == 0,
	}
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
