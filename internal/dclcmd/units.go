package dclcmd

import (
	"fmt"
	"regexp"
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

	for unit := 0; ; unit++ {
		candidate := fmt.Sprintf("%s%d", bare, unit)
		if _, mounted := c.Mounts.Lookup(candidate); !mounted {
			return candidate
		}
	}
}
