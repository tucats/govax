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

// ShowDevices implements the SHOW DEVICES console command, expanded to
// full detail with /FULL. A name selects, as on VMS, every device whose
// name begins with it ("DU" shows DUA0, DUA1, ...; a colon or a leading
// "_" is ignored), and when no device does, the command says
// %SYSTEM-W-NOSUCHDEV, as VMS's SHOW DEVICE does (the author's VMS
// system, 2026-10-08).
//
// A disk-class device's /FULL output (showDiskDeviceFull) is a from-scratch,
// VMS-realistic reformat.
func (c *Console) ShowDevices(name string, full bool) error {
	prefix := strings.ToUpper(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(name), ":"), "_"))
	shown := 0

	defer func() {
		if shown == 0 && prefix != "" {
			c.Printf("%%%s\n", strings.TrimPrefix(conditionLine(ssNOSUCHDEV), "-"))
		}
	}()

	for _, d := range c.Devices.All() {
		if !strings.HasPrefix(d.Name, prefix) {
			continue
		}

		shown++

		if !full {
			c.Printf("Device %s\n", d.Name)

			continue
		}

		if d.DevClass == iodev.DeviceClassDisk {
			c.showDiskDeviceFull(d)

			continue
		}

		if d.DevClass == iodev.DeviceClassMailbox || d.DevClass == iodev.DeviceClassTT {
			c.showRecordDeviceFull(d)

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

// The DEVCHAR bits SHOW DEVICE/FULL names, in bit order, which is the
// order VMS names them in.
var deviceCharacteristics = []struct {
	bit  uint32
	text string
}{
	{vmsdef.Symbols["DEV$M_REC"], "record-oriented device"},
	{vmsdef.Symbols["DEV$M_CCL"], "carriage control"},
	{vmsdef.Symbols["DEV$M_SHR"], "shareable"},
	{vmsdef.Symbols["DEV$M_MBX"], "mailbox device"},
}

// terminalProtection is the protection SHOW DEVICE/FULL shows for a
// terminal, VMS 7.1's for TTA0: (govax keeps none per terminal), and
// nullProtection NLA0:'s.
const (
	terminalProtection = 0xFF00
	nullProtection     = 0
)

// showRecordDeviceFull prints SHOW DEVICE/FULL of a terminal, a mailbox,
// or the null device NLA0: in the layouts VMS 7.1 gave them
// (testdata/mp/probe2 and the author's notes, docs/PHASE-45.md):
//
//	Terminal TTA0:, device type unknown, is online, record-oriented device, carriage
//	    control.
//
//	    Error count                    0    Operations completed                  0
//	    Owner process                 ""    Owner UIC                      [SYSTEM]
//	    Owner process ID        00000000    Dev Prot              S:RWPL,O:RWPL,G,W
//	    Reference count                0    Default buffer size                  80
//
//	Device MBA11:, device type local memory mailbox, is online, record-oriented
//	    device, shareable, mailbox device.
//
//	Device NLA0:, device type null device, is online, record-oriented device,
//	    shareable, mailbox device.
//
// The sentence starts with "Terminal" for a terminal and "Device"
// otherwise; the characteristics after "is online" come from DEVCHAR's
// bits; and it wraps a word at a time, a line taking words while it
// stays within 80 columns. The owner UIC is shown as uicText shows it,
// and the protection as protectionText does: a mailbox's own (its
// $CREMBX promsk), VMS's for a terminal or NLA0:. The counts are the
// device's own: operations are counted as each $QIO completes.
func (c *Console) showRecordDeviceFull(d *iodev.Device) {
	first := "Device"
	if d.DevClass == iodev.DeviceClassTT {
		first = "Terminal"
	}

	phrases := []string{"device type " + recordDeviceTypeName(d), "is online"}

	if d.Allocated() {
		phrases = append(phrases, "allocated")
	}

	for _, ch := range deviceCharacteristics {
		if d.DevChar&ch.bit != 0 {
			phrases = append(phrases, ch.text)
		}
	}

	words := strings.Fields(first + " " + d.Name + ":, " + strings.Join(phrases, ", ") + ".")
	line := words[0]

	for _, w := range words[1:] {
		if len(line)+1+len(w) > 80 {
			c.Printf("%s\n", line)

			line = "    " + w

			continue
		}

		line += " " + w
	}

	c.Printf("%s\n\n", line)

	owner := `""`

	if d.PID != 0 && c.RTL != nil {
		if env, found := c.RTL.FindProcess(d.PID); found && env.Process.Name != "" {
			owner = fmt.Sprintf("%q", env.Process.Name)
		}
	}

	protection := uint32(nullProtection)

	switch {
	case d.DevClass == iodev.DeviceClassTT:
		protection = terminalProtection
	case c.RTL != nil:
		if m, ok := c.RTL.Mailboxes.For(d); ok {
			protection = m.Protection
		}
	}

	c.vmsRow("Error count", d.ErrCnt, "Operations completed", d.OpCnt)
	c.vmsRow("Owner process", owner, "Owner UIC", uicText(d.OwnUIC))
	c.vmsRow("Owner process ID", fmt.Sprintf("%08X", d.PID), "Dev Prot", protectionText(protection))
	c.vmsRow("Reference count", d.RefCnt, "Default buffer size", d.DevBufSize)
}

// recordDeviceTypeName is a terminal's or mailbox's device type as SHOW
// DEVICE/FULL names it. Device types are numbered within each class: a
// mailbox's type 1 (DT$_MBX) is "local memory mailbox" and 3 the null
// device; a terminal's are the terminal types (VT100). Any other is
// "unknown", as VMS 7.1 showed its TTA0:.
func recordDeviceTypeName(d *iodev.Device) string {
	if d.DevClass == iodev.DeviceClassMailbox {
		switch d.DevType {
		case 1:
			return "local memory mailbox"
		case iodev.DeviceTypeNull:
			return "null device"
		}

		return "unknown"
	}

	if d.DevClass == iodev.DeviceClassTT {
		if name, ok := iodev.DeviceTypeName(d.DevType); ok && d.DevType >= 64 {
			return name
		}
	}

	return "unknown"
}

// systemUIC is [1,4], the SYSTEM account's UIC.
const systemUIC = 1<<16 | 4

// uicText is a UIC as SHOW DEVICE/FULL shows it: by its identifier where
// it has one, "[SYSTEM]" for [1,4] (govax has no rights database, and
// SYSTEM is its one account), and otherwise "[group,member]" in octal,
// as VMS 7.1 showed NLA0:'s [1,1].
func uicText(uic uint32) string {
	if uic == systemUIC {
		return "[SYSTEM]"
	}

	return fmt.Sprintf("[%o,%o]", uic>>16, uic&0xFFFF)
}

// protectionText is a protection mask (corevms's uicprot.go: four 4-bit
// fields, System, Owner, Group, World, each bit denying read, write,
// logical, and physical access) as SHOW DEVICE/FULL shows it: each
// category's letter, then the accesses it allows as R, W, P, and L, as
// "S:RWPL,O:RWPL,G,W" shows a category allowed nothing.
func protectionText(mask uint32) string {
	parts := make([]string, 0, 4)

	for i, category := range []string{"S", "O", "G", "W"} {
		field := mask >> (4 * i) & 0xF

		allowed := ""

		for _, a := range []struct {
			bit    uint32
			letter string
		}{{1, "R"}, {2, "W"}, {8, "P"}, {4, "L"}} {
			if field&a.bit == 0 {
				allowed += a.letter
			}
		}

		if allowed == "" {
			parts = append(parts, category)
		} else {
			parts = append(parts, category+":"+allowed)
		}
	}

	return strings.Join(parts, ",")
}

// vmsRow prints one line of SHOW DEVICE/FULL in VMS's two columns: the
// first label and its value right-justified to 32 columns, then the
// second to 39, with four blanks between.
func (c *Console) vmsRow(label1 string, val1 any, label2 string, val2 any) {
	v1, v2 := fmt.Sprint(val1), fmt.Sprint(val2)

	c.Printf("    %s%*s    %s%*s\n", label1, 32-len(label1), v1, label2, 39-len(label2), v2)
}
