package console

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/uaf"
)

// LogIn logs process 1 in as username, after STARTUP.COM has run as
// SYSTEM (docs/planning/PHASE-95 - authentication.md, Decisions 3, 6,
// and 7): the process becomes the account's (Machine.LogIn), and its home,
// the account's DEVICE and DIRECTORY, becomes SYS$LOGIN and the default
// directory (Interpreter.SetLoginDirectory).
//
// SYSTEM's home is SYS$SYSROOT:[SYSMGR], SYS$MANAGER, as on VMS, with two
// exceptions. systemHome, when it isn't "", is the default volume's
// directory the configuration named (vax.default.volume.directory), and a
// configured starting place wins. And a home that isn't there (no system
// volume, as the tests run) leaves the default where it was, the default
// volume's directory or the host's, with SYS$LOGIN naming the volume's
// directory when it's on one. Another account's home is used as its
// record gives it, there or not, as LOGINOUT does.
func (c *Console) LogIn(username, systemHome string) (*corevms.Login, error) {
	l, err := c.Machine.LogIn(username, true)
	if err != nil {
		return nil, err
	}

	device, directory := l.Record.Device, l.Record.Directory

	if l.Record.Username == uaf.SystemRecord().Username {
		if systemHome != "" {
			device, directory = splitHome(systemHome)
		}

		if !c.HomeExists(device, directory) {
			var ok bool
			if device, directory, ok = c.VolumeDefault(); !ok {
				return l, nil // the host's directory, as before accounts
			}
		}
	}

	return l, c.SetLoginDirectory(device, directory)
}

// splitHome splits "DUA1:[WORK]" into its device and directory.
func splitHome(home string) (device, directory string) {
	if i := strings.LastIndex(home, ":"); i >= 0 {
		return home[:i+1], home[i+1:]
	}

	return "", home
}
