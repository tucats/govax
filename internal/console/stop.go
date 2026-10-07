package console

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// STOP and the cleanup of processes (docs/PHASE-45.md, subtask 12).

// StopProcess is STOP: it deletes a process and its subprocesses at once,
// as the operator with every privilege would. The process is named, as
// for SHOW PROCESS, by name (in process 1's UIC group) or by /IDENTIFICATION
// (hexadecimal; the PID wins if both are given). A process that isn't
// there is %SYSTEM-W-NONEXPR. Process 1 is the console's own, so STOP
// of it is refused (%SYSTEM-F-NOPRIV): govax has no logging out, and
// EXIT ends the session. With neither a name nor an identification,
// that is process 1 too.
//
// The deleted process's files are closed, and its termination message
// goes to its termination mailbox if it had one, with the final status
// SS$_ABORT. If the CPU was in the process (a run stopped there), it is
// returned to process 1.
func (c *Console) StopProcess(name, pid string) error {
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

	if env == c.RTL {
		return vmserrors.New(vmserrors.SS_NOPRIV)
	}

	c.RTL.DeleteNow(env)

	return c.ReturnToProcessOne()
}

// endOtherProcesses deletes every process but process 1, closing their
// files, before the machine they ran on is replaced or ends (INIT,
// VMINIT, ZERO, and govax's exit). It must run while the old memory is
// still the machine's, since a process's rundown reads its tables.
func (c *Console) endOtherProcesses() {
	if c.RTL == nil || c.Mem == nil {
		return
	}

	c.RTL.DeleteOtherProcesses()
}
