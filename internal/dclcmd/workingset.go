package dclcmd

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmserrors"
)

// WorkingSetOptions are SET WORKING_SET's qualifiers: the corevms
// change it asks for, and /LOG.
type WorkingSetOptions struct {
	Change corevms.WorkingSetChange
	Log    bool
}

// SetWorkingSet is SET WORKING_SET: it changes the process's default
// working set (/LIMIT), quota, and extent within what the process is
// authorized, and turns automatic adjustment on or off
// (corevms.Environment.SetWorkingSet has the rules). With /LOG, or when
// a value asked for was cut back to its bound, it reports the values
// that resulted, as the DCL Dictionary's examples show:
//
//	%SET-I-NEWLIMS, new working set: Limit = 150 Quota = 700 Extent = 700
//
// (When VMS shows the message without /LOG is unconfirmed: both of the
// Dictionary's examples show it without, one of them for a value within
// its bounds.)
func (c *Context) SetWorkingSet(opts WorkingSetOptions) error {
	if c.RTL == nil || c.RTL.Process == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	v, clamped := c.RTL.SetWorkingSet(opts.Change)
	if opts.Log || clamped {
		c.Printf("%%SET-I-NEWLIMS, new working set: Limit = %d Quota = %d Extent = %d\n",
			v.Limit, v.Quota, v.Extent)
	}

	return nil
}

// ShowWorkingSet is SHOW WORKING_SET: the process's default working set
// (shown as the limit), quota, and extent, whether automatic adjustment
// is enabled, and the authorized quota and extent, laid out as the DCL
// Dictionary's VAX example (the exact spacing is unconfirmed):
//
//	  Working Set      /Limit= 180 /Quota= 350           /Extent=1200
//	  Adjustment enabled    Authorized Quota= 350 Authorized Extent=1200
func (c *Context) ShowWorkingSet() error {
	if c.RTL == nil || c.RTL.Process == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	v := c.RTL.WorkingSetValues()

	adjust := "enabled "
	if !v.Adjust {
		adjust = "disabled"
	}

	c.Printf("  Working Set      /Limit=%4d /Quota=%4d           /Extent=%4d\n", v.Limit, v.Quota, v.Extent)
	c.Printf("  Adjustment %s   Authorized Quota=%4d Authorized Extent=%4d\n", adjust, v.AuthQuota, v.AuthExt)

	return nil
}
