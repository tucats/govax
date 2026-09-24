package console

import (
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console MOUNT/DISMOUNT commands
// (docs/PHASE-22.md, subtask 12) — the operator-facing half of "attach a
// disk-image container file to a device name". The interesting work
// (actually opening the container and reading its ODS-2 volume structure)
// already lives in the sibling internal/rms package's MountTable
// (mount.go, built in subtask 4); this file is a thin, console-specific
// layer on top of it that adds two things MountTable deliberately doesn't
// know about: auto-creating an internal/io Device record for a device
// name nobody has DEFINE/DEVICE'd yet, and translating internal/rms's
// plain Go errors into the real, numbered VMS status codes an operator
// would see from a genuine MOUNT/DISMOUNT command
// (internal/vmserrors's SS_DEVMOUNT/SS_DEVNOTMOUNT/SS_NOMOUNT).
//
// # A note for a reader new to Go
//
// Both methods below have the receiver "(c *Console)", meaning they're
// called as c.Mount(...)/c.Dismount(...) on a *Console value — see
// machine.go's own Console type for what state they read and write
// (c.Mounts, c.Devices). Returning a plain "error" (Go's built-in error
// interface) is this whole codebase's convention for "something went
// wrong describe it to the caller"; internal/vmserrors.VMSError (what
// vmserrors.New/vmserrors.Wrap actually construct) implements that
// interface, so it can be returned here exactly like any other error.

// Mount attaches the disk-image container file at path to device, making
// it available to internal/rms's SYS$CREATE/SYS$OPEN handlers the way a
// real VMS operator's MOUNT command makes a physical (or, here, emulated)
// disk available to RMS. write selects read/write vs. read-only access,
// matching MOUNT's own /WRITE (default here) and /NOWRITE — see this
// package's DCL grammar file (internal/bootdata/files/evax.dcl)'s "verb
// mount" for how the console command line maps onto these two arguments.
//
// Two things distinguish this from calling c.Mounts.Mount directly:
//
//  1. If device isn't already a known device (internal/io.DeviceTable,
//     normally populated by an explicit DEFINE/DEVICE command), Mount
//     auto-creates one classed as a disk, matching real VMS's own
//     "MOUNT is how a lot of operators first make a device exist at all"
//     workflow — see docs/PHASE-22.md's "Device model" design decision.
//     This only happens after the mount itself succeeds, so a failed
//     MOUNT (bad path, corrupt volume, ...) never leaves a phantom disk
//     device behind for a device name nothing is actually attached to.
//  2. Failures are translated from internal/rms's plain Go errors into
//     the real, literal VMS status codes an actual MOUNT command reports
//     (SS_DEVMOUNT for "already mounted", SS_NOMOUNT for everything
//     else — see vmserrors/codes_sys.go's own doc comment on why these
//     particular numbers were chosen), rather than a generic wrapped
//     string a calling program or operator script would have no reliable
//     way to recognize.
//
// Mount checks "is anything already mounted on device" itself (via
// c.Mounts.Lookup) before ever calling c.Mounts.Mount, rather than trying
// to distinguish MountTable.Mount's "already mounted" failure from every
// other kind of failure by inspecting its error text afterward — a plain
// Go error's text is not a stable, checkable contract the way a real VMS
// status code is, so this reads the same state MountTable.Mount would
// have consulted internally, and reports the precise, correct status
// itself instead of guessing from a string.
func (c *Console) Mount(device, path string, write bool) error {
	if _, alreadyMounted := c.Mounts.Lookup(device); alreadyMounted {
		return vmserrors.New(vmserrors.SS_DEVMOUNT, device)
	}

	if err := c.Mounts.Mount(device, path, write); err != nil {
		return vmserrors.Wrap(vmserrors.SS_NOMOUNT, err, device)
	}

	if _, found := c.Devices.Find(device); !found {
		c.Devices.Define(device, iodev.DeviceOptions{DevClass: iodev.DeviceClassDisk})
	}

	return nil
}

// Dismount detaches whatever container is currently mounted on device,
// flushing any pending writes and closing the underlying container file
// (internal/rms.MountTable.Dismount's own job — see that method's doc
// comment). Like Mount, it checks device's current mount state itself
// first, so a "nothing mounted here" DISMOUNT is reported as the real
// SS_DEVNOTMOUNT status rather than whatever plain error text
// MountTable.Dismount happens to return for the same condition.
//
// Dismount deliberately leaves device's internal/io.Device record (if
// Mount auto-created one, or an operator DEFINE/DEVICE'd it explicitly)
// in place — matching real VMS, where DISMOUNT makes a device unmounted,
// not undefined; the device name is still "known", just with nothing
// attached to it, exactly the state it would be in before the first ever
// MOUNT.
func (c *Console) Dismount(device string) error {
	if _, mounted := c.Mounts.Lookup(device); !mounted {
		return vmserrors.New(vmserrors.SS_DEVNOTMOUNT, device)
	}

	if err := c.Mounts.Dismount(device); err != nil {
		return vmserrors.Wrap(vmserrors.SS_NOMOUNT, err, device)
	}

	return nil
}
