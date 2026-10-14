package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
)

// LIB$FID_TO_NAME (docs/PHASE-58, subtask 7; the survey's "FID_TO_NAME
// (149)"), written from the 5.0 LIB$ manual (LIB-149):
//
//	LIB$FID_TO_NAME device-name ,file-id ,filespec [,filespec-length]
//	                [,directory-id] [,acp-status]
//
// It names a file from its device and file ID, the two things RMS leaves
// in a NAM (NAM$T_DVI, NAM$W_FID). VMS asks the disk's ACP for the file
// specification attribute, which follows the file header's back link,
// directory by directory, to the master file directory; govax's ACP
// lacks that attribute, so the walk is RMS's (rms.MountTable.ACPFileSpec,
// through corevms.Environment.FileSpecByID).

// maxFIDDeviceName is the longest device-name the manual allows.
const maxFIDDeviceName = 64

// libFidToName is LIB$FID_TO_NAME. device-name is a disk device (or a
// logical name for one), file-id the address of three words ($FIDDEF:
// file number, sequence number, relative volume number and the file
// number's extension); directory-id is for structure level 1 disks and
// is ignored (govax's volumes are all level 2).
//
// filespec receives the file's specification, written by its class,
// filespec-length the characters written, and acp-status the status of
// the walk up the back links: all three only when the routine succeeds.
//
// Choices the manual leaves open, each **unconfirmed** (probe cases):
//
//   - The specification begins with the device's full name, as NAM$T_DVI
//     and DVI$_FULLDEVNAM have it ("_GOVAX$DUA1:"), whatever form of
//     the name the program gave; then "[DIR.SUB]NAME.TYP;VER", the
//     directories' and file's names those of their directory entries.
//   - A file whose back links don't reach the master file directory (a
//     back link of 0, or to a directory that has gone) is named without a
//     directory, acp-status SS$_NOSUCHFILE, and the routine still
//     succeeds.
//   - A file ID that names no file is SS$_NOSUCHFILE, the $QIO's status.
//   - A device that doesn't exist is SS$_NOSUCHDEV, one that has no
//     volume mounted SS$_DEVNOTMOUNT, as $ASSIGN and $QIO would report.
//
// Statuses: LIB$_NORMAL; LIB$_STRTRU when filespec is too short;
// LIB$_INVARG for a required argument omitted (fewer than three
// arguments, or one of 0) or a device name of more than 64 characters;
// LIB$_INVFILSPE for a device that isn't a disk; LIB$_INVSTRDES and the
// rest of the string routines'; the device and file statuses above. An
// argument it can't read or write is an access violation, signaled.
func libFidToName(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 3 || argv[0] == 0 || argv[1] == 0 || argv[2] == 0 {
		return libInvArg, nil
	}

	device, err := readString(env, argv[0])
	if err != nil {
		return stringFailure(env, err)
	}

	if len(device) > maxFIDDeviceName {
		return libInvArg, nil
	}

	fid, err := readFileID(env, argv[1])
	if err != nil {
		return memoryFault(env, err)
	}

	name, notDisk, status := env.FileSpecByID(device, fid)

	switch {
	case notDisk:
		return libInvFilSpe, nil
	case status != 0:
		return status, nil
	}

	stored, truncated, err := writeString(env, argv[2], name.Spec)
	if err != nil {
		return stringFailure(env, err)
	}

	mem, cpu := env.Memory(), env.CPU()

	if a := arg(argv, 3); a != 0 {
		if err := mem.StoreWord(cpu, a, uint16(stored)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if a := arg(argv, 5); a != 0 {
		if err := mem.StoreLongword(cpu, a, name.LinkStatus); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if truncated {
		return libStrTru, nil
	}

	return libNormal, nil
}

// readFileID reads the three-word file ID at addr ($FIDDEF): FID$W_NUM,
// FID$W_SEQ, then FID$B_RVN and FID$B_NMX.
func readFileID(env *corevms.Environment, addr uint32) (rms.FileID, error) {
	mem, cpu := env.Memory(), env.CPU()

	num, err := mem.LoadWord(cpu, addr)
	if err != nil {
		return rms.FileID{}, err
	}

	seq, err := mem.LoadWord(cpu, addr+2)
	if err != nil {
		return rms.FileID{}, err
	}

	last, err := mem.LoadWord(cpu, addr+4)
	if err != nil {
		return rms.FileID{}, err
	}

	return rms.FileID{Num: num, Seq: seq, Rvn: uint8(last), Nmx: uint8(last >> 8)}, nil
}
