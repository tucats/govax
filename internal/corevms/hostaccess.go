package corevms

// Host file access (docs/planning/PHASE-97 - host file access.md).
//
// govax can reach the files of the computer it runs on, the host: a name
// with a "/" in it, a bare name when the default directory is on the
// host, a name with govax's /HOST qualifier, a program's EXE$OPEN, and a
// few more. VMS has no such thing, so no VMS rule says who may. A site
// that wants to keep unprivileged users and their programs inside the
// emulated system turns on the configuration key vax.host.sysprv: then
// only a process whose current privileges include SYSPRV may use a host
// file. With the key off, the default, every process may, as before,
// and the gate isn't even asked, so SET DEBUG PRIVILEGES shows nothing.
//
// Every such access asks one rule, ruleHostFile, through HostAllowed.
// RMS reaches it through the hook UseSession sets on each process's
// session (rms.Session.HostAccess), which rms.Session.Locate asks before
// it places a name on the host.

// HostSYSPRVSetting is the configuration key that makes host files need
// SYSPRV. The machine reads it into System.HostRequiresSYSPRV.
const HostSYSPRVSetting = "vax.host.sysprv"

// HostAllowed is the gate for a host file: 0 if the process may use one
// now, SS$_NOPRIV if not. With System.HostRequiresSYSPRV off it is
// always 0, and nothing is traced. The privileges are the process's
// current ones, read at the moment of the access, from the caller's
// mode (DCL's commands from supervisor mode, a program's from its own;
// no mode is exempt, so the mode shows only in the trace).
func (env *Environment) HostAllowed() uint32 {
	if env.System == nil || !env.HostRequiresSYSPRV {
		return 0
	}

	return env.requirePrivilege(ruleHostFile)
}

// HostCommandAllowed is HostAllowed for one of DCL's own commands that
// reach the host without a file name RMS places (SPAWN/HOST, and MOUNT
// and INITIALIZE/CONTAINER of a container file): the same rule, asked
// from supervisor mode, DCL's, as CommandPrivilege asks it ("HOST_FILE").
// With the key off it is 0, and nothing is traced.
func (env *Environment) HostCommandAllowed() uint32 {
	if env.System == nil || !env.HostRequiresSYSPRV {
		return 0
	}

	return env.CommandPrivilege("HOST_FILE")
}
