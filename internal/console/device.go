package console

import (
	"fmt"
	"strings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vmsdef"
)

// DefineDevice implements the DEFINE/DEVICE console command, registering
// a new device in c.Devices. Unlike most console commands, this doesn't
// require INIT.
func (c *Console) DefineDevice(name string, opts iodev.DeviceOptions) *iodev.Device {
	return c.Devices.Define(name, opts)
}

// ShowDevices implements the SHOW DEVICES console command,optionally
// filtered to one device by name and expanded to full detail
// with /FULL. Matches show_device's own behavior of printing nothing at
// all when no device matches (no "no matching devices" fallback message —
// unlike ShowLogicals, which does print one; that asymmetry is in the C
// source, not invented here).
//
// A disk-class device's /FULL output (showDiskDeviceFull) is a from-scratch,
// VMS-realistic reformat.
func (c *Console) ShowDevices(name string, full bool) error {
	for _, d := range c.Devices.All() {
		if name != "" && d.Name != strings.ToUpper(strings.TrimSuffix(name, ":")) {
			continue
		}

		if !full {
			c.Printf("Device %s\n", d.Name)

			continue
		}

		if d.DevClass == iodev.DeviceClassDisk {
			c.showDiskDeviceFull(d)

			continue
		}

		if d.DevClass == iodev.DeviceClassMailbox && d.DevType == iodev.DeviceTypeNull {
			c.showNullDeviceFull(d)

			continue
		}

		allocated := ""
		if d.Allocated() {
			allocated = ", allocated"
		}

		c.Printf("Device %s%s\n", d.Name, allocated)
		c.Printf("    DEVCLASS=%d (%s)   DEVTYPE=%d\n", d.DevClass, iodev.DeviceClassName(d.DevClass), d.DevType)
		c.Printf("    DEVBUFSIZE=%d\n", d.DevBufSize)
		c.Printf("    RECSIZE=%d\n", d.RecSize)
		c.Printf("    DEVCHAR=%08X    DEVCHAR2=%08X\n", d.DevChar, d.DevChar2)
		c.Printf("    DEVDEPEND=%08X  DEVDEPEND2=%08X\n", d.DevDepend, d.DevDepend2)
		c.Printf("    PID=%08X        OWNUIC=%08X\n", d.PID, d.OwnUIC)
		c.Printf("    LOCKID=%08X\n", d.LockID)
		c.Printf("    REFCNT=%d\n", d.RefCnt)
	}

	return nil
}

// showDiskDeviceFull prints SHOW DEVICE/FULL's disk-class output in the
// two-column layout real VMS uses, e.g.:
//
//	Disk DUA0:, device type RD54, is online, mounted (READ/WRITE), file-oriented device.
//
//	    Error count                    0    Operations completed              12887
//	    Reference count               36    Default buffer size                 512
//	    Total blocks              311200    Free blocks                         736
//
//	    Volume label         "OPENVMS071"    Cluster size                         3
//	    Number of files               353    Maximum files allowed            38900
//
// "Operations completed" is, for a mounted volume, the logical I/O
// operations (block reads and writes) ods2 has counted since the mount
// (rms.MountTable.Operations), and d.OpCnt otherwise.
//
// "Reference count" is d.RefCnt, the count SYS$ASSIGN already maintains
// (internal/rtl/devices.go) — the real VMS meaning of a device's
// reference count (outstanding channel assigns), not anything specific to
// the mounted ODS-2 volume.
//
// "Free blocks"/"Number of files"/"Total blocks"/"Cluster size"/"Maximum
// files allowed" come live from c.Mounts.VolumeStats when d is actually
// mounted (real, on-disk numbers via github.com/tucats/ods2/volume.Stats),
// falling back to d's own static DEFINE/DEVICE fields — normally all
// zero, since MOUNT's own auto-created device records never set them —
// when nothing is mounted or the live scan fails. "Volume label"
// similarly prefers the live c.Mounts.VolumeLabel over d.VolName.
//
// govax has no online/offline concept, so every disk device is reported
// "is online" unconditionally; "allocated" follows it, as on VMS, when
// $ALLOC has allocated the device (docs/PHASE-26.md); "device type X" is omitted whenever
// d.DevType doesn't map to a known name (iodev.DeviceTypeName) rather
// than printing a placeholder — see that function's own doc comment.
func (c *Console) showDiskDeviceFull(d *iodev.Device) {
	stats, mounted, statErr := c.Mounts.VolumeStats(d.Name)
	live := mounted && statErr == nil

	clusterSize, maxFiles := d.Cluster, d.MaxFiles
	totalBlocks, freeBlocks, fileCount := d.MaxBlock, d.FreeBlocks, uint32(0)
	volLabel := d.VolName

	if live {
		clusterSize, maxFiles = uint32(stats.ClusterSize), stats.MaxFiles
		totalBlocks, freeBlocks, fileCount = stats.TotalBlocks, stats.FreeBlocks, stats.FileCount
		volLabel, _ = c.Mounts.VolumeLabel(d.Name)
	}

	typeClause := ""
	if typeName, ok := iodev.DeviceTypeName(d.DevType); ok {
		typeClause = fmt.Sprintf(", device type %s", typeName)
	}

	mountClause := ""

	if mounted {
		access := "READ ONLY"
		if c.Mounts.Writable(d.Name) {
			access = "READ/WRITE"
		}

		mountClause = fmt.Sprintf(", mounted (%s)", access)
	}

	allocClause := ""
	if d.Allocated() {
		allocClause = ", allocated"
	}

	// A mounted volume's operations are ods2's count of its block reads
	// and writes; otherwise the device record's own (normally 0).
	operations := d.OpCnt
	if n, ok := c.Mounts.Operations(d.Name); ok {
		operations = n
	}

	c.Printf("Disk %s:%s, is online%s%s, file-oriented device.\n\n", d.Name, typeClause, allocClause, mountClause)
	c.statRow("Error count", d.ErrCnt, "Operations completed", operations)
	c.statRow("Reference count", d.RefCnt, "Default buffer size", d.DevBufSize)
	c.statRow("Total blocks", totalBlocks, "Free blocks", freeBlocks)
	c.Printf("\n")
	c.statRow("Volume label", fmt.Sprintf("%q", volLabel), "Cluster size", clusterSize)
	c.statRow("Number of files", fileCount, "Maximum files allowed", maxFiles)
}

