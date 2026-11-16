package dclcmd

import (
	"os"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmserrors"
)

// SHOW SYSTEM and SHOW PROCESS (docs/planning/PHASE-44.md, subtask 10). The
// reports are built by internal/corevms (showsys.go), which has the
// process table and the scheduler; these commands find the process and
// print.

// ShowSystem is SHOW SYSTEM: a line for each process, with its state,
// priority, and CPU time. Its title names the system as govax and its
// version, and the node as the host (systemName, hostNode).
func (c *Context) ShowSystem() error {
	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	for _, line := range c.RTL.SystemReport(systemName(), hostNode()) {
		c.Printf("%s\n", line)
	}

	return nil
}

// ShowProcess is SHOW PROCESS: one process's details. With neither a name
// nor /IDENTIFICATION it is the console's own process, process 1, as
// DCL's SHOW PROCESS is the process typing it. The process is found by
// findProcess. Another process is shown only to a process that may see
// it, as $GETJPI's rule says (corevms's MaySee: its own UIC, GROUP for
// its group, WORLD for any; docs/planning/PHASE-94 - privileges.md,
// subtask 8), else %SYSTEM-F-NOPRIV.
//
// The qualifiers choose what follows the report's heading in place of
// the process's details (ProcessSections).
func (c *Context) ShowProcess(name, pid string, show ProcessSections) error {
	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	env, err := c.findProcess(name, pid)
	if err != nil {
		return err
	}

	if st := c.RTL.MaySee(env); st != 0 {
		return vmserrors.New(vmserrors.SS_NOPRIV)
	}

	lines := env.ProcessReport(hostNode(), env != c.RTL)
	if show.Privileges || show.Rights || show.Quotas {
		lines = lines[:processHeadingLines:processHeadingLines]

		if show.Quotas {
			lines = append(lines, processQuotasReport(env)...)
		}

		switch {
		case show.Privileges:
			lines = append(lines, processPrivilegesReport(env)...)
		case show.Rights:
			lines = append(lines, processRightsReport(env)...)
		}
	}

	for _, line := range lines {
		c.Printf("%s\n", line)
	}

	return nil
}

// ProcessSections are SHOW PROCESS's qualifiers that choose its report's
// sections: /PRIVILEGES (the privileges, then the rights), /RIGHTS (the
// rights alone), and /QUOTAS (what is left of each quota). With none of
// them the report is the process's details. With more than one, each
// section is shown once, the quotas first, as VMS's /ALL orders them
// (unconfirmed: the probe ran each qualifier alone).
type ProcessSections struct {
	Privileges, Rights, Quotas bool
}

// findProcess is the process SHOW PROCESS and SET PROCESS name: the one
// whose PID is pid, hexadecimal, as SHOW SYSTEM shows it; else the one
// called name in the process's own UIC group, as VMS looks up process
// names; else the process itself. pid wins if both are given. A process
// that isn't there is %SYSTEM-W-NONEXPR.
func (c *Context) findProcess(name, pid string) (*corevms.Environment, error) {
	switch {
	case pid != "":
		v, err := strconv.ParseUint(strings.TrimSpace(pid), 16, 32)
		if err != nil {
			return nil, vmserrors.New(vmserrors.SS_IVIDENT)
		}

		if env, found := c.RTL.FindProcess(uint32(v)); found {
			return env, nil
		}

		return nil, vmserrors.New(vmserrors.SS_NONEXPR)

	case name != "":
		if env, found := c.RTL.FindProcessName(c.RTL.Process.UICGroup(), name); found {
			return env, nil
		}

		return nil, vmserrors.New(vmserrors.SS_NONEXPR)
	}

	return c.RTL, nil
}

// processHeadingLines is how many of ProcessReport's lines are its
// heading: a blank line, then the time, user, and process ID, then the
// node and process name.
const processHeadingLines = 3

// systemName is what SHOW SYSTEM calls the operating system: govax and
// its version ("GOVAX 1.0-280"), where VMS's says "OpenVMS V7.3".
func systemName() string {
	if BuildVersion == "" {
		return "GOVAX"
	}

	return "GOVAX " + BuildVersion
}

// hostNode is SHOW SYSTEM's node: the host's short name (its name up to
// the first dot), or GOVAX, $GETSYI's node name, if the host won't say.
func hostNode() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "GOVAX"
	}

	short, _, _ := strings.Cut(name, ".")

	return short
}
