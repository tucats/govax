package dclcmd

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmserrors"
)

// SET PROTECTION/DEFAULT and SHOW PROTECTION (docs/planning/PHASE-93 -
// file protection.md, subtask 5): the process's default file protection,
// the code a new file gets when nothing else gives it one
// (corevms.Process.DefaultProtection).

// ShowProtection is SHOW PROTECTION: the default protection of the
// process whose DCL runs the command, as VMS 7.1 showed it, indented two
// spaces, a category with no access "NO ACCESS":
//
//	  SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD=NO ACCESS
func (c *Context) ShowProtection() error {
	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	c.Printf("  %s\n", security.Protection(c.RTL.Process.DefaultProtection).Text("NO ACCESS"))

	return nil
}

// SetProtection is SET PROTECTION[=(code)] with /DEFAULT: code's items
// ("G:RWED", "W:R", or the whole code as one item) are merged into the
// process's default protection, a category the code doesn't name keeping
// what it had, and no code changes nothing (the probe's "SET
// PROTECTION/DEFAULT"). A category that isn't SYSTEM, OWNER, GROUP, or
// WORLD is DCL's %DCL-W-IVKEYW naming it, and any other fault in the
// code %DCL-W-IVPROT, as VMS 7.1's DCL reported them.
//
// Without /DEFAULT the command changes a file's protection, VMS's obsolete
// form of SET SECURITY/PROTECTION, which comes with Phase 93's subtask 11.
func (c *Context) SetProtection(code []string, isDefault bool) error {
	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	if !isDefault {
		return vmserrors.New(vmserrors.CLI_PROTNOTIMPL)
	}

	if len(code) == 0 {
		return nil
	}

	p := c.RTL.Process

	merged, err := security.ParseProtection(strings.Join(code, ","), security.Protection(p.DefaultProtection))
	if err != nil {
		var unknown security.ErrUnknownCategory
		if errors.As(err, &unknown) {
			return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, unknown.Name)
		}

		return vmserrors.New(vmserrors.CLI_IVPROT)
	}

	p.DefaultProtection = uint16(merged)

	return nil
}
