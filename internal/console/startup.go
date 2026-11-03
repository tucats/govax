package console

import (
	"github.com/tucats/govax/internal/sysvol"
)

// StartupProcedure is govax's built-in startup procedure, by its name in
// internal/bootdata: the one run when there is no system volume.
const StartupProcedure = "startup.com"

// startupName is STARTUP.COM's name and directory on the system volume.
const startupName = sysvol.SysMgr + "STARTUP.COM"

// RunStartup runs govax's startup procedure at command level 1, as a
// login command procedure runs before a user's first command
// (docs/PHASE-87 - system volume.md, Decisions 1 and 11).
//
// With a system volume, it is the volume's STARTUP.COM, opened by device
// and directory (DUA0:[SYSTEM.SYSMGR]STARTUP.COM), so that it doesn't
// depend on a logical name; it is the user's to change, and it runs
// SYS$MANAGER:SYSTARTUP_VMS.COM last. Without one, or if the volume has no
// STARTUP.COM (a read-only volume someone deleted it from), it is the
// built-in copy, found as a host file through the console's search path
// (Paths: the current directory, --path's directories, then govax's own),
// which finds no SYS$SYSDEVICE and so uses only govax's built-in files.
func (c *Console) RunStartup(dispatch func(string) error) error {
	if device := c.Mounts.System(); device != "" {
		spec := device + ":" + startupName

		if names, err := c.ContainerSession.Search(spec); err == nil && len(names) > 0 {
			return c.RunProcedure(spec, dispatch)
		}
	}

	return c.RunHostProcedure(StartupProcedure, dispatch)
}
