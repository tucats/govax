package dclcmd

import (
	"errors"
	"fmt"
	"strings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console MOUNT/DISMOUNT commands
// (docs/planning/PHASE-22.md, subtask 12) — the operator-facing half of "attach a
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
// Both methods below have the receiver "(c *Context)", meaning they're
// called as c.Mount(...)/c.Dismount(...) on a *Context value — see
// context.go's Context type for what state they read and write
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
// package's DCL grammar file (internal/bootdata/files/console.dcl)'s "verb
// mount" for how the console command line maps onto these two arguments.
//
// Two things distinguish this from calling c.Mounts.Mount directly:
//
//  1. If device isn't already a known device (internal/io.DeviceTable,
//     normally populated by an explicit DEFINE/DEVICE command), Mount
//     auto-creates one classed as a disk, matching real VMS's own
//     "MOUNT is how a lot of operators first make a device exist at all"
//     workflow — see docs/planning/PHASE-22.md's "Device model" design decision.
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
//
// device may be a logical name ("DISK:") and is translated first; a
// leading "_" suppresses that (docs/planning/PHASE-25.md). Once mounted, the
// volume's label is defined as the logical name DISK$label in the system
// table, as VMS's MOUNT does, so DISK$label:[DIR]FILE reaches it.
func (c *Context) Mount(device, path string, write bool) error {
	device, err := c.mountDevice(device)
	if err != nil {
		return err
	}

	if _, alreadyMounted := c.Mounts.Lookup(device); alreadyMounted {
		return vmserrors.New(vmserrors.SS_DEVMOUNT, device)
	}

	if err := c.Mounts.Mount(device, path, write); err != nil {
		return vmserrors.Wrap(vmserrors.SS_NOMOUNT, err, device)
	}

	if _, found := c.Devices.Find(device); !found {
		c.Devices.Define(device, iodev.DeviceOptions{DevClass: iodev.DeviceClassDisk})
	}

	c.defineVolumeName(device)

	return nil
}

// defineVolumeName defines DISK$label in the system table for the volume
// just mounted on device, as VMS's MOUNT does. A label that isn't a
// usable name just goes without one; the mount itself has already
// succeeded.
func (c *Context) defineVolumeName(device string) {
	if name, ok := c.volumeLogicalName(device); ok {
		eqv := []lnm.Equivalence{{Value: "_" + device + ":", Attrs: lnm.AttrConcealed | lnm.AttrTerminal}}
		_, _ = c.Logicals.Define(lnm.SystemTableName, name, lnm.Executive, 0, eqv)
	}
}

// MountCommand is the console's MOUNT command: Mount, then, when log is
// true (no /NOLOG) and the console is verbose, VMS's message saying what
// was mounted where:
//
//	%MOUNT-I-MOUNTED, WORK mounted on _DUA0:
//
// /NOLOG silences only that message; a failed mount is still reported.
//
// A device given as a controller alone (MOUNT DUA "foo.dsk") is that
// controller's lowest unit with nothing mounted on it (ResolveUnit), and
// the message names the unit chosen.
//
// A volume mounted is known to every process, as VMS's MOUNT/SYSTEM makes
// it, so the command needs SYSNAM (docs/planning/PHASE-94 -
// privileges.md, Decision 10); without it MOUNT says %SYSTEM-F-NOPRIV
// (govax's choice of message). A container file is a host file, so with
// vax.host.sysprv on it needs SYSPRV too (hostCommandPrivilege); a RAM
// disk has none. STARTUP.COM's mounts, before
// INITIALIZE/MEMORY makes process 1, aren't checked.
//
// For an MD device, a RAM disk, file is the volume label, and there is no
// container file (mountRAMDisk; docs/planning/PHASE-96 - RAM disk.md,
// Decision 6). For any other device a missing file is asked for at the
// terminal ("_Container file: "), as DCL asked for it when the grammar
// required it; a command not typed at the terminal gets DCL's
// %DCL-W-INSFPRM instead, and Ctrl/Z at the prompt drops the command.
func (c *Context) MountCommand(device, file string, write, log bool) error {
	if err := c.commandPrivilege("MOUNT"); err != nil {
		return err
	}

	device = c.ResolveUnit(device)

	physical, err := c.mountDevice(device)
	if err != nil {
		return err
	}

	if unit, ok := RAMDiskName(physical); ok {
		return c.mountRAMDisk(unit, file, write, log)
	}

	if err := c.hostCommandPrivilege(); err != nil {
		return err
	}

	if strings.TrimSpace(file) == "" {
		reply, err := c.askContainerFile()
		if err != nil || reply == "" {
			return err
		}

		file = reply
	}

	if err := c.Mount(device, file, write); err != nil {
		return err
	}

	if log {
		c.reportMounted(device, false)
	}

	return nil
}

// askContainerFile asks for MOUNT's container file at the terminal, again
// while the reply is empty, as DCL asks for a required parameter. It
// returns "" with no error when Ctrl/Z ends the prompt, which drops the
// command with $STATUS SS$_NORMAL, shown, as DCL does
// (dclshell.parseCommand). With no terminal to ask, the parameter is
// missing: %DCL-W-INSFPRM.
func (c *Context) askContainerFile() (string, error) {
	if c.AskParameter == nil {
		return "", vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	for {
		reply, ok := c.AskParameter("_Container file: ")
		if !ok {
			if c.SetStatus != nil {
				c.SetStatus(ssNormal | stsInhibitMsg)
			}

			return "", nil
		}

		if reply = strings.TrimSpace(reply); reply != "" {
			return parameterText(reply), nil
		}
	}
}

// parameterText is a reply to a parameter prompt as the grammar reads a
// $string parameter typed on the line: a quoted string without its quotes
// (and "" in it one "), anything else uppercased.
func parameterText(reply string) string {
	if len(reply) >= 2 && reply[0] == '"' && reply[len(reply)-1] == '"' {
		return strings.ReplaceAll(reply[1:len(reply)-1], `""`, `"`)
	}

	return strings.ToUpper(reply)
}

// reportMounted shows, when the console is verbose, the MOUNTED message
// for the volume mounted on device (a name Mount accepted, so its
// translation can't fail), preceded by WRITELOCK when writeLocked says
// the volume could only be mounted read-only.
func (c *Context) reportMounted(device string, writeLocked bool) {
	physical, err := c.mountDevice(device)
	if err != nil {
		return
	}

	label, _ := c.Mounts.VolumeLabel(physical)
	label = strings.ToUpper(strings.TrimSpace(label))

	if writeLocked {
		c.info(vmserrors.New(vmserrors.MOUNT_WRITELOCK))
	}

	c.info(vmserrors.New(vmserrors.MOUNT_MOUNTED, label, "_"+physical+":"))
}

// mountDevice translates a MOUNT/DISMOUNT device argument into the
// physical device it names.
func (c *Context) mountDevice(text string) (string, error) {
	device, err := rms.PhysicalDevice(c.Logicals, text)
	if err != nil {
		if lnmErr := logicalNameFailure(err); lnmErr != nil {
			return "", lnmErr
		}

		return "", vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, text)
	}

	return device, nil
}

// volumeLogicalName returns DISK$label for the volume mounted on device,
// or false when it has no label.
func (c *Context) volumeLogicalName(device string) (string, bool) {
	label, ok := c.Mounts.VolumeLabel(device)
	label = strings.TrimSpace(label)

	if !ok || label == "" {
		return "", false
	}

	return "DISK$" + strings.ToUpper(label), true
}

// commandPrivilege asks the privilege gate about one of govax's own
// commands (corevms.Environment.CommandPrivilege): nil, or the refusal's
// status as the command's error. Without a process (before
// INITIALIZE/MEMORY) there's nothing to ask, and nothing is refused.
func (c *Context) commandPrivilege(command string) error {
	if c.RTL == nil {
		return nil
	}

	if st := c.RTL.CommandPrivilege(command); st != 0 {
		return vmserrors.New(st)
	}

	return nil
}

// hostCommandPrivilege asks the gate about a command that reaches a host
// file of its own, a container file (corevms.Environment.HostCommandAllowed;
// docs/planning/PHASE-97 - host file access.md, Decision 2): nil, or
// %SYSTEM-F-NOPRIV when vax.host.sysprv is on and the process lacks
// SYSPRV. Without a process nothing is refused.
func (c *Context) hostCommandPrivilege() error {
	if c.RTL == nil {
		return nil
	}

	if st := c.RTL.HostCommandAllowed(); st != 0 {
		return vmserrors.New(st)
	}

	return nil
}

// DismountCommand is the console's DISMOUNT command: Dismount, if the
// process holds SYSNAM, as MountCommand needs it.
func (c *Context) DismountCommand(device string) error {
	if err := c.commandPrivilege("DISMOUNT"); err != nil {
		return err
	}

	return c.Dismount(device)
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
//
// Like Mount, it translates device first, and it deletes the DISK$label
// logical name Mount defined for the volume.
//
// A volume with files open isn't dismounted, and the messages say why,
// as VMS's DISMOUNT words them (dismountRefusal).
func (c *Context) Dismount(device string) error {
	device, err := c.mountDevice(device)
	if err != nil {
		return err
	}

	if _, mounted := c.Mounts.Lookup(device); !mounted {
		return vmserrors.New(vmserrors.SS_DEVNOTMOUNT, device)
	}

	// The system volume stays mounted for the whole session, as VMS's
	// system disk does (docs/planning/PHASE-87 - system volume.md). The second
	// line's wording is govax's: VMS's for this case is unconfirmed.
	if c.Mounts.IsSystem(device) {
		head := fmt.Sprintf("DISM-W-CANNOTDMT, %s: cannot be dismounted", device)
		c.Printf("%%%s\n", head)
		c.Printf("%%DISM-W-SYSDISK, device is the system volume\n")

		return vmserrors.InhibitMessage(errors.New(head))
	}

	name, labeled := c.volumeLogicalName(device)

	if err := c.Mounts.Dismount(device); err != nil {
		var open *rms.FilesOpenError
		if errors.As(err, &open) {
			return c.dismountRefusal(open)
		}

		return vmserrors.Wrap(vmserrors.SS_NOMOUNT, err, device)
	}

	if labeled {
		c.deleteVolumeLogicalName(name, device)
	}

	return nil
}

// dismountRefusal shows why a volume with files open can't be
// dismounted, a message for each kind of file, as the DCL Dictionary's
// DISMOUNT example shows VMS's:
//
//	%DISM-W-CANNOTDMT, DUA1: cannot be dismounted
//	%DISM-W-INSWPGFIL, 1 swap or page files installed on volume
//	%DISM-W-USERFILES, 2 user files open on volume
//
// (VMS 7.3's wording; VMS 7.1's is unconfirmed, as no probe can install a
// page file on a volume it may dismount.) It returns the first message,
// already shown, as the command's failure.
func (c *Context) dismountRefusal(open *rms.FilesOpenError) error {
	head := fmt.Sprintf("DISM-W-CANNOTDMT, %s: cannot be dismounted", open.Device)
	c.Printf("%%%s\n", head)

	if open.PageFiles > 0 {
		c.Printf("%%DISM-W-INSWPGFIL, %d swap or page files installed on volume\n", open.PageFiles)
	}

	if open.UserFiles > 0 {
		c.Printf("%%DISM-W-USERFILES, %d user files open on volume\n", open.UserFiles)
	}

	return vmserrors.InhibitMessage(errors.New(head))
}

// deleteVolumeLogicalName deletes name from the system table if it still
// names device (another volume with the same label may have replaced
// it since).
func (c *Context) deleteVolumeLogicalName(name, device string) {
	e, err := c.Logicals.Translate(lnm.SystemTableName, name, lnm.Executive, 0)
	if err != nil || e.Mode != lnm.Executive || e.Equivalences[0].Value != "_"+device+":" {
		return
	}

	_, _ = c.Logicals.Delete(lnm.SystemTableName, name, lnm.Executive)
}