// statRow prints one line of SHOW DEVICE/FULL's two-column stat block:
// two label/value pairs, each label left-justified and its value
// right-justified within a fixed field width, approximating real VMS's
// own fixed-column layout closely enough to be readable without
// depending on byte-exact VMS field widths (which vary per specific
// field on genuine VMS output, e.g. the wider value field "Volume label"
// needs for its quoted string).
func (c *Console) statRow(label1 string, val1 any, label2 string, val2 any) {
	c.Printf("    %-27s%12v    %-27s%12v\n", label1, val1, label2, val2)
}

// The DEVCHAR bits SHOW DEVICE/FULL names.
var (
	devRecord    = vmsdef.Symbols["DEV$M_REC"]
	devShareable = vmsdef.Symbols["DEV$M_SHR"]
	devMailbox   = vmsdef.Symbols["DEV$M_MBX"]
)

// showNullDeviceFull prints SHOW DEVICE/FULL of the null device NLA0: in
// the layout VMS 7.1 gave it (testdata/mp/probe2, docs/PHASE-45.md):
//
//	Device NLA0:, device type null device, is online, record-oriented device,
//	    shareable, mailbox device.
//
//	    Error count                    0    Operations completed                 31
//	    Owner process                 ""    Owner UIC                         [1,1]
//	    Owner process ID        00000000    Dev Prot    S:RWPL,O:RWPL,G:RWPL,W:RWPL
//	    Reference count               10    Default buffer size                 512
//
// The characteristics after "is online" come from DEVCHAR's bits, and the
// first sentence wraps where the next phrase would pass column 78. The
// owner UIC is group and member in octal. The protection is VMS's for
// NLA0: (govax keeps none per device). The counts are the device's own.
func (c *Console) showNullDeviceFull(d *iodev.Device) {
	phrases := []string{"device type null device", "is online"}

	if d.Allocated() {
		phrases = append(phrases, "allocated")
	}

	for _, ch := range []struct {
		bit  uint32
		text string
	}{{devRecord, "record-oriented device"}, {devShareable, "shareable"}, {devMailbox, "mailbox device"}} {
		if d.DevChar&ch.bit != 0 {
			phrases = append(phrases, ch.text)
		}
	}

	// "Device NLA0:" and the phrases, wrapped: a phrase that would take the
	// line (with its comma) past column 78 starts the next, indented.
	line := fmt.Sprintf("Device %s:", d.Name)

	for _, p := range phrases {
		if len(line)+len(", ")+len(p)+1 > 78 {
			c.Printf("%s,\n", line)

			line = "    " + p

			continue
		}

		line += ", " + p
	}

	c.Printf("%s.\n\n", line)

	owner := `""`

	if d.PID != 0 && c.RTL != nil {
		if env, found := c.RTL.FindProcess(d.PID); found && env.Process.Name != "" {
			owner = fmt.Sprintf("%q", env.Process.Name)
		}
	}

	c.vmsRow("Error count", d.ErrCnt, "Operations completed", d.OpCnt)
	c.vmsRow("Owner process", owner, "Owner UIC", fmt.Sprintf("[%o,%o]", d.OwnUIC>>16, d.OwnUIC&0xFFFF))
	c.vmsRow("Owner process ID", fmt.Sprintf("%08X", d.PID), "Dev Prot", "S:RWPL,O:RWPL,G:RWPL,W:RWPL")
	c.vmsRow("Reference count", d.RefCnt, "Default buffer size", d.DevBufSize)
}

// vmsRow prints one line of SHOW DEVICE/FULL in VMS's two columns: the
// first label and its value right-justified to 32 columns, then the
// second to 39, with four blanks between.
func (c *Console) vmsRow(label1 string, val1 any, label2 string, val2 any) {
	v1, v2 := fmt.Sprint(val1), fmt.Sprint(val2)

	c.Printf("    %s%*s    %s%*s\n", label1, 32-len(label1), v1, label2, 39-len(label2), v2)
}
