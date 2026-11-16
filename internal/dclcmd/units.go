package dclcmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/rms"
)

// This file is the "next free unit" rule for the devices govax mounts as
// it starts (docs/planning/PHASE-87 - system volume.md, Decision 6). A VMS device
// name is a device type (DU, an MSCP disk), a controller letter (A), and
// a unit number (0): DUA0. The system volume claims a unit (DUA0 by
// default), so a configuration that also mounts a work volume can't know
// which units are left. It names the controller alone instead, DUA, and
// gets the lowest unit nothing is mounted on: DUA1, beside the system
// volume.

// ResolveUnit also chooses an MD controller's unit for MOUNT: a RAM disk
// has to be defined before it's mounted, so MOUNT MDA takes the lowest
// unit that is a RAM disk with nothing mounted on it.

// controllerName is a device name with no unit: two letters of device
// type and a controller letter.
var controllerName = regexp.MustCompile(`^[A-Z]{3}$`)

// ResolveUnit returns the device a startup setting or option names. A
// controller with no unit ("DUA", "dua:", "_DUA:") is the lowest unit of
// it with nothing mounted on it ("DUA1" when the system volume is DUA0);
// anything else (a device with its unit, a logical name) comes back as it
// was given, uppercase and without its colon, for Mount to translate. A
// logical name of three letters (DSK:) is a logical name, not a
// controller.
func (c *Context) ResolveUnit(device string) string {
	name := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(device), ":"))
	bare := strings.TrimPrefix(name, "_")

	if !controllerName.MatchString(bare) {
		return name
	}

	// A logical name translates to something other than itself.
	if !strings.HasPrefix(name, "_") {
		if physical, err := rms.PhysicalDevice(c.Logicals, bare); err == nil && physical != bare {
			return name
		}
	}

	// An MD controller's unit must be a RAM disk already: the lowest one
	// that is, with nothing mounted (docs/planning/PHASE-96 - RAM
	// disk.md, Decision 6). With none, the rule below gives a unit MOUNT
	// will find isn't one (NOSUCHDEV).
	if unit, ok := c.freeRAMDisk(bare); ok {
		return unit
	}

	for unit := 0; ; unit++ {
		candidate := fmt.Sprintf("%s%d", bare, unit)
		if _, mounted := c.Mounts.Lookup(candidate); !mounted {
			return candidate
		}
	}
}

// freeRAMDisk returns the lowest unit of the MD controller controller
// ("MDA") that is a RAM disk with nothing mounted on it, and false when
// controller isn't an MD controller or has no such unit.
func (c *Context) freeRAMDisk(controller string) (string, bool) {
	if _, ok := RAMDiskName(controller); !ok || c.Devices == nil {
		return "", false
	}

	best, found := 0, false

	for _, d := range c.Devices.All() {
		number, err := strconv.Atoi(strings.TrimPrefix(d.Name, controller))
		if !strings.HasPrefix(d.Name, controller) || err != nil || number < 0 {
			continue
		}

		if _, mounted := c.Mounts.Lookup(d.Name); mounted || !c.Mounts.IsMemory(d.Name) {
			continue
		}

		if !found || number < best {
			best, found = number, true
		}
	}

	return fmt.Sprintf("%s%d", controller, best), found
}
