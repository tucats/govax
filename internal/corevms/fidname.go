package corevms

import (
	"errors"
	"strings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
)

// FileName is what FileSpecByID found: a file's specification and the
// status of the walk up its back links.
type FileName struct {
	// Spec is the file's full specification: the volume's logical name
	// and the directory path, "DISK$WORK:[DIR.SUB]NAME.TYP;3"; or, when
	// the back links don't reach the master file directory, the device
	// and the file's name alone; or, when the walk failed, the device
	// alone.
	Spec string

	// LinkStatus is SS$_NORMAL when the back links reached the master
	// file directory, SS$_NOSUCHFILE when they didn't, and the failure
	// when the walk failed.
	LinkStatus uint32
}

// FileSpecByID names the file with ID fid on the disk device names (a
// device or a logical name for one, "DUA1:", "_DUA1:", or NAM$T_DVI's
// "_GOVAX$DUA1"), as LIB$FID_TO_NAME asks the ACP for its file
// specification attribute (docs/PHASE-58, subtask 7), looking for it in
// the directory did when that isn't nil. notDisk reports a device that
// isn't a disk. status is 0, or the status the $ASSIGN or $QIO a VAX
// routine would make reports: the device name's translation's
// (SS$_IVLOGNAM, ...), SS$_NOSUCHDEV, SS$_DEVNOTMOUNT, or the ACP's
// (SS$_NOSUCHFILE, SS$_FILESTRUCT), when fn is the device alone.
//
// The device is named as VMS 7.1 names it (testdata/lib58): by the
// volume's logical name, "DISK$" and its label.
func (env *Environment) FileSpecByID(name string, fid rms.FileID, did *rms.FileID) (fn FileName, notDisk bool, status uint32) {
	if name == "" {
		return FileName{}, false, ssIvLogNam
	}

	device, st := env.deviceName(name)
	if st != 0 {
		return FileName{}, false, st
	}

	d, found := env.Devices.Find(device)
	if !found {
		return FileName{}, false, ssNoSuchDev
	}

	if d.DevClass != iodev.DeviceClassDisk {
		return FileName{}, true, 0
	}

	if env.Mounts == nil {
		return FileName{}, false, acpStatus(rms.ErrACPNotMounted)
	}

	label, ok := env.Mounts.VolumeLabel(d.Name)
	if !ok {
		return FileName{}, false, acpStatus(rms.ErrACPNotMounted)
	}

	prefix := "DISK$" + strings.TrimSpace(label) + ":"

	spec, broken, err := env.Mounts.ACPFileSpec(d.Name, fid, did)
	if err != nil {
		st := acpStatus(err)

		return FileName{Spec: prefix, LinkStatus: st}, false, st
	}

	fn = FileName{Spec: prefix + spec, LinkStatus: ssNormal}
	if errors.Is(broken, rms.ErrBrokenBacklink) {
		fn.LinkStatus = acpStatus(rms.ErrACPNoSuchFile)
	}

	return fn, false, 0
}
