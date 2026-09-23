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
func (c *Console) ShowDevices(name string, full bool) error {
	for _, d := range c.Devices.All() {
		if name != "" && d.Name != name {
			continue
		}

		c.Printf("Device %s\n", d.Name)
		
		if !full {
			continue
		}
		
		c.Printf("    DEVCLASS=%d (%s)   DEVTYPE=%d\n", d.DevClass, iodev.DeviceClassName(d.DevClass), d.DevType)
		
		if d.DevClass == iodev.DeviceClassDisk {
			c.Printf("    ACPPID=%08X\n", d.ACPPID)
			c.Printf("    CLUSTER=%d\n", d.Cluster)
			c.Printf("    CYLINDERS=%d\n", d.Cylinders)
			c.Printf("    FREEBLOCKS=%d\n", d.FreeBlocks)
			c.Printf("    MAXBLOCK=%d\n", d.MaxBlock)
			c.Printf("    MAXFILES=%d\n", d.MaxFiles)
			c.Printf("    SECTORS=%d\n", d.Sectors)
			c.Printf("    SERIAL=%d\n", d.Serial)
			c.Printf("    VOLNAME=%s\n", d.VolName)
			c.Printf("    MEDIANAME=%s\n", d.MediaName)
			c.Printf("    MEDIATYPE=%s\n", d.MediaType)
			c.Printf("    ROOTDEVNAME=%s\n", d.RootDevName)
			c.showMountedVolume(d.Name)
		}

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
