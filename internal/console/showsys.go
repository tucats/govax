package console

import (
	"os"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// SHOW SYSTEM and SHOW PROCESS (docs/PHASE-44.md, subtask 10). The
// reports are built by internal/corevms (showsys.go), which has the
// process table and the scheduler; these commands find the process and
// print.

// ShowSystem is SHOW SYSTEM: a line for each process, with its state,
// priority, and CPU time. Its title names the system as govax and its
// version, and the node as the host (systemName, hostNode).
func (c *Console) ShowSystem() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	for _, line := range c.RTL.SystemReport(systemName(), hostNode()) {
		c.Printf("%s\n", line)
	}

	return nil
}

// ShowProcess is SHOW PROCESS: one process's details. With neither a name
// nor /IDENTIFICATION it is the console's own process, process 1, as
// DCL's SHOW PROCESS is the process typing it. A name is looked up in
// process 1's UIC group, as VMS looks up process names; pid is
// hexadecimal, as SHOW SYSTEM shows it, and wins if both are given. A
// process that isn't there is %SYSTEM-W-NONEXPR.
func (c *Console) ShowProcess(name, pid string) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	env := c.RTL

	switch {
	case pid != "":
		v, err := strconv.ParseUint(strings.TrimSpace(pid), 16, 32)
		if err != nil {
			return vmserrors.New(vmserrors.SS_IVIDENT)
		}

		var found bool
		if env, found = c.RTL.FindProcess(uint32(v)); !found {
			return vmserrors.New(vmserrors.SS_NONEXPR)
		}

	case name != "":
		var found bool
		if env, found = c.RTL.FindProcessName(c.RTL.Process.UICGroup(), name); !found {
			return vmserrors.New(vmserrors.SS_NONEXPR)
		}
	}

	for _, line := range env.ProcessReport(hostNode()) {
		c.Printf("%s\n", line)
	}

	return nil
}

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
