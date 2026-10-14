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
// number's extension); directory-id, when given, is the directory to
// look for the file in, in place of its back link (VMS 7.1 uses it on a
// level 2 disk too, and a directory ID of 0 finds no file).
//
// filespec receives the file's specification, written by its class,
// filespec-length the characters written, and acp-status the status of
// the walk up the back links. As VMS 7.1 does it (testdata/lib58):
//
//   - The specification begins with the volume's logical name,
//     "DISK$LABEL:", whatever form of the device's name the program gave;
//     then "[DIR.SUB]NAME.TYP;VER", the directories' and file's names
//     those of their directory entries; a file its directory has no entry
//     for is "[]NAME.TYP;VER" (rms.MountTable.ACPFileSpec).
//   - When the ACP can't name the file (a file ID that names none,
//     SS$_NOSUCHFILE; a header slot never used, SS$_FILESTRUCT), the
//     specification is the device's name alone, acp-status the ACP's
//     status, and that is the routine's status too.
//   - A device that isn't a disk is LIB$_INVFILSPE, with filespec-length
//     0 and acp-status SS$_NORMAL written (filespec isn't).
//   - A device that doesn't exist is SS$_NOSUCHDEV, an empty device name
//     SS$_IVLOGNAM, one that has no volume mounted SS$_DEVNOTMOUNT, with
//     nothing written.
//
// Statuses: SS$_NORMAL (the manual says LIB$_NORMAL); LIB$_STRTRU when
// filespec is too short; LIB$_INVARG for a required argument omitted
// (fewer than three arguments, or a device-name or filespec of 0) or a
// device name of more than 64 characters; the rest above. An argument it
// can't read or write (a file-id of 0) is an access violation, signaled.
func libFidToName(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 3 || argv[0] == 0 || argv[2] == 0 {
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

	var did *rms.FileID

	if a := arg(argv, 4); a != 0 {
		id, err := readFileID(env, a)
		if err != nil {
			return memoryFault(env, err)
		}

		did = &id
	}

	name, notDisk, status := env.FileSpecByID(device, fid, did)

	switch {
	case notDisk:
		if status, err := storeFIDResults(env, argv, 0, ssNormal); err != nil || status != 0 {
			return status, err
		}

		return libInvFilSpe, nil
	case status != 0 && name.Spec == "":
		return status, nil
	}

	stored, truncated, err := writeString(env, argv[2], name.Spec)
	if err != nil {
		return stringFailure(env, err)
	}

	if status, err := storeFIDResults(env, argv, uint16(stored), name.LinkStatus); err != nil || status != 0 {
		return status, err
	}

	switch {
	case status != 0:
		return status, nil
	case truncated:
		return libStrTru, nil
	}

	return ssNormal, nil
}

// storeFIDResults writes LIB$FID_TO_NAME's filespec-length and
// acp-status, where given. A memory error is signaled.
func storeFIDResults(env *corevms.Environment, argv []uint32, length uint16, acpStatus uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	if a := arg(argv, 3); a != 0 {
		if err := mem.StoreWord(cpu, a, length); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if a := arg(argv, 5); a != 0 {
		if err := mem.StoreLongword(cpu, a, acpStatus); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	return 0, nil
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
