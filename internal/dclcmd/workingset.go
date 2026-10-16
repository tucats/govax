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
//	%SET-I-NEWLIMS, new working set:  Limit = 150  Quota = 700  Extent = 700
//
// (two blanks before each value, as VMS 7.1 shows it).
// (When VMS shows the message without /LOG is unconfirmed: both of the
// Dictionary's examples show it without, one of them for a value within
// its bounds.)
func (c *Context) SetWorkingSet(opts WorkingSetOptions) error {
	if c.RTL == nil || c.RTL.Process == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	v, clamped := c.RTL.SetWorkingSet(opts.Change)
	if opts.Log || clamped {
		c.Printf("%%SET-I-NEWLIMS, new working set:  Limit = %d  Quota = %d  Extent = %d\n",
			v.Limit, v.Quota, v.Extent)
	}

	return nil
}

// ShowWorkingSet is SHOW WORKING_SET: the process's default working set
// (shown as the limit), quota, and extent, whether automatic adjustment
// is enabled, and the authorized quota and extent, laid out as VMS 7.1
// lays them out (testdata/pg63, round 1): the numbers unpadded, the gaps
// between the fields fixed.
//
//	  Working Set      /Limit=512   /Quota=1024    /Extent=16400
//	  Adjustment enabled      Authorized Quota=1024  Authorized Extent=16400
//
// (With adjustment disabled, the gap after "disabled" is unconfirmed: one
// blank fewer, so the columns line up.)
func (c *Context) ShowWorkingSet() error {
	if c.RTL == nil || c.RTL.Process == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	v := c.RTL.WorkingSetValues()

	adjust := "enabled      "
	if !v.Adjust {
		adjust = "disabled     "
	}

	c.Printf("  Working Set      /Limit=%d   /Quota=%d    /Extent=%d\n", v.Limit, v.Quota, v.Extent)
	c.Printf("  Adjustment %sAuthorized Quota=%d  Authorized Extent=%d\n", adjust, v.AuthQuota, v.AuthExt)

	return nil
}
