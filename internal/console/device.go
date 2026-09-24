package console

import (
	"fmt"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
)

// DefineDevice implements the DEFINE/DEVICE console command
// (define_device.c), registering a new device in c.Devices. Unlike most
// console commands, this doesn't require INIT — devices.c's own device
// list exists independent of vax_init, matching define_device.c's own
// lack of a vax_init check.
func (c *Console) DefineDevice(name string, opts iodev.DeviceOptions) *iodev.Device {
	return c.Devices.Define(name, opts)
}

// ShowDevices implements the SHOW DEVICES console command (show_device.c),
// optionally filtered to one device by name and expanded to full detail
// with /FULL. Matches show_device's own behavior of printing nothing at
// all when no device matches (no "no matching devices" fallback message —
// unlike ShowLogicals, which does print one; that asymmetry is in the C
// source, not invented here).
//
// A disk-class device's /FULL output (showDiskDeviceFull) is a from-scratch,
// VMS-realistic reformat — not a port of show_device.c, which never
// produced this layout itself (it just dumped raw struct fields, same as
// the plain fallback below still does for every other device class).
func (c *Console) ShowDevices(name string, full bool) error {
	for _, d := range c.Devices.All() {
		if name != "" && d.Name != name {
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

		c.Printf("Device %s\n", d.Name)
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
// "is online" unconditionally; "device type X" is omitted whenever
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

	c.Printf("Disk %s:%s, is online%s, file-oriented device.\n\n", d.Name, typeClause, mountClause)
	c.statRow("Error count", d.ErrCnt, "Operations completed", d.OpCnt)
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
	c.Printf("    %-27s%5v    %-27s%12v\n", label1, val1, label2, val2)
}

// DefineLogical implements the DEFINE/LOGICAL console command
// (define_logical.c), defining name's value within table. Also doesn't
// require INIT, matching define_logical.c.
func (c *Console) DefineLogical(table, name, value string) error {
	if c.CPU != nil && c.CPU.DebugEnabled(vax.DebugLogicals) {
		fmt.Fprintf(c.CPU.DebugWriter(), "DEBUG: DEFINE/LOGICAL %s/TABLE=%s %q\n", name, table, value)
	}

	c.Logicals.Set(table, name, value, 0)

	return nil
}

// ShowLogicals implements the SHOW LOGICAL_NAMES console command
// (show_logical.c), optionally filtered by table and/or name. Always
// quotes the value: this port's LogicalName.Value is always a real string
// once Set has run (Set is the only way to create an entry, and it always
// assigns a concrete value), so show_logical.c's "<undefined>" branch for
// a nil value has nothing to trigger it here — same as in the C source,
// where every entry actually reachable through set_logical has already
// had getmem+strcpy run on its value by the time show_logical can see it.
func (c *Console) ShowLogicals(table, name string) error {
	entries := c.Logicals.AllMatching(table, name)
	for _, e := range entries {
		c.Printf("%s [%s] = %q\n", e.Name.Name, e.Table, e.Name.Value)
	}

	if len(entries) == 0 {
		c.Printf("No matching logical names.\n")
	}
	
	return nil
}
