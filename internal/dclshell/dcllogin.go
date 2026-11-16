package dclshell

import (
	"strings"

	"github.com/tucats/govax/internal/lnm"
)

// DCL's side of a login (docs/planning/PHASE-95 - authentication.md,
// Decision 7): the logical names that say where the user's home is, and
// the default directory there. VMS 7.1 gave a logged-in user (the probe's
// round 2, SHOW LOGICAL/FULL):
//
//	"SYS$LOGIN" [exec] = "UAF95$DISK:[UAF95]" (LNM$JOB_81C66040)
//	"SYS$LOGIN_DEVICE" [exec] = "UAF95$DISK:" (LNM$JOB_81C66040)
//	"SYS$SCRATCH" [exec] = "UAF95$DISK:[UAF95]" (LNM$JOB_81C66040)
//	"SYS$DISK" [exec] = "UAF95$DISK:" (LNM$PROCESS_TABLE)
//
// each the account's DEVICE and DIRECTORY as the record has them, not
// translated, and the default directory there.

// The names a login defines.
const (
	loginName       = "SYS$LOGIN"
	loginDeviceName = "SYS$LOGIN_DEVICE"
	scratchName     = "SYS$SCRATCH"
	diskName        = "SYS$DISK"
)

// SetLoginDirectory makes device and directory (an account's "DUA1:" and
// "[SMITH]", or "SYS$SYSROOT:" and "[SYSMGR]") process 1's home: SYS$LOGIN,
// SYS$LOGIN_DEVICE, and SYS$SCRATCH in its job table at executive mode,
// the default directory there, and SYS$DISK at executive mode in its
// process table, in place of the supervisor-mode one SET DEFAULT
// defines. A home SET DEFAULT can't take is its error, nothing changed
// but the names.
func (c *Interpreter) SetLoginDirectory(device, directory string) error {
	return c.home.setLoginDirectory(device, directory)
}

// setLoginDirectory is SetLoginDirectory for sh, any process's DCL (a
// batch job's too, dclbatch.go).
func (sh *Shell) setLoginDirectory(device, directory string) error {
	device = strings.ToUpper(strings.TrimSpace(device))
	directory = strings.ToUpper(strings.TrimSpace(directory))

	if device != "" && !strings.HasSuffix(device, ":") {
		device += ":"
	}

	define := func(table, name, value string) {
		eqv := []lnm.Equivalence{{Value: value}}
		_, _ = sh.Logicals.Define(table, name, lnm.Executive, 0, eqv)
	}

	job := sh.Logicals.JobTableName
	define(job, loginName, device+directory)
	define(job, loginDeviceName, device)
	define(job, scratchName, device+directory)

	if err := sh.ContainerSession.SetDefault(device + directory); err != nil {
		return err
	}

	if device != "" {
		_, _ = sh.Logicals.DeleteExact(lnm.ProcessTableName, diskName, lnm.Supervisor)
		define(lnm.ProcessTableName, diskName, device)
	}

	return nil
}

// HomeExists reports whether device and directory are a directory on a
// mounted volume, which a home must be to be the default directory.
func (c *Interpreter) HomeExists(device, directory string) bool {
	device = strings.TrimSpace(device)
	if device != "" && !strings.HasSuffix(device, ":") {
		device += ":"
	}

	_, found, err := c.home.ContainerSession.Parse(device+strings.TrimSpace(directory), "", "", false, false)

	return err == nil && found
}

// VolumeDefault returns the default directory's device and directory
// when it is on a volume ("DUA1:" and "[WORK]"), and false when the
// default is the host's: there is no SYS$DISK.
func (c *Interpreter) VolumeDefault() (device, directory string, ok bool) {
	sh := c.home
	if _, err := sh.Logicals.Translate(lnm.FileDevName, diskName, lnm.User, 0); err != nil {
		return "", "", false
	}

	text, _, _ := strings.Cut(sh.ContainerSession.DefaultString(), "\n")

	i := strings.LastIndex(text, ":")
	if i < 0 || sh.ContainerSession.DefaultMissing() {
		return "", "", false
	}

	return text[:i+1], text[i+1:], true
}
