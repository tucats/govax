package dclcmd

import (
	"fmt"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmserrors"
)

// SET PROCESS's changes to a process (docs/planning/PHASE-94 -
// privileges.md, subtask 8): /PRIORITY, /SUSPEND, and /RESUME, of the
// process itself or of another named by its process name or
// /IDENTIFICATION, each by the services' rule (GROUP for a process of the
// group, WORLD for any other; corevms's dclprocess.go). VMS 7.1's DCL said
// of a refusal
//
//	%SET-E-NOTSET, error modifying 00000130
//	-SYSTEM-F-NOPRIV, insufficient privilege or object protection violation
//
// and of a priority above its own without ALTPRI, checked before anything
// else,
//
//	%SET-E-NOTSET, need ALTPRI privilege to elevate above base priority
//
// (testdata/priv94's SEE sections, and round 2's PRIORITY sections).
// /PRIVILEGES is privileges.go's, and changes only the process itself.

// ProcessOptions are SET PROCESS's parameter and qualifiers.
type ProcessOptions struct {
	Name           string   // the process name parameter
	Identification string   // /IDENTIFICATION's PID, hexadecimal
	Privileges     []string // /PRIVILEGES's list, nil if not given
	HasPriority    bool     // /PRIORITY given
	Priority       int      // /PRIORITY's value
	Suspend        bool     // /SUSPEND
	Resume         bool     // /RESUME
}

// setProcNotSet is SET's NOTSET condition as SET PROCESS shows it, an
// error (unconfirmed: the value; the logs show the message).
var setProcNotSet = setStatus("", 6, 2)

// SetProcess is SET PROCESS: the privileges of the process itself, then
// the priority, suspension, or resumption of the process opts names.
func (c *Context) SetProcess(opts ProcessOptions) error {
	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	if opts.Privileges != nil {
		if err := c.SetProcessPrivileges(opts.Privileges); err != nil {
			return err
		}
	}

	if !opts.HasPriority && !opts.Suspend && !opts.Resume {
		return nil
	}

	if opts.HasPriority && (opts.Priority < 0 || !c.RTL.MayRaisePriority(uint32(opts.Priority))) {
		if opts.Priority < 0 {
			return vmserrors.NewSegment(vmserrors.CLI_IVVALU, "PRIORITY")
		}

		return vmserrors.ConditionError{Status: setProcNotSet,
			Text: "SET-E-NOTSET, need ALTPRI privilege to elevate above base priority"}
	}

	target, err := c.findProcess(opts.Name, opts.Identification)
	if err != nil {
		return err
	}

	var steps []func(*corevms.Environment) uint32

	if opts.HasPriority {
		steps = append(steps, func(t *corevms.Environment) uint32 { return c.RTL.SetProcessPriority(t, uint32(opts.Priority)) })
	}

	if opts.Suspend {
		steps = append(steps, c.RTL.SuspendProcess)
	}

	if opts.Resume {
		steps = append(steps, c.RTL.ResumeProcess)
	}

	for _, step := range steps {
		if st := step(target); st&1 == 0 {
			text := fmt.Sprintf("SET-E-NOTSET, error modifying %08X\n%s", target.Process.PID, ConditionLine(st))

			return vmserrors.ConditionError{Status: setProcNotSet, Text: text}
		}
	}

	return nil
}
