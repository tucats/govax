package corevms

import (
	"errors"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
)

// FileName is what FileSpecByID found: a file's specification and the
// status of the walk up its back links.
type FileName struct {
	// Spec is the file's full specification: the device's full name and
	// the directory path, "_GOVAX$DUA1:[DIR.SUB]NAME.TYP;3"; or, when
	// the back links don't reach the master file directory, the device
	// and the file's name alone.
	Spec string

	// LinkStatus is SS$_NORMAL when the back links reached the master
	// file directory, SS$_NOSUCHFILE when they didn't.
	LinkStatus uint32
}

// FileSpecByID names the file with ID fid on the disk device names (a
// device or a logical name for one, "DUA1:", "_DUA1:", or NAM$T_DVI's
// "_GOVAX$DUA1"), as LIB$FID_TO_NAME asks the ACP for its file
// specification attribute (docs/PHASE-58, subtask 7). notDisk reports a
// device that isn't a disk. status is 0, or the status the $ASSIGN or
// $QIO a VAX routine would make reports: the device name's translation's
// (SS$_IVDEVNAM, ...), SS$_NOSUCHDEV, SS$_DEVNOTMOUNT, or
// SS$_NOSUCHFILE for a file ID that names no file.
func (env *Environment) FileSpecByID(name string, fid rms.FileID) (fn FileName, notDisk bool, status uint32) {
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

	spec, broken, err := env.Mounts.ACPFileSpec(d.Name, fid)
	if err != nil {
		return FileName{}, false, acpStatus(err)
	}

	fn = FileName{Spec: nodeDeviceName(env, d) + spec, LinkStatus: ssNormal}
	if errors.Is(broken, rms.ErrBrokenBacklink) {
		fn.LinkStatus = acpStatus(rms.ErrACPNoSuchFile)
	}

	return fn, false, 0
}
